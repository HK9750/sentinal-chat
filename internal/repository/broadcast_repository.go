package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/broadcast"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresBroadcastRepository struct {
	db *gorm.DB
}

func NewBroadcastRepository(db *gorm.DB) BroadcastRepository {
	return &PostgresBroadcastRepository{db: db}
}

func (r *PostgresBroadcastRepository) Create(ctx context.Context, b *broadcast.BroadcastList) error {
	res := r.db.WithContext(ctx).Create(b)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresBroadcastRepository) GetByID(ctx context.Context, id uuid.UUID) (broadcast.BroadcastList, error) {
	var b broadcast.BroadcastList
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return broadcast.BroadcastList{}, sentinal_errors.ErrNotFound
		}
		return broadcast.BroadcastList{}, err
	}
	return b, nil
}

func (r *PostgresBroadcastRepository) Update(ctx context.Context, b broadcast.BroadcastList) error {
	res := r.db.WithContext(ctx).Save(&b)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresBroadcastRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&broadcast.BroadcastList{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresBroadcastRepository) GetUserBroadcastLists(ctx context.Context, ownerID uuid.UUID) ([]broadcast.BroadcastList, error) {
	var lists []broadcast.BroadcastList
	err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("created_at DESC").
		Find(&lists).Error
	if err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *PostgresBroadcastRepository) SearchBroadcastLists(ctx context.Context, ownerID uuid.UUID, query string) ([]broadcast.BroadcastList, error) {
	var lists []broadcast.BroadcastList
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND name ILIKE ?", ownerID, "%"+query+"%").
		Order("name ASC").
		Find(&lists).Error
	if err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *PostgresBroadcastRepository) AddRecipient(ctx context.Context, rec *broadcast.BroadcastRecipient) error {
	res := r.db.WithContext(ctx).Create(rec)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresBroadcastRepository) RemoveRecipient(ctx context.Context, broadcastID, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&broadcast.BroadcastRecipient{}, "broadcast_id = ? AND user_id = ?", broadcastID, userID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresBroadcastRepository) GetRecipients(ctx context.Context, broadcastID uuid.UUID) ([]broadcast.BroadcastRecipient, error) {
	var recipients []broadcast.BroadcastRecipient
	err := r.db.WithContext(ctx).
		Where("broadcast_id = ?", broadcastID).
		Order("added_at ASC").
		Find(&recipients).Error
	if err != nil {
		return nil, err
	}
	return recipients, nil
}

func (r *PostgresBroadcastRepository) GetRecipientCount(ctx context.Context, broadcastID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&broadcast.BroadcastRecipient{}).
		Where("broadcast_id = ?", broadcastID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresBroadcastRepository) IsRecipient(ctx context.Context, broadcastID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&broadcast.BroadcastRecipient{}).
		Where("broadcast_id = ? AND user_id = ?", broadcastID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresBroadcastRepository) BulkAddRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, userID := range userIDs {
			recipient := &broadcast.BroadcastRecipient{
				BroadcastID: broadcastID,
				UserID:      userID,
				AddedAt:     now,
			}
			res := tx.Create(recipient)
			if res.Error != nil {
				// Skip duplicates
				if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
					continue
				}
				return res.Error
			}
		}
		return nil
	})
}

func (r *PostgresBroadcastRepository) BulkRemoveRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&broadcast.BroadcastRecipient{}, "broadcast_id = ? AND user_id IN ?", broadcastID, userIDs)
	if res.Error != nil {
		return res.Error
	}
	return nil
}
