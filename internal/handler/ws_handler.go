package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorillawebsocket "github.com/gorilla/websocket"

	"sentinal-chat/internal/events"
	"sentinal-chat/internal/middleware"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
	"sentinal-chat/pkg/logger"
)

type WSHandler struct {
	authService     *services.AuthService
	hub             *chatws.Hub
	realtimeService *services.RealtimeService
	logger          *logger.Logger
	upgrader        gorillawebsocket.Upgrader
}

func NewWSHandler(authService *services.AuthService, hub *chatws.Hub, realtimeService *services.RealtimeService, l *logger.Logger) *WSHandler {
	return &WSHandler{
		authService:     authService,
		hub:             hub,
		realtimeService: realtimeService,
		logger:          l,
		upgrader: gorillawebsocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(_ *http.Request) bool { return true },
		},
	}
}

func (h *WSHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/ws", h.Connect)
}

func (h *WSHandler) Connect(c *gin.Context) {
	claims, err := h.authenticate(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized", "code": "UNAUTHORIZED"})
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	userID, _ := uuid.Parse(claims.UserID)
	ctx, cancel := context.WithCancel(context.Background())
	client := &chatws.Client{
		ID:         uuid.NewString(),
		UserID:     userID,
		SessionID:  claims.SessionID,
		DeviceID:   claims.DeviceID,
		Conn:       conn,
		Send:       make(chan []byte, 128),
		Hub:        h.hub,
		CancelFunc: cancel,
	}
	h.hub.Register(client)
	go client.WritePump()
	ready, _ := json.Marshal(chatws.ConnectionReadyEnvelope(claims.UserID, claims.SessionID, claims.DeviceID))
	client.Send <- ready
	go h.readPump(ctx, client)
}

func (h *WSHandler) readPump(ctx context.Context, client *chatws.Client) {
	defer client.Close()
	_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, payload, err := client.Conn.ReadMessage()
		if err != nil {
			return
		}
		var frame httpdto.WebSocketInboundFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			h.sendError(client, "INVALID_FRAME", "invalid frame")
			continue
		}
		h.handleFrame(ctx, client, frame)
	}
}

func (h *WSHandler) handleFrame(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	switch frame.Type {
	case events.InboundPing:
		h.sendEnvelope(client, chatws.EventEnvelope{Type: "pong", SentAt: time.Now().UTC()})
	case events.InboundTypingStart, events.InboundTypingStop:
		h.handleTyping(ctx, client, frame)
	case events.InboundMessageSend:
		h.handleMessageSend(ctx, client, frame)
	case events.InboundMessageEdit:
		h.handleMessageEdit(ctx, client, frame)
	case events.InboundMessageDelete:
		h.handleMessageDelete(ctx, client, frame)
	case events.InboundMessageReactionAdd, events.InboundMessageReactionRemove:
		h.handleReaction(ctx, client, frame)
	case events.InboundMessagePin, events.InboundMessageUnpin:
		h.handlePin(ctx, client, frame)
	case events.InboundReceiptDelivered, events.InboundReceiptRead, events.InboundReceiptPlayed:
		h.handleReceipt(ctx, client, frame)
	case events.InboundPollVote, events.InboundPollClose:
		h.handlePoll(ctx, client, frame)
	case events.InboundCallStart, events.InboundCallOffer, events.InboundCallAnswer, events.InboundCallICE, events.InboundCallEnd:
		h.handleCall(ctx, client, frame)
	default:
		h.sendError(client, "UNKNOWN_EVENT", "unsupported event")
	}
}

func (h *WSHandler) handleTyping(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, err := parseFrameConversationID(frame)
	if err != nil {
		h.sendError(client, "INVALID_CONVERSATION", "invalid conversation")
		return
	}
	if err := h.realtimeService.HandleTyping(ctx, conversationID, client.UserID, frame.Type == events.InboundTypingStart); err != nil {
		h.sendError(client, "TYPING_FAILED", err.Error())
	}
}

