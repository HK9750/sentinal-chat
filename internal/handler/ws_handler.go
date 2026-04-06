package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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

func NewWSHandler(authService *services.AuthService, hub *chatws.Hub, realtimeService *services.RealtimeService, frontendURL string, l *logger.Logger) *WSHandler {
	allowedOrigins := buildAllowedOrigins(frontendURL)
	return &WSHandler{
		authService:     authService,
		hub:             hub,
		realtimeService: realtimeService,
		logger:          l,
		upgrader: gorillawebsocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     websocketOriginChecker(allowedOrigins),
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
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		_ = conn.Close()
		return
	}
	baseCtx := context.WithValue(context.Background(), logger.UserIdKey, claims.UserID)
	ctx, cancel := context.WithCancel(baseCtx)
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
	if h.realtimeService != nil {
		_ = h.realtimeService.UpdatePresence(ctx, client.UserID, true)
		// Background: bulk-mark all pending messages as DELIVERED for this user.
		go h.realtimeService.DeliverPendingOnConnect(ctx, client.UserID)
	}
	if h.logger != nil {
		h.logger.InfofCtx(ctx, "ws.connect user_id=%s session_id=%s device_id=%s client_id=%s", claims.UserID, claims.SessionID, claims.DeviceID, client.ID)
	}
	go client.WritePump()
	h.sendEnvelope(client, chatws.ConnectionReadyEnvelope(claims.UserID, claims.SessionID, claims.DeviceID))
	go h.readPump(ctx, client)
}

func (h *WSHandler) readPump(ctx context.Context, client *chatws.Client) {
	defer func() {
		if h.realtimeService != nil {
			_ = h.realtimeService.UpdatePresence(context.Background(), client.UserID, false)
		}
		client.Close()
		if h.logger != nil {
			h.logger.InfofCtx(context.Background(), "ws.disconnect user_id=%s device_id=%s client_id=%s", client.UserID.String(), strings.TrimSpace(client.DeviceID), client.ID)
		}
	}()
	_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetReadLimit(1 << 20)
	client.Conn.SetPongHandler(func(string) error {
		_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, payload, err := client.Conn.ReadMessage()
		if err != nil {
			if h.logger != nil {
				h.logger.InfofCtx(ctx, "ws.read.closed client_id=%s err=%v", client.ID, err)
			}
			return
		}
		var frame httpdto.WebSocketInboundFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			if h.logger != nil {
				h.logger.Warnw("ws.invalid_frame", "client_id", client.ID, "user_id", client.UserID.String(), "error", err.Error())
			}
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
	case events.InboundMessageDeleteBulk:
		h.handleMessageDeleteBulk(ctx, client, frame)
	case events.InboundMessageReactionAdd, events.InboundMessageReactionRemove:
		h.handleReaction(ctx, client, frame)
	case events.InboundMessagePin, events.InboundMessageUnpin:
		h.handlePin(ctx, client, frame)
	case events.InboundReceiptDelivered, events.InboundReceiptRead, events.InboundReceiptPlayed:
		h.handleReceipt(ctx, client, frame)
	case events.InboundPollVote, events.InboundPollClose:
		h.handlePoll(ctx, client, frame)
	case events.InboundCommandUndo, events.InboundCommandRedo:
		h.handleCommand(ctx, client, frame)
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
		ConversationID:  conversationID,
		SenderID:        client.UserID,
		ClientMessageID: stringValue(data["client_message_id"]),
		Type:            stringValue(data["type"]),
		Content:         stringValue(data["content"]),
		ReplyToMsgID:    replyTo,
		ExpiresAt:       expiresAt,
		AttachmentIDs:   attachmentIDs,
		MentionUserIDs:  mentionIDs,
		Poll:            pollInput,
	})
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "MESSAGE_SEND_FAILED", err.Error())
		return
	}
	_ = message
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
	message, err := h.realtimeService.EditMessage(ctx, client.UserID, services.EditMessageInput{ConversationID: conversationID, MessageID: messageID, EditorID: client.UserID, Content: stringValue(data["content"]), ExpiresAt: expiresAt})
	if err != nil {
		h.sendError(client, "MESSAGE_EDIT_FAILED", err.Error())
		return
	}
	_ = message
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
	_ = message
}

