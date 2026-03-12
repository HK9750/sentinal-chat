package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/events"
)

type EventEnvelope struct {
	Type           string         `json:"type"`
	ConversationID string         `json:"conversation_id,omitempty"`
	CallID         string         `json:"call_id,omitempty"`
	SentAt         time.Time      `json:"sent_at"`
	Data           map[string]any `json:"data,omitempty"`
}

func NewOutboxEvent(eventType string, aggregateType outbox.AggregateType, aggregateID uuid.UUID, payload EventEnvelope) (*outbox.OutboxEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &outbox.OutboxEvent{
		ID:            uuid.New(),
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID.String(),
		Payload:       body,
		Status:        outbox.StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func NewConversationEvent(eventType string, conversationID uuid.UUID, data map[string]any) EventEnvelope {
	return EventEnvelope{
		Type:           eventType,
		ConversationID: conversationID.String(),
		SentAt:         time.Now().UTC(),
		Data:           data,
	}
}

func NewMessageEvent(eventType string, conversationID uuid.UUID, data map[string]any) EventEnvelope {
	return NewConversationEvent(eventType, conversationID, data)
}

func NewCallEvent(eventType string, conversationID, callID uuid.UUID, data map[string]any) EventEnvelope {
	return EventEnvelope{
		Type:           eventType,
		ConversationID: conversationID.String(),
		CallID:         callID.String(),
		SentAt:         time.Now().UTC(),
		Data:           data,
	}
}

func ConnectionReadyEnvelope(userID, sessionID, deviceID string) EventEnvelope {
	return EventEnvelope{
		Type:   events.ConnectionReady,
		SentAt: time.Now().UTC(),
		Data: map[string]any{
			"user_id":    userID,
			"session_id": sessionID,
			"device_id":  deviceID,
		},
	}
}