func (h *WSHandler) handleMessageSend(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameDataErr := h.parseConversationData(frame)
	if frameDataErr != nil {
		h.sendError(client, frameDataErr.code, frameDataErr.message)
		return
	}
	attachmentIDs, err := services.AnyToUUIDSlice(data["attachment_ids"])
	if err != nil && data["attachment_ids"] != nil {
		h.sendError(client, "INVALID_ATTACHMENTS", "invalid attachment ids")
		return
	}
	mentionIDs, err := services.AnyToUUIDSlice(data["mention_user_ids"])
	if err != nil && data["mention_user_ids"] != nil {
		h.sendError(client, "INVALID_MENTIONS", "invalid mention ids")
		return
	}
	replyTo, err := services.AnyToUUIDPtr(data["reply_to_msg_id"])
	if err != nil {
		h.sendError(client, "INVALID_REPLY", "invalid reply target")
		return
	}
	expiresAt, err := services.AnyToTimePtr(data["expires_at"])
	if err != nil {
		h.sendError(client, "INVALID_EXPIRY", "invalid expires_at")
		return
	}
	var pollInput *services.CreatePollInput
	if rawPoll, ok := data["poll"].(map[string]any); ok {
		closesAt, err := services.AnyToTimePtr(rawPoll["closes_at"])
		if err != nil {
			h.sendError(client, "INVALID_POLL", "invalid poll payload")
			return
		}
		pollInput = &services.CreatePollInput{
			Question:       stringValue(rawPoll["question"]),
			AllowsMultiple: boolValue(rawPoll["allows_multiple"]),
			ClosesAt:       closesAt,
			Options:        services.AnyToStringSlice(rawPoll["options"]),
		}
	}
	message, err := h.realtimeService.SendMessage(ctx, client.UserID, services.SendMessageInput{
		ConversationID:   conversationID,
		SenderID:         client.UserID,
		ClientMessageID:  stringValue(data["client_message_id"]),
		Type:             stringValue(data["type"]),
		EncryptedContent: stringValue(data["encrypted_content"]),
		ReplyToMsgID:     replyTo,
		ExpiresAt:        expiresAt,
		AttachmentIDs:    attachmentIDs,
		MentionUserIDs:   mentionIDs,
		Poll:             pollInput,
	})
	if err != nil {
		h.sendError(client, "MESSAGE_SEND_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(events.MessageNew, conversationID, map[string]any{"message": message}))
}

func (h *WSHandler) handleMessageEdit(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageID, err := uuid.Parse(stringValue(data["message_id"]))
	if err != nil {
		h.sendError(client, "INVALID_MESSAGE", "invalid message")
		return
	}
	expiresAt, err := services.AnyToTimePtr(data["expires_at"])
	if err != nil {
		h.sendError(client, "INVALID_EXPIRY", "invalid expires_at")
		return
	}
	message, err := h.realtimeService.EditMessage(ctx, client.UserID, services.EditMessageInput{ConversationID: conversationID, MessageID: messageID, EditorID: client.UserID, EncryptedContent: stringValue(data["encrypted_content"]), ExpiresAt: expiresAt})
	if err != nil {
		h.sendError(client, "MESSAGE_EDIT_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(events.MessageEdited, conversationID, map[string]any{"message": message}))
}

func (h *WSHandler) handleMessageDelete(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageID, err := uuid.Parse(stringValue(data["message_id"]))
	if err != nil {
		h.sendError(client, "INVALID_MESSAGE", "invalid message")
		return
	}
	message, err := h.realtimeService.DeleteMessage(ctx, client.UserID, services.DeleteMessageInput{ConversationID: conversationID, MessageID: messageID, ActorID: client.UserID})
	if err != nil {
		h.sendError(client, "MESSAGE_DELETE_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(events.MessageDeleted, conversationID, map[string]any{"message": message}))
}

func (h *WSHandler) handleReaction(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageID, err := uuid.Parse(stringValue(data["message_id"]))
	if err != nil {
		h.sendError(client, "INVALID_MESSAGE", "invalid message")
		return
	}
	reactions, err := h.realtimeService.UpdateReaction(ctx, services.ReactionInput{ConversationID: conversationID, MessageID: messageID, ActorID: client.UserID, Code: stringValue(data["reaction_code"])}, frame.Type == events.InboundMessageReactionAdd)
	if err != nil {
		h.sendError(client, "REACTION_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(events.MessageReaction, conversationID, map[string]any{"message_id": messageID.String(), "reactions": reactions}))
}

func (h *WSHandler) handlePin(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageID, err := uuid.Parse(stringValue(data["message_id"]))
	if err != nil {
		h.sendError(client, "INVALID_MESSAGE", "invalid message")
		return
	}
	pinned := frame.Type == events.InboundMessagePin
	if err := h.realtimeService.PinMessage(ctx, services.PinMessageInput{ConversationID: conversationID, MessageID: messageID, ActorID: client.UserID, Pinned: pinned}); err != nil {
		h.sendError(client, "PIN_FAILED", err.Error())
		return
	}
	eventType := events.MessagePinned
	if !pinned {
		eventType = events.MessageUnpinned
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(eventType, conversationID, map[string]any{"message_id": messageID.String(), "pinned": pinned}))
}

func (h *WSHandler) handleReceipt(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageIDs, err := services.AnyToUUIDSlice(data["message_ids"])
	if err != nil {
		h.sendError(client, "INVALID_MESSAGE_IDS", "invalid message ids")
		return
	}
	upToSeq, err := services.AnyToInt64Ptr(data["up_to_seq_id"])
	if err != nil {
		h.sendError(client, "INVALID_SEQUENCE", "invalid up_to_seq_id")
		return
	}
	status := receiptStatus(frame.Type)
	if err := h.realtimeService.UpdateReceipt(ctx, client.UserID, services.ReceiptInput{ConversationID: conversationID, MessageIDs: messageIDs, ActorID: client.UserID, Status: status, UpToSeqID: upToSeq}); err != nil {
		h.sendError(client, "RECEIPT_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewMessageEvent(events.ReceiptUpdate, conversationID, map[string]any{"message_ids": uuidStrings(messageIDs), "user_id": client.UserID.String(), "status": status, "up_to_seq_id": upToSeq}))
}

func (h *WSHandler) handlePoll(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	pollID, err := uuid.Parse(stringValue(data["poll_id"]))
	if err != nil {
		h.sendError(client, "INVALID_POLL", "invalid poll")
		return
	}
	if frame.Type == events.InboundPollClose {
		poll, err := h.realtimeService.ClosePoll(ctx, client.UserID, conversationID, pollID)
		if err != nil {
			h.sendError(client, "POLL_CLOSE_FAILED", err.Error())
			return
		}
		h.sendEnvelope(client, chatws.NewConversationEvent(events.PollUpdate, conversationID, map[string]any{"poll": poll}))
		return
	}
	optionIDs, err := services.AnyToUUIDSlice(data["option_ids"])
	if err != nil {
		h.sendError(client, "INVALID_OPTIONS", "invalid poll options")
		return
	}
	poll, err := h.realtimeService.VotePoll(ctx, services.VotePollInput{ConversationID: conversationID, PollID: pollID, ActorID: client.UserID, OptionIDs: optionIDs})
	if err != nil {
		h.sendError(client, "POLL_VOTE_FAILED", err.Error())
		return
	}
	h.sendEnvelope(client, chatws.NewConversationEvent(events.PollUpdate, conversationID, map[string]any{"poll": poll}))
}

func (h *WSHandler) handleCall(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	data, ok := frame.Data.(map[string]any)
	if !ok {
		h.sendError(client, "INVALID_DATA", "invalid payload")
		return
	}
	if frame.Type == events.InboundCallStart {
		conversationID, err := parseFrameConversationID(frame)
		if err != nil {
			h.sendError(client, "INVALID_CONVERSATION", "invalid conversation")
			return
		}
		payload, err := h.realtimeService.StartCall(ctx, services.CallStartInput{ConversationID: conversationID, CallerID: client.UserID, Type: stringValue(data["type"])})
		if err != nil {
			h.sendError(client, "CALL_START_FAILED", err.Error())
			return
		}
		callID, _ := uuid.Parse(stringValue(payload["call_id"]))
		h.sendEnvelope(client, chatws.NewCallEvent(events.CallIncoming, conversationID, callID, payload))
		return
	}
	callID, err := uuid.Parse(strings.TrimSpace(frame.CallID))
	if err != nil {
		h.sendError(client, "INVALID_CALL", "invalid call")
		return
	}
	if frame.Type == events.InboundCallEnd {
		if err := h.realtimeService.EndCall(ctx, services.CallEndInput{CallID: callID, ActorID: client.UserID, Reason: stringValue(data["reason"])}); err != nil {
			h.sendError(client, "CALL_END_FAILED", err.Error())
			return
		}
		h.sendEnvelope(client, chatws.EventEnvelope{Type: events.CallEnded, CallID: callID.String(), SentAt: time.Now().UTC(), Data: map[string]any{"call_id": callID.String(), "actor_id": client.UserID.String()}})
		return
	}
	conversationID, err := parseFrameConversationID(frame)
	if err != nil {
		h.sendError(client, "INVALID_CONVERSATION", "invalid conversation")
		return
	}
	toUserID, err := uuid.Parse(stringValue(data["to_user_id"]))
	if err != nil {
		h.sendError(client, "INVALID_TARGET", "invalid target user")
		return
	}
	if err := h.realtimeService.ForwardCallSignal(ctx, frame.Type, services.CallSignalInput{CallID: callID, ConversationID: conversationID, FromUserID: client.UserID, ToUserID: toUserID, Payload: data}); err != nil {
		h.sendError(client, "CALL_SIGNAL_FAILED", err.Error())
	}
}

func (h *WSHandler) authenticate(c *gin.Context) (*middleware.TokenClaims, error) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = extractBearer(c)
	}
	if token == "" {
		return nil, sentinal_errors.ErrUnauthorized
	}
	claims, err := h.authService.ParseAccessToken(token)
	if err != nil {
		return nil, err
	}
	if err := h.authService.ValidateAccessSession(c.Request.Context(), claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (h *WSHandler) sendEnvelope(client *chatws.Client, envelope chatws.EventEnvelope) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	select {
	case client.Send <- body:
	default:
	}
}

func (h *WSHandler) sendError(client *chatws.Client, code, message string) {
	h.sendEnvelope(client, chatws.EventEnvelope{Type: events.ErrorEvent, SentAt: time.Now().UTC(), Data: map[string]any{"code": code, "message": message}})
}

type frameDataError struct {
	code    string
	message string
}

func (h *WSHandler) parseConversationData(frame httpdto.WebSocketInboundFrame) (uuid.UUID, map[string]any, *frameDataError) {
	conversationID, err := parseFrameConversationID(frame)
	if err != nil {
		return uuid.Nil, nil, &frameDataError{code: "INVALID_CONVERSATION", message: "invalid conversation"}
	}
	data, ok := frame.Data.(map[string]any)
	if !ok {
		return uuid.Nil, nil, &frameDataError{code: "INVALID_DATA", message: "invalid payload"}
	}
	return conversationID, data, nil
}

func parseFrameConversationID(frame httpdto.WebSocketInboundFrame) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(frame.ConversationID))
}

func stringValue(value any) string {
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func boolValue(value any) bool {
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	return false
}

func receiptStatus(frameType string) string {
	status := "DELIVERED"
	if frameType == events.InboundReceiptRead {
		status = "READ"
	}
	if frameType == events.InboundReceiptPlayed {
		status = "PLAYED"
	}
	return status
}

func uuidStrings(ids []uuid.UUID) []string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, id.String())
	}
	return items
}

func extractBearer(c *gin.Context) string {
	value := c.GetHeader("Authorization")
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
