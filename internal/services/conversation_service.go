package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/proxy"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConversationService struct {
	db        *gorm.DB
	repo      repository.ConversationRepository
	eventRepo repository.EventRepository
	access    *proxy.AccessControl
}

func NewConversationService(db *gorm.DB, repo repository.ConversationRepository, eventRepo repository.EventRepository, access *proxy.AccessControl) *ConversationService {
	return &ConversationService{db: db, repo: repo, eventRepo: eventRepo, access: access}
}

func (s *ConversationService) Create(ctx context.Context, cmd commands.CreateConversationCommand) (commands.Result, error) {
	if err := cmd.Validate(); err != nil {
		return commands.Result{}, err
	}

	if s.db == nil {
		return s.createDirect(ctx, cmd)
	}

	var result commands.Result
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepo := repository.NewConversationRepository(tx)
		eventRepo := repository.NewEventRepository(tx)
		prevRepo := s.repo
		prevEvent := s.eventRepo
		s.repo = convRepo
		s.eventRepo = eventRepo
		defer func() {
			s.repo = prevRepo
			s.eventRepo = prevEvent
		}()

		res, err := s.createDirect(ctx, cmd)
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

func (s *ConversationService) createDirect(ctx context.Context, cmd commands.CreateConversationCommand) (commands.Result, error) {
	conv := conversation.Conversation{
		ID:               uuid.New(),
		Type:             cmd.Type,
		Subject:          convNullString(cmd.Subject),
		Description:      convNullString(cmd.Description),
		CreatedBy:        uuid.NullUUID{UUID: cmd.CreatorID, Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		DisappearingMode: "OFF",
	}

	if err := s.repo.Create(ctx, &conv); err != nil {
		return commands.Result{}, err
	}

	for _, participantID := range cmd.ParticipantIDs {
		role := "MEMBER"
		if participantID == cmd.CreatorID {
			role = "OWNER"
		}
		p := &conversation.Participant{
			ConversationID: conv.ID,
			UserID:         participantID,
			Role:           role,
			JoinedAt:       time.Now(),
		}
		_ = s.repo.AddParticipant(ctx, p)
	}

	payload, _ := json.Marshal(conv)
	_ = s.eventRepo.CreateOutboxEvent(ctx, &event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "conversation",
		AggregateID:   conv.ID,
		EventType:     "conversation.created",
		Payload:       string(payload),
		CreatedAt:     time.Now(),
	})

	return commands.Result{AggregateID: conv.ID.String(), Payload: conv}, nil
}

func (s *ConversationService) GetUserConversations(ctx context.Context, userID uuid.UUID, page, limit int) ([]conversation.Conversation, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.GetUserConversations(ctx, userID, page, limit)
}

func convNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
