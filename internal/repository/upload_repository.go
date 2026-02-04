package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/upload"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresUploadRepository struct {
	db *gorm.DB
}

func NewUploadRepository(db *gorm.DB) UploadRepository {
	return &PostgresUploadRepository{db: db}
}

func (r *PostgresUploadRepository) Create(ctx context.Context, u *upload.UploadSession) error {
	res := r.db.WithContext(ctx).Create(u)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUploadRepository) GetByID(ctx context.Context, id uuid.UUID) (upload.UploadSession, error) {
	var u upload.UploadSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return upload.UploadSession{}, sentinal_errors.ErrNotFound
		}
		return upload.UploadSession{}, err
	}
	return u, nil
}

func (r *PostgresUploadRepository) Update(ctx context.Context, u upload.UploadSession) error {
	u.UpdatedAt = time.Now()
	res := r.db.WithContext(ctx).Save(&u)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUploadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&upload.UploadSession{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUploadRepository) GetUserUploadSessions(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	err := r.db.WithContext(ctx).
		Where("uploader_id = ?", uploaderID).
		Order("created_at DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) GetInProgressUploads(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	err := r.db.WithContext(ctx).
		Where("uploader_id = ? AND status = 'IN_PROGRESS'", uploaderID).
		Order("created_at DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) GetCompletedUploads(ctx context.Context, uploaderID uuid.UUID, page, limit int) ([]upload.UploadSession, int64, error) {
	var sessions []upload.UploadSession
	var total int64

	q := r.db.WithContext(ctx).
		Model(&upload.UploadSession{}).
		Where("uploader_id = ? AND status = 'COMPLETED'", uploaderID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

func (r *PostgresUploadRepository) UpdateProgress(ctx context.Context, sessionID uuid.UUID, uploadedBytes int64) error {
	res := r.db.WithContext(ctx).
		Model(&upload.UploadSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"uploaded_bytes": uploadedBytes,
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUploadRepository) MarkCompleted(ctx context.Context, sessionID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session upload.UploadSession
		if err := tx.Where("id = ?", sessionID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		return tx.Model(&upload.UploadSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]interface{}{
				"status":         "COMPLETED",
				"uploaded_bytes": session.SizeBytes,
				"updated_at":     time.Now(),
			}).Error
	})
}

func (r *PostgresUploadRepository) MarkFailed(ctx context.Context, sessionID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&upload.UploadSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":     "FAILED",
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUploadRepository) GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	cutoff := time.Now().Add(-olderThan)
	err := r.db.WithContext(ctx).
		Where("status = 'IN_PROGRESS' AND updated_at < ?", cutoff).
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res := r.db.WithContext(ctx).
		Delete(&upload.UploadSession{}, "status = 'IN_PROGRESS' AND updated_at < ?", cutoff)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
