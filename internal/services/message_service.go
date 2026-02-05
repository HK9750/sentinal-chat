package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/proxy"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageService struct {
	db          *gorm.DB
	messageRepo repository.MessageRepository
	eventRepo   repository.EventRepository
	access      *proxy.AccessControl
	bus         *commands.Bus
}

func NewMessageService(db *gorm.DB, messageRepo repository.MessageRepository, eventRepo repository.EventRepository, access *proxy.AccessControl, bus *commands.Bus) *MessageService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &MessageService{
		db:          db,
		messageRepo: messageRepo,
		eventRepo:   eventRepo,
		access:      access,
		bus:         bus,
	}
	svc.RegisterHandlers()
	return svc
}

func (s *MessageService) HandleSendMessage(ctx context.Context, cmd commands.SendMessageCommand) (commands.Result, error) {
	if err := cmd.Validate(); err != nil {
		return commands.Result{}, err
	}
	if s.bus != nil {
		return s.bus.Execute(ctx, cmd)
	}
	return s.executeSendMessage(ctx, cmd)
}

func (s *MessageService) RegisterHandlers() {
	if s.bus == nil {
		s.bus = commands.NewBus()
	}
	s.bus.Register("message.send", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		typed, ok := cmd.(commands.SendMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		return s.executeSendMessage(ctx, typed)
	}))
	s.bus.Register("message.update", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SimpleCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		msg, ok := c.Payload.(message.Message)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.Update(ctx, msg); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: msg.ID.String(), Payload: msg}, nil
	}))

	// message.edit - Edit an existing message
	s.bus.Register("message.edit", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.EditMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		msg, err := s.messageRepo.GetByID(ctx, c.MessageID)
		if err != nil {
			return commands.Result{}, err
		}
		if msg.SenderID != c.UserID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		msg.Content = sql.NullString{String: c.NewContent, Valid: true}
		msg.EditedAt = sql.NullTime{Time: time.Now(), Valid: true}
		if err := s.messageRepo.Update(ctx, msg); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message", "message.updated", msg.ID, msg)
		return commands.Result{AggregateID: msg.ID.String(), Payload: msg}, nil
	}))

	// message.delete - Delete a message
	s.bus.Register("message.delete", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.DeleteMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		msg, err := s.messageRepo.GetByID(ctx, c.MessageID)
		if err != nil {
			return commands.Result{}, err
		}
		if msg.SenderID != c.UserID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if c.DeleteForEveryone {
			if err := s.messageRepo.SoftDelete(ctx, c.MessageID); err != nil {
				return commands.Result{}, err
			}
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message", "message.deleted", c.MessageID, map[string]any{
			"message_id":          c.MessageID,
			"deleted_by":          c.UserID,
			"delete_for_everyone": c.DeleteForEveryone,
		})
		return commands.Result{AggregateID: c.MessageID.String()}, nil
	}))

	// message.react - Add a reaction to a message
	s.bus.Register("message.react", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.ReactToMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		reaction := &message.MessageReaction{
			MessageID:    c.MessageID,
			UserID:       c.UserID,
			ReactionCode: c.ReactionCode,
			CreatedAt:    time.Now(),
		}
		if err := s.messageRepo.AddReaction(ctx, reaction); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "reaction", "reaction.added", c.MessageID, map[string]any{
			"message_id":    c.MessageID,
			"user_id":       c.UserID,
			"reaction_code": c.ReactionCode,
		})
		return commands.Result{AggregateID: c.MessageID.String(), Payload: reaction}, nil
	}))

	// message.remove_reaction - Remove a reaction from a message
	s.bus.Register("message.remove_reaction", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RemoveReactionCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messageRepo.RemoveReaction(ctx, c.MessageID, c.UserID, c.ReactionCode); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "reaction", "reaction.removed", c.MessageID, map[string]any{
			"message_id":    c.MessageID,
			"user_id":       c.UserID,
			"reaction_code": c.ReactionCode,
		})
		return commands.Result{AggregateID: c.MessageID.String()}, nil
	}))

	// message.star - Star a message
	s.bus.Register("message.star", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.StarMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		starred := &message.StarredMessage{
			UserID:    c.UserID,
			MessageID: c.MessageID,
			StarredAt: time.Now(),
		}
		if err := s.messageRepo.StarMessage(ctx, starred); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message", "message.starred", c.MessageID, map[string]any{
			"message_id": c.MessageID,
			"user_id":    c.UserID,
		})
		return commands.Result{AggregateID: c.MessageID.String(), Payload: starred}, nil
	}))

	// message.unstar - Unstar a message
	s.bus.Register("message.unstar", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UnstarMessageCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messageRepo.UnstarMessage(ctx, c.UserID, c.MessageID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message", "message.unstarred", c.MessageID, map[string]any{
			"message_id": c.MessageID,
			"user_id":    c.UserID,
		})
		return commands.Result{AggregateID: c.MessageID.String()}, nil
	}))

	// message.read - Mark a message as read
	s.bus.Register("message.read", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.MarkMessageReadCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messageRepo.MarkAsRead(ctx, c.MessageID, c.UserID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message_receipt", "receipt.read", c.MessageID, map[string]any{
			"message_id": c.MessageID,
			"user_id":    c.UserID,
			"read_at":    time.Now(),
		})
		return commands.Result{AggregateID: c.MessageID.String()}, nil
	}))

	// message.delivered - Mark a message as delivered
	s.bus.Register("message.delivered", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.MarkMessageDeliveredCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messageRepo.MarkAsDelivered(ctx, c.MessageID, c.UserID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "message_receipt", "receipt.delivered", c.MessageID, map[string]any{
			"message_id":   c.MessageID,
			"user_id":      c.UserID,
			"delivered_at": time.Now(),
		})
		return commands.Result{AggregateID: c.MessageID.String()}, nil
	}))

	// message.typing - Typing indicator (ephemeral, no persistence)
	s.bus.Register("message.typing", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.TypingCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		eventType := "typing.started"
		if !c.IsTyping {
			eventType = "typing.stopped"
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "typing", eventType, c.ConversationID, map[string]any{
			"conversation_id": c.ConversationID,
			"user_id":         c.UserID,
		})
		return commands.Result{AggregateID: c.ConversationID.String()}, nil
	}))

	// message.create_poll - Create a poll
	s.bus.Register("message.create_poll", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.CreatePollCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		poll := &message.Poll{
			ID:             uuid.New(),
			Question:       c.Question,
			AllowsMultiple: c.AllowsMultiple,
			CreatedAt:      time.Now(),
		}
		if err := s.messageRepo.CreatePoll(ctx, poll); err != nil {
			return commands.Result{}, err
		}
		// Add poll options
		for i, optionText := range c.Options {
			opt := &message.PollOption{
				ID:         uuid.New(),
				PollID:     poll.ID,
				OptionText: optionText,
				Position:   i,
			}
			if err := s.messageRepo.AddPollOption(ctx, opt); err != nil {
				return commands.Result{}, err
			}
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "poll", "poll.created", poll.ID, map[string]any{
			"poll_id":  poll.ID,
			"question": c.Question,
		})
		return commands.Result{AggregateID: poll.ID.String(), Payload: poll}, nil
	}))

	// message.vote_poll - Vote on a poll
	s.bus.Register("message.vote_poll", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.VotePollCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		for _, optionID := range c.OptionIDs {
			vote := &message.PollVote{
				PollID:   c.PollID,
				OptionID: optionID,
				UserID:   c.UserID,
				VotedAt:  time.Now(),
			}
			if err := s.messageRepo.VotePoll(ctx, vote); err != nil {
				return commands.Result{}, err
			}
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "poll", "poll.voted", c.PollID, map[string]any{
			"poll_id":    c.PollID,
			"user_id":    c.UserID,
			"option_ids": c.OptionIDs,
		})
		return commands.Result{AggregateID: c.PollID.String()}, nil
	}))

	// message.close_poll - Close a poll
	s.bus.Register("message.close_poll", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.ClosePollCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.messageRepo.ClosePoll(ctx, c.PollID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "poll", "poll.closed", c.PollID, map[string]any{
			"poll_id":   c.PollID,
			"closed_by": c.UserID,
			"closed_at": time.Now(),
		})
		return commands.Result{AggregateID: c.PollID.String()}, nil
	}))
}

