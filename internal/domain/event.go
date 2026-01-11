package domain

import (
	"time"
)

type OutboxEvent struct {
	ID            string                 `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AggregateType string                 `gorm:"type:text;not null" json:"aggregate_type"`
	AggregateID   string                 `gorm:"type:uuid;not null" json:"aggregate_id"`
	EventType     string                 `gorm:"type:text;not null" json:"event_type"`
	Payload       map[string]interface{} `gorm:"type:jsonb;not null;serializer:json" json:"payload"`
	CreatedAt     time.Time              `gorm:"default:CURRENT_TIMESTAMP;index:idx_outbox_pending" json:"created_at"`
	ProcessedAt   *time.Time             `json:"processed_at,omitempty"`
}
