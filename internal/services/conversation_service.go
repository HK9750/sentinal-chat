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
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConversationService struct {
	db        *gorm.DB
	repo      repository.ConversationRepository
	eventRepo repository.EventRepository
	access    *proxy.AccessControl
	bus       *commands.Bus
}

func NewConversationService(db *gorm.DB, repo repository.ConversationRepository, eventRepo repository.EventRepository, access *proxy.AccessControl, bus *commands.Bus) *ConversationService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &ConversationService{db: db, repo: repo, eventRepo: eventRepo, access: access, bus: bus}
	svc.RegisterHandlers(bus)
	return svc
}

func (s *ConversationService) RegisterHandlers(bus *commands.Bus) {
	if bus == nil {
		return
	}
	bus.Register("conversation.create", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		typed, ok := cmd.(commands.CreateConversationCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		return s.executeCreate(ctx, typed)
	}))
	bus.Register("conversation.update", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SimpleCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		conv, ok := c.Payload.(conversation.Conversation)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.Update(ctx, conv); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: conv.ID.String(), Payload: conv}, nil
	}))
}

func (s *ConversationService) Create(ctx context.Context, cmd commands.CreateConversationCommand) (commands.Result, error) {
	if err := cmd.Validate(); err != nil {
		return commands.Result{}, err
	}
	if s.bus != nil {
		return s.bus.Execute(ctx, cmd)
	}
	return s.executeCreate(ctx, cmd)
}

func (s *ConversationService) executeCreate(ctx context.Context, cmd commands.CreateConversationCommand) (commands.Result, error) {
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

func (s *ConversationService) GetByID(ctx context.Context, conversationID uuid.UUID) (conversation.Conversation, error) {
	return s.repo.GetByID(ctx, conversationID)
}

func (s *ConversationService) Update(ctx context.Context, conv conversation.Conversation) error {
	conv.UpdatedAt = time.Now()
	return s.repo.Update(ctx, conv)
}

func (s *ConversationService) Delete(ctx context.Context, conversationID uuid.UUID) error {
	return s.repo.Delete(ctx, conversationID)
}

func (s *ConversationService) GetDirectConversation(ctx context.Context, userID1, userID2 uuid.UUID) (conversation.Conversation, error) {
	return s.repo.GetDirectConversation(ctx, userID1, userID2)
}

func (s *ConversationService) SearchConversations(ctx context.Context, userID uuid.UUID, query string) ([]conversation.Conversation, error) {
	return s.repo.SearchConversations(ctx, userID, query)
}

func (s *ConversationService) GetConversationsByType(ctx context.Context, userID uuid.UUID, convType string) ([]conversation.Conversation, error) {
	return s.repo.GetConversationsByType(ctx, userID, convType)
}

func (s *ConversationService) GetByInviteLink(ctx context.Context, link string) (conversation.Conversation, error) {
	return s.repo.GetByInviteLink(ctx, link)
}

func (s *ConversationService) RegenerateInviteLink(ctx context.Context, conversationID uuid.UUID) (string, error) {
	return s.repo.RegenerateInviteLink(ctx, conversationID)
}

func (s *ConversationService) AddParticipant(ctx context.Context, p *conversation.Participant) error {
	return s.repo.AddParticipant(ctx, p)
}

func (s *ConversationService) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.RemoveParticipant(ctx, conversationID, userID)
}

func (s *ConversationService) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]conversation.Participant, error) {
	return s.repo.GetParticipants(ctx, conversationID)
}

func (s *ConversationService) GetParticipant(ctx context.Context, conversationID, userID uuid.UUID) (conversation.Participant, error) {
	return s.repo.GetParticipant(ctx, conversationID, userID)
}

func (s *ConversationService) UpdateParticipantRole(ctx context.Context, conversationID, userID uuid.UUID, role string) error {
	return s.repo.UpdateParticipantRole(ctx, conversationID, userID, role)
}

func (s *ConversationService) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	return s.repo.IsParticipant(ctx, conversationID, userID)
}

func (s *ConversationService) GetParticipantCount(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	return s.repo.GetParticipantCount(ctx, conversationID)
}

func (s *ConversationService) MuteConversation(ctx context.Context, conversationID, userID uuid.UUID, until time.Time) error {
	return s.repo.MuteConversation(ctx, conversationID, userID, until)
}

func (s *ConversationService) UnmuteConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.UnmuteConversation(ctx, conversationID, userID)
}

func (s *ConversationService) PinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.PinConversation(ctx, conversationID, userID)
}

func (s *ConversationService) UnpinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.UnpinConversation(ctx, conversationID, userID)
}

func (s *ConversationService) ArchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.ArchiveConversation(ctx, conversationID, userID)
}

func (s *ConversationService) UnarchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	return s.repo.UnarchiveConversation(ctx, conversationID, userID)
}

func (s *ConversationService) UpdateLastReadSequence(ctx context.Context, conversationID, userID uuid.UUID, seqID int64) error {
	return s.repo.UpdateLastReadSequence(ctx, conversationID, userID, seqID)
}

func (s *ConversationService) GetConversationSequence(ctx context.Context, conversationID uuid.UUID) (conversation.ConversationSequence, error) {
	return s.repo.GetConversationSequence(ctx, conversationID)
}

func (s *ConversationService) IncrementSequence(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	return s.repo.IncrementSequence(ctx, conversationID)
}

func convNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
