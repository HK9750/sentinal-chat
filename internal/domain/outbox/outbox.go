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
	ID            uuid.UUID
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       []byte
	Status        Status
	RetryCount    int
	Error         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ProcessedAt   *time.Time
}

// TableName returns the database table name
func (OutboxEvent) TableName() string {
	return "outbox_events"
}