func (s *MessageService) Bus() *commands.Bus {
	return s.bus
}

func (s *MessageService) GetConversationMessages(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int, userID uuid.UUID) ([]message.Message, error) {
	if s.access != nil {
		if err := s.access.CanViewConversation(ctx, userID, conversationID); err != nil {
			return nil, err
		}
	}
	if limit <= 0 {
		limit = 50
	}
	return s.messageRepo.GetConversationMessages(ctx, conversationID, beforeSeq, limit)
}

func (s *MessageService) GetByID(ctx context.Context, messageID uuid.UUID, userID uuid.UUID) (message.Message, error) {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return message.Message{}, err
	}
	if s.access != nil {
		if err := s.access.CanViewConversation(ctx, userID, msg.ConversationID); err != nil {
			return message.Message{}, err
		}
	}
	return msg, nil
}

func (s *MessageService) Delete(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.SoftDelete(ctx, messageID)
}

func (s *MessageService) HardDelete(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.HardDelete(ctx, messageID)
}

func (s *MessageService) Update(ctx context.Context, msg message.Message) error {
	return s.messageRepo.Update(ctx, msg)
}

func (s *MessageService) AddReaction(ctx context.Context, reaction *message.MessageReaction) error {
	return s.messageRepo.AddReaction(ctx, reaction)
}

