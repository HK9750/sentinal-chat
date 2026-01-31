package event

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// EventSubscription represents event_subscriptions
type EventSubscription struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	SubscriberName string    `gorm:"not null"`
	EventType      string    `gorm:"not null"`
	IsActive       bool      `gorm:"default:true"`
	CreatedAt      time.Time `gorm:"default:now()"`
}

// OutboxEventDelivery represents outbox_event_deliveries
type OutboxEventDelivery struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	EventID       uuid.UUID `gorm:"type:uuid;not null"`
	AttemptNumber int       `gorm:"not null"`
	Status        string    `gorm:"not null"`
	ErrorMessage  sql.NullString
	DeliveredAt   sql.NullTime
	CreatedAt     time.Time `gorm:"default:now()"`
}

func (EventSubscription) TableName() string {
	return "event_subscriptions"
}

func (OutboxEventDelivery) TableName() string {
	return "outbox_event_deliveries"
}
