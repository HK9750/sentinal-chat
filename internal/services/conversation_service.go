// Package services provides business logic for chat operations.
package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationService manages chat conversations and participants.
type ConversationService struct {
	db             *gorm.DB
	repo           repository.ConversationRepository
	eventPublisher *EventPublisher
}

// CreateConversationInput contains data needed to create a conversation.
type CreateConversationInput struct {
	Type           string
	Subject        string
	Description    string
	CreatorID      uuid.UUID
	ParticipantIDs []uuid.UUID
}

// NewConversationService creates a conversation service with dependencies.
func NewConversationService(db *gorm.DB, repo repository.ConversationRepository, eventPublisher *EventPublisher) *ConversationService {
	return &ConversationService{db: db, repo: repo, eventPublisher: eventPublisher}
}

// Create validates input and creates a new conversation.
func (s *ConversationService) Create(ctx context.Context, input CreateConversationInput) (conversation.Conversation, error) {
	if input.CreatorID == uuid.Nil {
		return conversation.Conversation{}, sentinal_errors.ErrInvalidInput
	}
	if input.Type != "DM" && input.Type != "GROUP" {
		return conversation.Conversation{}, sentinal_errors.ErrInvalidInput
	}
	if input.Type == "GROUP" && input.Subject == "" {
		return conversation.Conversation{}, sentinal_errors.ErrInvalidInput
	}
	if len(input.ParticipantIDs) == 0 {
		return conversation.Conversation{}, sentinal_errors.ErrInvalidInput
	}
	fmt.Println("input",input);
	return s.executeCreate(ctx, input)
}

// executeCreate runs the conversation creation in a transaction.
func (s *ConversationService) executeCreate(ctx context.Context, input CreateConversationInput) (conversation.Conversation, error) {
	if s.db == nil {
		return s.createDirect(ctx, input)
	}

	var result conversation.Conversation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepo := repository.NewConversationRepository(tx)
		prevRepo := s.repo
		s.repo = convRepo
		defer func() {
			s.repo = prevRepo
		}()

		res, err := s.createDirect(ctx, input)
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	if err != nil {
		return conversation.Conversation{}, err
	}
	return result, nil
}

// createDirect creates conversation and adds participants.
func (s *ConversationService) createDirect(ctx context.Context, input CreateConversationInput) (conversation.Conversation, error) {
	conv := conversation.Conversation{
		ID:               uuid.New(),
		Type:             input.Type,
		Subject:          convNullString(input.Subject),
		Description:      convNullString(input.Description),
		CreatedBy:        uuid.NullUUID{UUID: input.CreatorID, Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		DisappearingMode: "OFF",
	}

	if err := s.repo.Create(ctx, &conv); err != nil {
		return conversation.Conversation{}, err
	}

	for _, participantID := range input.ParticipantIDs {
		role := "MEMBER"
		if participantID == input.CreatorID {
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

	return conv, nil
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

func (s *ConversationService) StartTyping(ctx context.Context, conversationID, userID uuid.UUID, displayName string) error {
	if s.eventPublisher == nil || s.db == nil {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.eventPublisher.PublishTypingStarted(ctx, tx, conversationID, userID, displayName)
	})
}

func (s *ConversationService) StopTyping(ctx context.Context, conversationID, userID uuid.UUID, displayName string) error {
	if s.eventPublisher == nil || s.db == nil {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.eventPublisher.PublishTypingStopped(ctx, tx, conversationID, userID, displayName)
	})
}

func convNullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
