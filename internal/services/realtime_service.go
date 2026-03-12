package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/events"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type RealtimeService struct {
	broadcaster     chatws.Broadcaster
	messageService  *MessageService
	callService     *CallService
	conversationSvc *ConversationService
}

func NewRealtimeService(broadcaster chatws.Broadcaster, conversationSvc *ConversationService, messageSvc *MessageService, callSvc *CallService) *RealtimeService {
	return &RealtimeService{
		broadcaster:     broadcaster,
		messageService:  messageSvc,
		callService:     callSvc,
		conversationSvc: conversationSvc,
	}
}

func (s *RealtimeService) HandleTyping(ctx context.Context, conversationID, userID uuid.UUID, started bool) error {
	if s == nil || s.conversationSvc == nil || s.broadcaster == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if _, err := s.conversationSvc.Get(ctx, conversationID, userID); err != nil {
		return err
	}
	eventType := events.TypingStarted
	if !started {
		eventType = events.TypingStopped
	}
	return s.broadcaster.BroadcastConversation(ctx, conversationID, chatws.NewConversationEvent(eventType, conversationID, map[string]any{"user_id": userID.String()}), &userID)
}

func (s *RealtimeService) SendMessage(ctx context.Context, userID uuid.UUID, in SendMessageInput) (MessageView, error) {
	message, err := s.messageService.Send(ctx, in)
	if err != nil {
		return MessageView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.MessageNew, in.ConversationID, map[string]any{"message": message})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, &userID)
	}
	return message, nil
}

func (s *RealtimeService) EditMessage(ctx context.Context, userID uuid.UUID, in EditMessageInput) (MessageView, error) {
	message, err := s.messageService.Edit(ctx, in)
	if err != nil {
		return MessageView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.MessageEdited, in.ConversationID, map[string]any{"message": message})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, &userID)
	}
	return message, nil
}

func (s *RealtimeService) DeleteMessage(ctx context.Context, userID uuid.UUID, in DeleteMessageInput) (MessageView, error) {
	message, err := s.messageService.Delete(ctx, in)
	if err != nil {
		return MessageView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.MessageDeleted, in.ConversationID, map[string]any{"message": message})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, &userID)
	}
	return message, nil
}

func (s *RealtimeService) UpdateReaction(ctx context.Context, in ReactionInput, add bool) ([]ReactionView, error) {
	var (
		reactions []ReactionView
		err       error
	)
	if add {
		reactions, err = s.messageService.AddReaction(ctx, in)
	} else {
		reactions, err = s.messageService.RemoveReaction(ctx, in)
	}
	if err != nil {
		return nil, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.MessageReaction, in.ConversationID, map[string]any{"message_id": in.MessageID.String(), "reactions": reactions})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
	}
	return reactions, nil
}

func (s *RealtimeService) PinMessage(ctx context.Context, in PinMessageInput) error {
	_, err := s.messageService.Pin(ctx, in)
	if err != nil {
		return err
	}
	if s.broadcaster != nil {
		eventType := events.MessagePinned
		if !in.Pinned {
			eventType = events.MessageUnpinned
		}
		envelope := chatws.NewMessageEvent(eventType, in.ConversationID, map[string]any{"message_id": in.MessageID.String(), "pinned": in.Pinned})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
	}
	return nil
}

func (s *RealtimeService) UpdateReceipt(ctx context.Context, userID uuid.UUID, in ReceiptInput) error {
	if err := s.messageService.UpdateReceipt(ctx, in); err != nil {
		return err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.ReceiptUpdate, in.ConversationID, map[string]any{"message_ids": uuidSliceToStrings(in.MessageIDs), "user_id": userID.String(), "status": in.Status, "up_to_seq_id": in.UpToSeqID})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, &userID)
	}
	return nil
}

func (s *RealtimeService) VotePoll(ctx context.Context, in VotePollInput) (PollView, error) {
	poll, err := s.messageService.VotePoll(ctx, in)
	if err != nil {
		return PollView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewConversationEvent(events.PollUpdate, in.ConversationID, map[string]any{"poll": poll})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
	}
	return poll, nil
}

func (s *RealtimeService) ClosePoll(ctx context.Context, userID, conversationID, pollID uuid.UUID) (PollView, error) {
	poll, err := s.messageService.ClosePoll(ctx, conversationID, pollID, userID)
	if err != nil {
		return PollView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewConversationEvent(events.PollUpdate, conversationID, map[string]any{"poll": poll})
		_ = s.broadcaster.BroadcastConversation(ctx, conversationID, envelope, nil)
	}
	return poll, nil
}

func (s *RealtimeService) StartCall(ctx context.Context, in CallStartInput) (map[string]any, error) {
	call, participants, err := s.callService.Start(ctx, in)
	if err != nil {
		return nil, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewCallEvent(events.CallIncoming, in.ConversationID, call.ID, map[string]any{"call_id": call.ID.String(), "type": call.Type, "initiated_by": in.CallerID.String()})
		for _, participant := range participants {
			if participant.UserID == in.CallerID {
				continue
			}
			s.broadcaster.SendToUser(participant.UserID, envelope)
		}
	}
	return map[string]any{"call_id": call.ID.String(), "type": call.Type, "initiated_by": in.CallerID.String(), "started_at": call.StartedAt}, nil
}

func (s *RealtimeService) ForwardCallSignal(ctx context.Context, frameType string, in CallSignalInput) error {
	if err := s.callService.ForwardSignal(ctx, in); err != nil {
		return err
	}
	if s.broadcaster == nil {
		return nil
	}
	eventType := events.CallOffer
	if frameType == events.InboundCallAnswer {
		eventType = events.CallAnswer
	}
	if frameType == events.InboundCallICE {
		eventType = events.CallICE
	}
	s.broadcaster.SendToUser(in.ToUserID, chatws.NewCallEvent(eventType, in.ConversationID, in.CallID, map[string]any{"from_user_id": in.FromUserID.String(), "payload": in.Payload}))
	return nil
}

func (s *RealtimeService) EndCall(ctx context.Context, in CallEndInput) error {
	return s.callService.End(ctx, in)
}

func AnyToUUIDSlice(value any) ([]uuid.UUID, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, sentinal_errors.ErrInvalidInput
	}
	items := make([]uuid.UUID, 0, len(list))
	for _, item := range list {
		str, ok := item.(string)
		if !ok {
			return nil, sentinal_errors.ErrInvalidInput
		}
		id, err := uuid.Parse(strings.TrimSpace(str))
		if err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, nil
}

func AnyToUUIDPtr(value any) (*uuid.UUID, error) {
	str, ok := value.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(str))
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func AnyToTimePtr(value any) (*time.Time, error) {
	str, ok := value.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(str))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func AnyToInt64Ptr(value any) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	floatVal, ok := value.(float64)
	if !ok {
		return nil, sentinal_errors.ErrInvalidInput
	}
	parsed := int64(floatVal)
	return &parsed, nil
}

func AnyToStringSlice(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(list))
	for _, item := range list {
		str, ok := item.(string)
		if ok && strings.TrimSpace(str) != "" {
			items = append(items, strings.TrimSpace(str))
		}
	}
	return items
}

func uuidSliceToStrings(ids []uuid.UUID) []string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, id.String())
	}
	return items
}
