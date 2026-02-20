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
	ID              uuid.UUID
	CommandType     string
	UserID          uuid.UUID
	Status          Status
	Payload         []byte
	Result          []byte
	UndoData        []byte
	ErrorMessage    string
	ExecutionTimeMs int
	CreatedAt       time.Time
	ExecutedAt      *time.Time
	UndoneAt        *time.Time
}

// TableName returns the database table name
func (CommandLog) TableName() string {
	return "command_logs"
}

// ScheduledMessage for delayed delivery
type ScheduledMessage struct {
	ID             uuid.UUID
	MessageID      uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Content        string
	ScheduledFor   time.Time
	Timezone       string
	Status         string
	CreatedAt      time.Time
	SentAt         *time.Time
}

// TableName returns the database table name
func (ScheduledMessage) TableName() string {
	return "scheduled_messages"
}

// MessageVersion for edit history
type MessageVersion struct {
	ID            uuid.UUID
	MessageID     uuid.UUID
	Content       string
	EditedBy      uuid.UUID
	EditedAt      time.Time
	VersionNumber int
}

// TableName returns the database table name
func (MessageVersion) TableName() string {
	return "message_versions"
}
