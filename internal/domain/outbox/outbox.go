package outbox

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the processing state of an outbox event
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
)

// OutboxEvent stores domain events waiting to be published to Redis
type OutboxEvent struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	EventType     string    `gorm:"type:varchar(50);not null"`
	AggregateType string    `gorm:"type:varchar(50);not null"`
	AggregateID   string    `gorm:"type:varchar(36);not null"`
	Payload       []byte    `gorm:"type:jsonb;not null"`
	Status        Status    `gorm:"type:varchar(20);not null;default:'PENDING'"`
	RetryCount    int       `gorm:"default:0"`
	Error         string    `gorm:"type:text"`
	CreatedAt     time.Time `gorm:"not null;default:now()"`
	UpdatedAt     time.Time `gorm:"not null;default:now()"`
	ProcessedAt   *time.Time
}

// TableName returns the database table name
func (OutboxEvent) TableName() string {
	return "outbox_events"
}
