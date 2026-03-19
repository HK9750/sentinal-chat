package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/command"
	"sentinal-chat/internal/events"
	chatws "sentinal-chat/internal/websocket"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type RealtimeService struct {
	broadcaster     chatws.Broadcaster
	messageService  *MessageService
	callService     *CallService
	conversationSvc *ConversationService
	commandService  *CommandService
	userService     *UserService
}

type presenceCounter interface {
	UserConnectionCount(userID uuid.UUID) int
}

func NewRealtimeService(broadcaster chatws.Broadcaster, conversationSvc *ConversationService, messageSvc *MessageService, callSvc *CallService, commandSvc *CommandService, userSvc *UserService) *RealtimeService {
	return &RealtimeService{
		broadcaster:     broadcaster,
		messageService:  messageSvc,
		callService:     callSvc,
		conversationSvc: conversationSvc,
		commandService:  commandSvc,
		userService:     userSvc,
	}
}

func (s *RealtimeService) UpdatePresence(ctx context.Context, userID uuid.UUID, online bool) error {
	if s == nil || s.userService == nil || s.broadcaster == nil || userID == uuid.Nil {
		return nil
	}

	if !online {
		if counter, ok := s.broadcaster.(presenceCounter); ok && counter.UserConnectionCount(userID) > 0 {
			return nil
		}
	}

	if s.userService.users != nil {
		if err := s.userService.users.UpdateOnlineStatus(ctx, userID, online); err != nil {
			return err
		}
	}

	presenceData := map[string]any{
		"user_id":   userID.String(),
		"is_online": online,
	}
	if !online {
		presenceData["last_seen_at"] = time.Now().UTC()
	}
	envelope := chatws.NewUserEvent(events.PresenceUpdate, userID.String(), presenceData)

	s.broadcaster.SendToUser(userID, envelope)
	if err := s.broadcaster.PublishToUser(ctx, userID, envelope); err != nil {
		return err
	}

	if s.userService.users == nil {
		return nil
	}

	contacts, err := s.userService.users.GetUserContacts(ctx, userID)
	if err != nil {
		return err
	}
	for _, contact := range contacts {
		s.broadcaster.SendToUser(contact.ContactUserID, envelope)
		if err := s.broadcaster.PublishToUser(ctx, contact.ContactUserID, envelope); err != nil {
			return err
		}
	}

	return nil
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
	envelope := chatws.NewConversationEvent(eventType, conversationID, map[string]any{"user_id": userID.String()})
	if err := s.broadcaster.BroadcastConversation(ctx, conversationID, envelope, &userID); err != nil {
		return err
	}
	return s.broadcaster.PublishConversation(ctx, conversationID, envelope)
}

func (s *RealtimeService) SendMessage(ctx context.Context, userID uuid.UUID, in SendMessageInput) (MessageView, error) {
	message, err := s.messageService.Send(ctx, in)
	if err != nil {
		return MessageView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.MessageNew, in.ConversationID, map[string]any{"message": message})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
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
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
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
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
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
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
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
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
	}
	return nil
}