func (s *MessageService) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, reactionCode string) error {
	return s.messageRepo.RemoveReaction(ctx, messageID, userID, reactionCode)
}

func (s *MessageService) MarkAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	return s.messageRepo.MarkAsRead(ctx, messageID, userID)
}

func (s *MessageService) MarkAsDelivered(ctx context.Context, messageID, userID uuid.UUID) error {
	return s.messageRepo.MarkAsDelivered(ctx, messageID, userID)
}

func (s *MessageService) MarkAsPlayed(ctx context.Context, messageID, userID uuid.UUID) error {
	return s.messageRepo.MarkAsPlayed(ctx, messageID, userID)
}

func (s *MessageService) GetMessagesBySeqRange(ctx context.Context, conversationID uuid.UUID, startSeq, endSeq int64) ([]message.Message, error) {
	return s.messageRepo.GetMessagesBySeqRange(ctx, conversationID, startSeq, endSeq)
}

func (s *MessageService) GetUnreadMessages(ctx context.Context, conversationID, userID uuid.UUID) ([]message.Message, error) {
	return s.messageRepo.GetUnreadMessages(ctx, conversationID, userID)
}

func (s *MessageService) SearchMessages(ctx context.Context, conversationID uuid.UUID, query string, page, limit int) ([]message.Message, int64, error) {
	return s.messageRepo.SearchMessages(ctx, conversationID, query, page, limit)
}

func (s *MessageService) GetMessagesByType(ctx context.Context, conversationID uuid.UUID, msgType string, limit int) ([]message.Message, error) {
	return s.messageRepo.GetMessagesByType(ctx, conversationID, msgType, limit)
}

func (s *MessageService) GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (message.Message, error) {
	return s.messageRepo.GetLatestMessage(ctx, conversationID)
}

func (s *MessageService) MarkAsEdited(ctx context.Context, messageID uuid.UUID) error {
	return s.messageRepo.MarkAsEdited(ctx, messageID)
}

func (s *MessageService) GetMessageCountSince(ctx context.Context, conversationID uuid.UUID, since time.Time) (int64, error) {
	return s.messageRepo.GetMessageCountSince(ctx, conversationID, since)
}

func (s *MessageService) GetByIdempotencyKey(ctx context.Context, key string) (message.Message, error) {
	return s.messageRepo.GetByIdempotencyKey(ctx, key)
}

func (s *MessageService) GetByClientMessageID(ctx context.Context, clientMsgID string) (message.Message, error) {
	return s.messageRepo.GetByClientMessageID(ctx, clientMsgID)
}

func (s *MessageService) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]message.MessageReaction, error) {
	return s.messageRepo.GetMessageReactions(ctx, messageID)
}

func (s *MessageService) GetUserReaction(ctx context.Context, messageID, userID uuid.UUID) (message.MessageReaction, error) {
	return s.messageRepo.GetUserReaction(ctx, messageID, userID)
}

