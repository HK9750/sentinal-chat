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
}

func NewMessageService(db *gorm.DB, messageRepo repository.MessageRepository, eventRepo repository.EventRepository, access *proxy.AccessControl) *MessageService {
	return &MessageService{
		db:          db,
		messageRepo: messageRepo,
		eventRepo:   eventRepo,
		access:      access,
	}
}

func (s *MessageService) HandleSendMessage(ctx context.Context, cmd commands.SendMessageCommand) (commands.Result, error) {
	if err := cmd.Validate(); err != nil {
		return commands.Result{}, err
	}

	return s.executeSendMessage(ctx, cmd)
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
