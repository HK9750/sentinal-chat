package repository

import (
	"context"
	"time"

	"sentinal-chat/internal/domain/outbox"
)

type outboxRepository struct {
	db DBTX
}

func NewOutboxRepository(db DBTX) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Create(ctx context.Context, tx DBTX, event *outbox.OutboxEvent) error {
	execDB := tx
	if execDB == nil {
		execDB = r.db
	}
	_, err := execDB.ExecContext(ctx, `
        INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, status, retry_count, error, created_at, updated_at, processed_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `,
		event.ID,
		event.EventType,
		event.AggregateType,
		event.AggregateID,
		event.Payload,
		event.Status,
		event.RetryCount,
		event.Error,
		event.CreatedAt,
		event.UpdatedAt,
		event.ProcessedAt,
	)
	return err
}

func (r *outboxRepository) GetPending(ctx context.Context, limit int) ([]outbox.OutboxEvent, error) {
	var events []outbox.OutboxEvent
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, event_type, aggregate_type, aggregate_id, payload, status, retry_count, error, created_at, updated_at, processed_at
        FROM outbox_events
        WHERE status = $1 AND retry_count < $2
        ORDER BY created_at ASC
        LIMIT $3
    `, outbox.StatusPending, 10, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event outbox.OutboxEvent
		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.AggregateType,
			&event.AggregateID,
			&event.Payload,
			&event.Status,
			&event.RetryCount,
			&event.Error,
			&event.CreatedAt,
			&event.UpdatedAt,
			&event.ProcessedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *outboxRepository) MarkProcessing(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status = $1, updated_at = $2
        WHERE id = $3
    `, outbox.StatusProcessing, time.Now(), id)
	return err
}

func (r *outboxRepository) MarkCompleted(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status = $1, processed_at = $2, updated_at = $3
        WHERE id = $4
    `, outbox.StatusCompleted, &now, now, id)
	return err
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET status = $1, error = $2, updated_at = $3
        WHERE id = $4
    `, outbox.StatusFailed, errorMsg, time.Now(), id)
	return err
}

func (r *outboxRepository) IncrementRetry(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE outbox_events
        SET retry_count = retry_count + 1
        WHERE id = $1
    `, id)
	return err
}