func (s *MessageService) CreateReceipt(ctx context.Context, receipt *message.MessageReceipt) error {
	return s.messageRepo.CreateReceipt(ctx, receipt)
}

func (s *MessageService) UpdateReceipt(ctx context.Context, receipt message.MessageReceipt) error {
	return s.messageRepo.UpdateReceipt(ctx, receipt)
}

func (s *MessageService) GetMessageReceipts(ctx context.Context, messageID uuid.UUID) ([]message.MessageReceipt, error) {
	return s.messageRepo.GetMessageReceipts(ctx, messageID)
}

func (s *MessageService) BulkMarkAsDelivered(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return s.messageRepo.BulkMarkAsDelivered(ctx, messageIDs, userID)
}

func (s *MessageService) BulkMarkAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return s.messageRepo.BulkMarkAsRead(ctx, messageIDs, userID)
}

func (s *MessageService) AddMention(ctx context.Context, mention *message.MessageMention) error {
	return s.messageRepo.AddMention(ctx, mention)
}

func (s *MessageService) GetMessageMentions(ctx context.Context, messageID uuid.UUID) ([]message.MessageMention, error) {
	return s.messageRepo.GetMessageMentions(ctx, messageID)
}

func (s *MessageService) GetUserMentions(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.Message, int64, error) {
	return s.messageRepo.GetUserMentions(ctx, userID, page, limit)
}

func (s *MessageService) StarMessage(ctx context.Context, sMsg *message.StarredMessage) error {
	return s.messageRepo.StarMessage(ctx, sMsg)
}

func (s *MessageService) UnstarMessage(ctx context.Context, userID, messageID uuid.UUID) error {
	return s.messageRepo.UnstarMessage(ctx, userID, messageID)
}

func (s *MessageService) GetUserStarredMessages(ctx context.Context, userID uuid.UUID, page, limit int) ([]message.StarredMessage, int64, error) {
	return s.messageRepo.GetUserStarredMessages(ctx, userID, page, limit)
}

func (s *MessageService) IsMessageStarred(ctx context.Context, userID, messageID uuid.UUID) (bool, error) {
	return s.messageRepo.IsMessageStarred(ctx, userID, messageID)
}

func (s *MessageService) CreateAttachment(ctx context.Context, a *message.Attachment) error {
	return s.messageRepo.CreateAttachment(ctx, a)
}

func (s *MessageService) GetAttachmentByID(ctx context.Context, id uuid.UUID) (message.Attachment, error) {
	return s.messageRepo.GetAttachmentByID(ctx, id)
}

func (s *MessageService) LinkAttachmentToMessage(ctx context.Context, ma *message.MessageAttachment) error {
	return s.messageRepo.LinkAttachmentToMessage(ctx, ma)
}

func (s *MessageService) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]message.Attachment, error) {
	return s.messageRepo.GetMessageAttachments(ctx, messageID)
}

func (s *MessageService) MarkViewOnceViewed(ctx context.Context, attachmentID uuid.UUID) error {
	return s.messageRepo.MarkViewOnceViewed(ctx, attachmentID)
}

func (s *MessageService) CreateLinkPreview(ctx context.Context, lp *message.LinkPreview) error {
	return s.messageRepo.CreateLinkPreview(ctx, lp)
}

func (s *MessageService) GetLinkPreviewByHash(ctx context.Context, urlHash string) (message.LinkPreview, error) {
	return s.messageRepo.GetLinkPreviewByHash(ctx, urlHash)
}

func (s *MessageService) GetLinkPreviewByID(ctx context.Context, id uuid.UUID) (message.LinkPreview, error) {
	return s.messageRepo.GetLinkPreviewByID(ctx, id)
}

func (s *MessageService) CreatePoll(ctx context.Context, p *message.Poll) error {
	return s.messageRepo.CreatePoll(ctx, p)
}

func (s *MessageService) GetPollByID(ctx context.Context, id uuid.UUID) (message.Poll, error) {
	return s.messageRepo.GetPollByID(ctx, id)
}

func (s *MessageService) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	return s.messageRepo.ClosePoll(ctx, pollID)
}

func (s *MessageService) AddPollOption(ctx context.Context, o *message.PollOption) error {
	return s.messageRepo.AddPollOption(ctx, o)
}

