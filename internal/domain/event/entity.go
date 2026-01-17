package event

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// OutboxEvent represents outbox_events
type OutboxEvent struct {
	ID            uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	AggregateType string        `gorm:"not null"`
	AggregateID   uuid.UUID     `gorm:"type:uuid;not null"`
	EventType     string        `gorm:"not null"`
	Payload       string        `gorm:"type:jsonb;not null"`
	CorrelationID uuid.NullUUID `gorm:"type:uuid"`
	CreatedAt     time.Time     `gorm:"default:now()"`
	ProcessedAt   sql.NullTime
	RetryCount    int `gorm:"default:0"`
	MaxRetries    int `gorm:"default:5"`
	NextRetryAt   sql.NullTime
	ErrorMessage  sql.NullString
}

// CommandLog represents command_log
type CommandLog struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	CommandType    string        `gorm:"not null"`
	ActorID        uuid.NullUUID `gorm:"type:uuid"`
	AggregateType  string        `gorm:"not null"`
	AggregateID    uuid.NullUUID `gorm:"type:uuid"`
	Payload        string        `gorm:"type:jsonb;not null"`
	IdempotencyKey sql.NullString
	Status         string    `gorm:"type:command_status;default:'PENDING'"`
	CreatedAt      time.Time `gorm:"default:now()"`
	ExecutedAt     sql.NullTime
	ErrorMessage   sql.NullString
}

// AccessPolicy represents access_policies
type AccessPolicy struct {
	ID           uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ResourceType string        `gorm:"not null"`
	ResourceID   uuid.NullUUID `gorm:"type:uuid"`
	ActorType    string        `gorm:"not null"`
	ActorID      uuid.NullUUID `gorm:"type:uuid"`
	Permission   string        `gorm:"not null"`
	Granted      bool          `gorm:"default:true"`
	CreatedAt    time.Time     `gorm:"default:now()"`
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}

func (CommandLog) TableName() string {
	return "command_log"
}

func (AccessPolicy) TableName() string {
	return "access_policies"
}
