package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"sentinal-chat/internal/domain/outbox"
)

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Create(ctx context.Context, tx *gorm.DB, event *outbox.OutboxEvent) error {
	return tx.WithContext(ctx).Create(event).Error
}

func (r *outboxRepository) GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error) {
	var events []outbox.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", outbox.StatusPending, 10).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *outboxRepository) MarkProcessing(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&outbox.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     outbox.StatusProcessing,
			"updated_at": time.Now(),
		}).Error
}

func (r *outboxRepository) MarkCompleted(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&outbox.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       outbox.StatusCompleted,
			"processed_at": &now,
			"updated_at":   now,
		}).Error
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&outbox.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     outbox.StatusFailed,
			"error":      errorMsg,
			"updated_at": time.Now(),
		}).Error
}

func (r *outboxRepository) IncrementRetry(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&outbox.OutboxEvent{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}