func (s *MessageService) GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]message.PollOption, error) {
	return s.messageRepo.GetPollOptions(ctx, pollID)
}

func (s *MessageService) VotePoll(ctx context.Context, v *message.PollVote) error {
	return s.messageRepo.VotePoll(ctx, v)
}

func (s *MessageService) RemoveVote(ctx context.Context, pollID, optionID, userID uuid.UUID) error {
	return s.messageRepo.RemoveVote(ctx, pollID, optionID, userID)
}

func (s *MessageService) GetPollVotes(ctx context.Context, pollID uuid.UUID) ([]message.PollVote, error) {
	return s.messageRepo.GetPollVotes(ctx, pollID)
}

func (s *MessageService) GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]message.PollVote, error) {
	return s.messageRepo.GetUserVotes(ctx, pollID, userID)
}

func (s *MessageService) DeleteExpiredMessages(ctx context.Context) (int64, error) {
	return s.messageRepo.DeleteExpiredMessages(ctx)
}

func (s *MessageService) executeSendMessage(ctx context.Context, cmd commands.SendMessageCommand) (commands.Result, error) {
	if s.db == nil {
		return s.executeSendMessageDirect(ctx, cmd)
	}

	var result commands.Result
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)
		eventRepo := repository.NewEventRepository(tx)
		prevMsgRepo := s.messageRepo
		prevEventRepo := s.eventRepo
		s.messageRepo = msgRepo
		s.eventRepo = eventRepo
		defer func() {
			s.messageRepo = prevMsgRepo
			s.eventRepo = prevEventRepo
		}()

		res, err := s.executeSendMessageDirect(ctx, cmd)
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	if err != nil {
		return commands.Result{}, err
	}
	return result, nil
}

func (s *MessageService) executeSendMessageDirect(ctx context.Context, cmd commands.SendMessageCommand) (commands.Result, error) {
	if cmd.IdempotencyKey() != "" {
		if _, err := s.eventRepo.GetCommandLogByIdempotencyKey(ctx, cmd.IdempotencyKey()); err == nil {
			return commands.Result{}, commands.ErrDuplicateCommand
		} else if err != nil && err != sentinal_errors.ErrNotFound {
			return commands.Result{}, err
		}
	}

	if s.access != nil {
		if err := s.access.CanSendMessage(ctx, cmd.SenderID, cmd.ConversationID); err != nil {
			return commands.Result{}, err
		}
	}

	msg := message.Message{
		ID:             uuid.New(),
		ConversationID: cmd.ConversationID,
		SenderID:       cmd.SenderID,
		Content:        msgNullString(cmd.Content),
		Type:           "TEXT",
		CreatedAt:      time.Now(),
	}
	if cmd.ClientMsgID != "" {
		msg.ClientMessageID = msgNullString(cmd.ClientMsgID)
	}
	if cmd.IdempotencyKey() != "" {
		msg.IdempotencyKey = msgNullString(cmd.IdempotencyKey())
	}

	if err := s.messageRepo.Create(ctx, &msg); err != nil {
		return commands.Result{}, err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return commands.Result{}, err
	}

	outbox := &event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "message",
		AggregateID:   msg.ID,
		EventType:     "message.created",
		Payload:       string(payload),
		CreatedAt:     time.Now(),
	}
	if err := s.eventRepo.CreateOutboxEvent(ctx, outbox); err != nil {
		return commands.Result{}, err
	}

	if cmd.IdempotencyKey() != "" {
		log := &event.CommandLog{
			ID:             uuid.New(),
			CommandType:    cmd.CommandType(),
			ActorID:        uuid.NullUUID{UUID: cmd.SenderID, Valid: true},
			AggregateType:  "message",
			AggregateID:    uuid.NullUUID{UUID: msg.ID, Valid: true},
			Payload:        string(payload),
			IdempotencyKey: msgNullString(cmd.IdempotencyKey()),
			Status:         "EXECUTED",
			CreatedAt:      time.Now(),
			ExecutedAt:     msgNullTime(time.Now()),
		}
		_ = s.eventRepo.CreateCommandLog(ctx, log)
	}

	return commands.Result{AggregateID: msg.ID.String(), Payload: msg}, nil
}

func msgNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func msgNullTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}
