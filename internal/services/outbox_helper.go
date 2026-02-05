package services

import (
	"context"
	"encoding/json"
	"time"

	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

func createOutboxEvent(ctx context.Context, repo repository.EventRepository, aggregateType, eventType string, aggregateID uuid.UUID, payload interface{}) error {
	if repo == nil {
		return nil
	}
	data := []byte("{}")
	if payload != nil {
		if raw, err := json.Marshal(payload); err == nil {
			data = raw
		}
	}
	return repo.CreateOutboxEvent(ctx, &event.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       string(data),
		CreatedAt:     time.Now(),
	})
}
