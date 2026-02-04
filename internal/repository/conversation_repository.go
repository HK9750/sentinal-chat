package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/conversation"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &PostgresConversationRepository{db: db}
}

func (r *PostgresConversationRepository) Create(ctx context.Context, c *conversation.Conversation) error {
	res := r.db.WithContext(ctx).Create(c)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (conversation.Conversation, error) {
	var c conversation.Conversation
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("id = ?", id).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}
	return c, nil
}

func (r *PostgresConversationRepository) Update(ctx context.Context, c conversation.Conversation) error {
	res := r.db.WithContext(ctx).Save(&c)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&conversation.Conversation{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) GetUserConversations(ctx context.Context, userID uuid.UUID, page, limit int) ([]conversation.Conversation, int64, error) {
	var conversations []conversation.Conversation
	var total int64

	subQuery := r.db.Model(&conversation.Participant{}).
		Select("conversation_id").
		Where("user_id = ?", userID)

	q := r.db.WithContext(ctx).
		Model(&conversation.Conversation{}).
		Where("id IN (?)", subQuery)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.
		Preload("Participants").
		Order("updated_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&conversations).Error; err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

func (r *PostgresConversationRepository) GetDirectConversation(ctx context.Context, userID1, userID2 uuid.UUID) (conversation.Conversation, error) {
	var c conversation.Conversation

	// Find DIRECT conversation where both users are participants
	subQuery := r.db.Model(&conversation.Participant{}).
		Select("conversation_id").
		Where("user_id IN (?, ?)", userID1, userID2).
		Group("conversation_id").
		Having("COUNT(DISTINCT user_id) = 2")

	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("id IN (?) AND type = ?", subQuery, "DIRECT").
		First(&c).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}
	return c, nil
}

func (r *PostgresConversationRepository) SearchConversations(ctx context.Context, userID uuid.UUID, query string) ([]conversation.Conversation, error) {
	var conversations []conversation.Conversation

	subQuery := r.db.Model(&conversation.Participant{}).
		Select("conversation_id").
		Where("user_id = ?", userID)

	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("id IN (?) AND subject ILIKE ?", subQuery, "%"+query+"%").
		Find(&conversations).Error

	if err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *PostgresConversationRepository) GetConversationsByType(ctx context.Context, userID uuid.UUID, convType string) ([]conversation.Conversation, error) {
	var conversations []conversation.Conversation

	subQuery := r.db.Model(&conversation.Participant{}).
		Select("conversation_id").
		Where("user_id = ?", userID)

	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("id IN (?) AND type = ?", subQuery, convType).
		Find(&conversations).Error

	if err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *PostgresConversationRepository) GetByInviteLink(ctx context.Context, link string) (conversation.Conversation, error) {
	var c conversation.Conversation
	err := r.db.WithContext(ctx).
		Preload("Participants").
		Where("invite_link = ? AND (invite_link_revoked_at IS NULL OR invite_link_revoked_at > NOW())", link).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.Conversation{}, sentinal_errors.ErrNotFound
		}
		return conversation.Conversation{}, err
	}
	return c, nil
}

func (r *PostgresConversationRepository) RegenerateInviteLink(ctx context.Context, conversationID uuid.UUID) (string, error) {
	newLink := uuid.New().String()
	res := r.db.WithContext(ctx).
		Model(&conversation.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]interface{}{
			"invite_link":            newLink,
			"invite_link_revoked_at": nil,
		})
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", sentinal_errors.ErrNotFound
	}
	return newLink, nil
}

func (r *PostgresConversationRepository) AddParticipant(ctx context.Context, p *conversation.Participant) error {
	res := r.db.WithContext(ctx).Create(p)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresConversationRepository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&conversation.Participant{}, "conversation_id = ? AND user_id = ?", conversationID, userID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]conversation.Participant, error) {
	var participants []conversation.Participant
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Find(&participants).Error
	if err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *PostgresConversationRepository) GetParticipant(ctx context.Context, conversationID, userID uuid.UUID) (conversation.Participant, error) {
	var p conversation.Participant
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.Participant{}, sentinal_errors.ErrNotFound
		}
		return conversation.Participant{}, err
	}
	return p, nil
}

func (r *PostgresConversationRepository) UpdateParticipantRole(ctx context.Context, conversationID, userID uuid.UUID, role string) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("role", role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresConversationRepository) GetParticipantCount(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ?", conversationID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresConversationRepository) MuteConversation(ctx context.Context, conversationID, userID uuid.UUID, until time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("muted_until", until)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) UnmuteConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("muted_until", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) PinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("pinned_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) UnpinConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("pinned_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) ArchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("archived", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) UnarchiveConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("archived", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) UpdateLastReadSequence(ctx context.Context, conversationID, userID uuid.UUID, seqID int64) error {
	res := r.db.WithContext(ctx).
		Model(&conversation.Participant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("last_read_sequence", seqID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresConversationRepository) GetConversationSequence(ctx context.Context, conversationID uuid.UUID) (conversation.ConversationSequence, error) {
	var seq conversation.ConversationSequence
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		First(&seq).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conversation.ConversationSequence{}, sentinal_errors.ErrNotFound
		}
		return conversation.ConversationSequence{}, err
	}
	return seq, nil
}

func (r *PostgresConversationRepository) IncrementSequence(ctx context.Context, conversationID uuid.UUID) (int64, error) {
	var seq conversation.ConversationSequence

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Try to get existing sequence
		err := tx.Where("conversation_id = ?", conversationID).First(&seq).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create new sequence starting at 1
				seq = conversation.ConversationSequence{
					ConversationID: conversationID,
					LastSequence:   1,
					UpdatedAt:      time.Now(),
				}
				return tx.Create(&seq).Error
			}
			return err
		}

		// Increment existing sequence
		seq.LastSequence++
		seq.UpdatedAt = time.Now()
		return tx.Save(&seq).Error
	})

	if err != nil {
		return 0, err
	}
	return seq.LastSequence, nil
}
