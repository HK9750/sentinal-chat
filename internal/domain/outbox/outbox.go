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

type AggregateType string

const (
	AggregateConversation AggregateType = "conversation"
	AggregateMessage      AggregateType = "message"
	AggregatePoll         AggregateType = "poll"
	AggregateCall         AggregateType = "call"
	AggregatePresence     AggregateType = "presence"
	AggregateNotification AggregateType = "notification"
)

// OutboxEvent stores domain events waiting to be published to Redis
type OutboxEvent struct {
	ID            uuid.UUID
	EventType     string
	AggregateType AggregateType
	AggregateID   string
	Payload       []byte
	Status        Status
	RetryCount    int
	Error         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ProcessedAt   *time.Time
	NextRetryAt   *time.Time
}

// TableName returns the database table name
func (OutboxEvent) TableName() string {
	return "outbox_events"
}