func (s *RealtimeService) UpdateReceipt(ctx context.Context, userID uuid.UUID, in ReceiptInput) (ReceiptUpdateResult, error) {
	result, err := s.messageService.UpdateReceipt(ctx, in)
	if err != nil {
		return ReceiptUpdateResult{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewMessageEvent(events.ReceiptUpdate, in.ConversationID, map[string]any{"message_ids": uuidSliceToStrings(result.MessageIDs), "user_id": userID.String(), "status": result.Status, "up_to_seq_id": result.UpToSeqID})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
	}
	return result, nil
}

func (s *RealtimeService) VotePoll(ctx context.Context, in VotePollInput) (PollView, error) {
	poll, err := s.messageService.VotePoll(ctx, in)
	if err != nil {
		return PollView{}, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewConversationEvent(events.PollUpdate, in.ConversationID, map[string]any{"poll": poll})
		_ = s.broadcaster.BroadcastConversation(ctx, in.ConversationID, envelope, nil)
		_ = s.broadcaster.PublishConversation(ctx, in.ConversationID, envelope)
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
		_ = s.broadcaster.PublishConversation(ctx, conversationID, envelope)
	}
	return poll, nil
}

func (s *RealtimeService) StartCall(ctx context.Context, in CallStartInput) (map[string]any, error) {
	call, participantIDs, err := s.callService.Start(ctx, in)
	if err != nil {
		return nil, err
	}
	if s.broadcaster != nil {
		envelope := chatws.NewCallEvent(events.CallIncoming, in.ConversationID, call.ID, map[string]any{"call_id": call.ID.String(), "type": call.Type, "initiated_by": in.CallerID.String(), "participant_ids": uuidSliceToStrings(participantIDs)})
		for _, participantID := range participantIDs {
			if participantID == in.CallerID {
				continue
			}
			s.broadcaster.SendToUser(participantID, envelope)
		}
	}
	return map[string]any{"call_id": call.ID.String(), "type": call.Type, "initiated_by": in.CallerID.String(), "started_at": call.StartedAt, "participant_ids": uuidSliceToStrings(participantIDs)}, nil
}

func (s *RealtimeService) ForwardCallSignal(ctx context.Context, frameType string, in CallSignalInput) error {
	in.SignalType = frameType
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
	envelope := chatws.NewCallEvent(eventType, in.ConversationID, in.CallID, map[string]any{"from_user_id": in.FromUserID.String(), "payload": in.Payload})
	s.broadcaster.SendToUser(in.ToUserID, envelope)
	return s.broadcaster.PublishToUser(ctx, in.ToUserID, envelope)
}

func (s *RealtimeService) EndCall(ctx context.Context, in CallEndInput) error {
	_, err := s.callService.End(ctx, in)
	return err
}

func (s *RealtimeService) UndoLatest(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID) (CommandResult, error) {
	if s == nil || s.commandService == nil {
		return CommandResult{}, sentinal_errors.ErrServiceUnavailable
	}
	log, err := s.commandService.GetLatestUndoable(ctx, userID, conversationID)
	if err != nil {
		return CommandResult{}, err
	}
	return s.undoCommand(ctx, log)
}

func (s *RealtimeService) Redo(ctx context.Context, userID, commandID uuid.UUID) (CommandResult, error) {
	if s == nil || s.commandService == nil {
		return CommandResult{}, sentinal_errors.ErrServiceUnavailable
	}
	log, err := s.commandService.GetByID(ctx, commandID)
	if err != nil {
		return CommandResult{}, err
	}
	if log.UserID != userID {
		return CommandResult{}, sentinal_errors.ErrForbidden
	}
	if log.Status != command.StatusUndone || log.UndoneAt == nil {
		return CommandResult{}, sentinal_errors.ErrConflict
	}
	return s.redoCommand(ctx, &log)
}

func (s *RealtimeService) undoCommand(ctx context.Context, log command.CommandLog) (CommandResult, error) {
	switch log.CommandType {
	case command.CommandClearChat:
		if s.conversationSvc == nil {
			return CommandResult{}, sentinal_errors.ErrServiceUnavailable
		}
		return s.conversationSvc.applyCommandUndo(ctx, log)
	default:
		if s.messageService == nil {
			return CommandResult{}, sentinal_errors.ErrServiceUnavailable
		}
		return s.messageService.applyCommandUndo(ctx, log)
	}
}

func (s *RealtimeService) redoCommand(ctx context.Context, log *command.CommandLog) (CommandResult, error) {
	if log == nil {
		return CommandResult{}, sentinal_errors.ErrInvalidInput
	}
	switch log.CommandType {
	case command.CommandClearChat:
		if s.conversationSvc == nil {
			return CommandResult{}, sentinal_errors.ErrServiceUnavailable
		}
		return s.conversationSvc.applyCommandRedo(ctx, log)
	default:
		if s.messageService == nil {
			return CommandResult{}, sentinal_errors.ErrServiceUnavailable
		}
		return s.messageService.applyCommandRedo(ctx, log)
	}
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
