package command

import (
	"time"

	"github.com/google/uuid"
)

// Status represents command execution state
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusExecuting Status = "EXECUTING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusUndone    Status = "UNDONE"
)

// CommandLog stores command execution history
type CommandLog struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CommandType     string    `gorm:"type:varchar(50);not null"`
	UserID          uuid.UUID `gorm:"type:uuid;not null"`
	Status          Status    `gorm:"type:varchar(20);not null;default:'PENDING'"`
	Payload         []byte    `gorm:"type:jsonb;not null"`
	Result          []byte    `gorm:"type:jsonb"`
	UndoData        []byte    `gorm:"type:jsonb"`
	ErrorMessage    string    `gorm:"type:text"`
	ExecutionTimeMs int       `gorm:"type:int"`
	CreatedAt       time.Time `gorm:"not null;default:now()"`
	ExecutedAt      *time.Time
	UndoneAt        *time.Time
}

// TableName returns the database table name
func (CommandLog) TableName() string {
	return "command_logs"
}

// ScheduledMessage for delayed delivery
type ScheduledMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	MessageID      uuid.UUID `gorm:"type:uuid;not null"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null"`
	SenderID       uuid.UUID `gorm:"type:uuid;not null"`
	Content        string    `gorm:"type:text;not null"`
	ScheduledFor   time.Time `gorm:"not null"`
	Timezone       string    `gorm:"type:varchar(50);default:'UTC'"`
	Status         string    `gorm:"type:varchar(20);default:'PENDING'"`
	CreatedAt      time.Time `gorm:"default:now()"`
	SentAt         *time.Time
}

// TableName returns the database table name
func (ScheduledMessage) TableName() string {
	return "scheduled_messages"
}

// MessageVersion for edit history
type MessageVersion struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	MessageID     uuid.UUID `gorm:"type:uuid;not null"`
	Content       string    `gorm:"type:text;not null"`
	EditedBy      uuid.UUID `gorm:"type:uuid;not null"`
	EditedAt      time.Time `gorm:"not null;default:now()"`
	VersionNumber int       `gorm:"not null"`
}

// TableName returns the database table name
func (MessageVersion) TableName() string {
	return "message_versions"
}
