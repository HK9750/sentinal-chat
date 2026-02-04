package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/event"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresEventRepository struct {
	db *gorm.DB
}

func NewEventRepository(db *gorm.DB) EventRepository {
	return &PostgresEventRepository{db: db}
}

func (r *PostgresEventRepository) CreateOutboxEvent(ctx context.Context, e *event.OutboxEvent) error {
	res := r.db.WithContext(ctx).Create(e)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEventRepository) GetOutboxEventByID(ctx context.Context, id uuid.UUID) (event.OutboxEvent, error) {
	var e event.OutboxEvent
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.OutboxEvent{}, sentinal_errors.ErrNotFound
		}
		return event.OutboxEvent{}, err
	}
	return e, nil
}

func (r *PostgresEventRepository) GetPendingOutboxEvents(ctx context.Context, limit int) ([]event.OutboxEvent, error) {
	var events []event.OutboxEvent
	q := r.db.WithContext(ctx).
		Where("processed_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= NOW())")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Order("created_at ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *PostgresEventRepository) MarkOutboxEventProcessed(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&event.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"processed_at":  time.Now(),
			"error_message": nil,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) MarkOutboxEventFailed(ctx context.Context, id uuid.UUID, nextRetryAt time.Time, errorMessage string) error {
	res := r.db.WithContext(ctx).
		Model(&event.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"next_retry_at": nextRetryAt,
			"error_message": errorMessage,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) CreateOutboxEventDelivery(ctx context.Context, d *event.OutboxEventDelivery) error {
	res := r.db.WithContext(ctx).Create(d)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEventRepository) GetOutboxEventDeliveries(ctx context.Context, eventID uuid.UUID) ([]event.OutboxEventDelivery, error) {
	var deliveries []event.OutboxEventDelivery
	err := r.db.WithContext(ctx).
		Where("event_id = ?", eventID).
		Order("attempt_number ASC").
		Find(&deliveries).Error
	if err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (r *PostgresEventRepository) CreateCommandLog(ctx context.Context, l *event.CommandLog) error {
	res := r.db.WithContext(ctx).Create(l)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEventRepository) GetCommandLogByID(ctx context.Context, id uuid.UUID) (event.CommandLog, error) {
	var l event.CommandLog
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&l).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.CommandLog{}, sentinal_errors.ErrNotFound
		}
		return event.CommandLog{}, err
	}
	return l, nil
}

func (r *PostgresEventRepository) GetCommandLogByIdempotencyKey(ctx context.Context, key string) (event.CommandLog, error) {
	var l event.CommandLog
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&l).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.CommandLog{}, sentinal_errors.ErrNotFound
		}
		return event.CommandLog{}, err
	}
	return l, nil
}

func (r *PostgresEventRepository) UpdateCommandLog(ctx context.Context, l event.CommandLog) error {
	res := r.db.WithContext(ctx).Save(&l)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) UpdateCommandStatus(ctx context.Context, id uuid.UUID, status string, executedAt time.Time, errorMessage string) error {
	updates := map[string]interface{}{
		"status":      status,
		"executed_at": executedAt,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	res := r.db.WithContext(ctx).
		Model(&event.CommandLog{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) CreateAccessPolicy(ctx context.Context, p *event.AccessPolicy) error {
	res := r.db.WithContext(ctx).Create(p)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEventRepository) GetAccessPolicyByID(ctx context.Context, id uuid.UUID) (event.AccessPolicy, error) {
	var p event.AccessPolicy
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.AccessPolicy{}, sentinal_errors.ErrNotFound
		}
		return event.AccessPolicy{}, err
	}
	return p, nil
}

func (r *PostgresEventRepository) UpdateAccessPolicy(ctx context.Context, p event.AccessPolicy) error {
	res := r.db.WithContext(ctx).Save(&p)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) DeleteAccessPolicy(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&event.AccessPolicy{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEventRepository) HasAccessPolicy(ctx context.Context, resourceType string, resourceID uuid.UUID, actorType string, actorID uuid.UUID, permission string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&event.AccessPolicy{}).
		Where("resource_type = ? AND actor_type = ? AND permission = ? AND granted = true", resourceType, actorType, permission).
		Where("resource_id IS NULL OR resource_id = ?", resourceID).
		Where("actor_id IS NULL OR actor_id = ?", actorID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresEventRepository) ListAccessPolicies(ctx context.Context, resourceType string, resourceID uuid.UUID, actorType string, actorID uuid.UUID, permission string, grantedOnly bool) ([]event.AccessPolicy, error) {
	var policies []event.AccessPolicy
	q := r.db.WithContext(ctx).Model(&event.AccessPolicy{})
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if resourceID != uuid.Nil {
		q = q.Where("resource_id = ?", resourceID)
	}
	if actorType != "" {
		q = q.Where("actor_type = ?", actorType)
	}
	if actorID != uuid.Nil {
		q = q.Where("actor_id = ?", actorID)
	}
	if permission != "" {
		q = q.Where("permission = ?", permission)
	}
	if grantedOnly {
		q = q.Where("granted = true")
	}
	err := q.Order("created_at DESC").Find(&policies).Error
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *PostgresEventRepository) CreateEventSubscription(ctx context.Context, s *event.EventSubscription) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEventRepository) GetEventSubscription(ctx context.Context, subscriberName, eventType string) (event.EventSubscription, error) {
	var s event.EventSubscription
	err := r.db.WithContext(ctx).
		Where("subscriber_name = ? AND event_type = ?", subscriberName, eventType).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.EventSubscription{}, sentinal_errors.ErrNotFound
		}
		return event.EventSubscription{}, err
	}
	return s, nil
}

func (r *PostgresEventRepository) GetActiveEventSubscriptions(ctx context.Context, eventTypes []string) ([]event.EventSubscription, error) {
	var subs []event.EventSubscription
	q := r.db.WithContext(ctx).Where("is_active = true")
	if len(eventTypes) > 0 {
		q = q.Where("event_type IN ?", eventTypes)
	}
	err := q.Order("created_at ASC").Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *PostgresEventRepository) UpdateEventSubscriptionStatus(ctx context.Context, subscriberName, eventType string, isActive bool) error {
	res := r.db.WithContext(ctx).
		Model(&event.EventSubscription{}).
		Where("subscriber_name = ? AND event_type = ?", subscriberName, eventType).
		Update("is_active", isActive)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}