func (h *WSHandler) handleMessageDeleteBulk(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}

	messageIDs, err := services.AnyToUUIDSlice(data["message_ids"])
	if err != nil || len(messageIDs) == 0 {
		h.sendError(client, "INVALID_MESSAGE_IDS", "invalid message ids")
		return
	}

	mode := strings.TrimSpace(strings.ToUpper(stringValue(data["delete_mode"])))
	if mode == "" {
		mode = "FOR_ME"
	}
	if mode != "FOR_ME" && mode != "FOR_EVERYONE" {
		h.sendError(client, "INVALID_DELETE_MODE", "invalid delete mode")
		return
	}

	items, err := h.realtimeService.DeleteMessages(ctx, services.BulkDeleteMessagesInput{
		ConversationID: conversationID,
		ActorID:        client.UserID,
		MessageIDs:     messageIDs,
		DeleteMode:     mode,
	})
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "MESSAGE_DELETE_FAILED", err.Error())
		return
	}

	_ = items
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
	_ = reactions
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
}

func (h *WSHandler) handleReceipt(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	conversationID, data, frameErr := h.parseConversationData(frame)
	if frameErr != nil {
		h.sendError(client, frameErr.code, frameErr.message)
		return
	}
	messageIDs := []uuid.UUID{}
	if rawMessageIDs, exists := data["message_ids"]; exists && rawMessageIDs != nil {
		var err error
		messageIDs, err = services.AnyToUUIDSlice(rawMessageIDs)
		if err != nil {
			h.sendError(client, "INVALID_MESSAGE_IDS", "invalid message ids")
			return
		}
	}
	upToSeq, err := services.AnyToInt64Ptr(data["up_to_seq_id"])
	if err != nil {
		h.sendError(client, "INVALID_SEQUENCE", "invalid up_to_seq_id")
		return
	}
	status := receiptStatus(frame.Type)
	result, err := h.realtimeService.UpdateReceipt(ctx, client.UserID, services.ReceiptInput{ConversationID: conversationID, MessageIDs: messageIDs, ActorID: client.UserID, Status: status, UpToSeqID: upToSeq})
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "RECEIPT_FAILED", err.Error())
		return
	}
	_ = result
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
		_ = poll
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
	_ = poll
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
			h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_CONVERSATION", "invalid conversation")
			return
		}
		payload, err := h.realtimeService.StartCall(ctx, services.CallStartInput{ConversationID: conversationID, CallerID: client.UserID, Type: stringValue(data["type"])})
		if err != nil {
			h.sendErrorWithRequestID(client, frame.RequestID, "CALL_START_FAILED", err.Error())
			return
		}
		callID, _ := uuid.Parse(stringValue(payload["call_id"]))
		envelope := chatws.NewCallEvent(events.CallIncoming, conversationID, callID, payload)
		envelope.RequestID = strings.TrimSpace(frame.RequestID)
		h.sendEnvelope(client, envelope)
		return
	}
	callID, err := uuid.Parse(strings.TrimSpace(frame.CallID))
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_CALL", "invalid call")
		return
	}
	if frame.Type == events.InboundCallEnd {
		reason := stringValue(data["reason"])
		if h.logger != nil {
			h.logger.InfowCtx(ctx, "[CALL_END] inbound call:end received",
				"call_id", callID.String(),
				"actor_id", client.UserID.String(),
				"reason", reason,
				"request_id", strings.TrimSpace(frame.RequestID),
				"conversation_id", strings.TrimSpace(frame.ConversationID),
			)
		}

		if err := h.realtimeService.EndCall(ctx, services.CallEndInput{CallID: callID, ActorID: client.UserID, Reason: reason}); err != nil {
			if h.logger != nil {
				h.logger.WarnwCtx(ctx, "[CALL_END] failed to emit call:end",
					"call_id", callID.String(),
					"actor_id", client.UserID.String(),
					"reason", reason,
					"error", err.Error(),
				)
			}
			h.sendErrorWithRequestID(client, frame.RequestID, "CALL_END_FAILED", err.Error())
			return
		}

		if h.logger != nil {
			h.logger.InfowCtx(ctx, "[CALL_END] call:end emitted",
				"call_id", callID.String(),
				"actor_id", client.UserID.String(),
				"reason", reason,
			)
		}
		return

	}
	conversationID, err := parseFrameConversationID(frame)
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_CONVERSATION", "invalid conversation")
		return
	}
	toUserID, err := uuid.Parse(stringValue(data["to_user_id"]))
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_TARGET", "invalid target user")
		return
	}
	if err := h.realtimeService.ForwardCallSignal(ctx, frame.Type, services.CallSignalInput{CallID: callID, ConversationID: conversationID, FromUserID: client.UserID, ToUserID: toUserID, Payload: data}); err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "CALL_SIGNAL_FAILED", err.Error())
	}
}

func (h *WSHandler) handleCommand(ctx context.Context, client *chatws.Client, frame httpdto.WebSocketInboundFrame) {
	var conversationID *uuid.UUID
	if strings.TrimSpace(frame.ConversationID) != "" {
		parsedConversationID, err := parseFrameConversationID(frame)
		if err != nil {
			h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_CONVERSATION", "invalid conversation")
			return
		}
		conversationID = &parsedConversationID
	}

	if frame.Type == events.InboundCommandUndo {
		result, err := h.realtimeService.UndoLatest(ctx, client.UserID, conversationID)
		if err != nil {
			h.sendErrorWithRequestID(client, frame.RequestID, "COMMAND_UNDO_FAILED", err.Error())
			return
		}
		envelope := chatws.NewUserEvent(events.CommandUndone, client.UserID.String(), map[string]any{"command": result})
		envelope.RequestID = strings.TrimSpace(frame.RequestID)
		h.sendEnvelope(client, envelope)
		return
	}

	data, ok := frame.Data.(map[string]any)
	if !ok {
		h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_DATA", "invalid payload")
		return
	}
	commandID, err := uuid.Parse(stringValue(data["command_id"]))
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "INVALID_COMMAND", "invalid command")
		return
	}
	result, err := h.realtimeService.Redo(ctx, client.UserID, commandID)
	if err != nil {
		h.sendErrorWithRequestID(client, frame.RequestID, "COMMAND_REDO_FAILED", err.Error())
		return
	}
	envelope := chatws.NewUserEvent(events.CommandRedone, client.UserID.String(), map[string]any{"command": result})
	envelope.RequestID = strings.TrimSpace(frame.RequestID)
	h.sendEnvelope(client, envelope)
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
	if client == nil {
		return
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("ws.send_envelope.marshal: %v", err)
		}
		return
	}

	if !client.Enqueue(body) {
		if h.logger != nil {
			h.logger.Errorw("ws.send_envelope.queue_full", "client_id", client.ID, "user_id", client.UserID.String())
		}
	}
}

func (h *WSHandler) sendError(client *chatws.Client, code, message string) {
	h.sendErrorWithRequestID(client, "", code, message)
}

func (h *WSHandler) sendErrorWithRequestID(client *chatws.Client, requestID, code, message string) {
	h.sendEnvelope(client, chatws.EventEnvelope{Type: events.ErrorEvent, RequestID: strings.TrimSpace(requestID), SentAt: time.Now().UTC(), Data: map[string]any{"code": code, "message": message}})
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

func websocketOriginChecker(allowedOrigins map[string]struct{}) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return false
		}
		_, ok := allowedOrigins[parsed.Scheme+"://"+parsed.Host]
		return ok
	}
}

func buildAllowedOrigins(frontendURL string) map[string]struct{} {
	origins := map[string]struct{}{
		"http://localhost:3000":  {},
		"http://127.0.0.1:3000":  {},
		"https://localhost:3000": {},
		"https://127.0.0.1:3000": {},
	}
	if parsed, err := url.Parse(strings.TrimSpace(frontendURL)); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origins[parsed.Scheme+"://"+parsed.Host] = struct{}{}
	}
	return origins
}
