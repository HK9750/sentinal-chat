package command

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status represents command execution state (maps to command_status enum)
type Status string

const (
	StatusPending  Status = "PENDING"
	StatusExecuted Status = "EXECUTED"
	StatusFailed   Status = "FAILED"
	StatusUndone   Status = "UNDONE"
)

type CommandType string

const (
	CommandDeleteMessage CommandType = "DELETE_MESSAGE"
	CommandEditMessage   CommandType = "EDIT_MESSAGE"
	CommandPinMessage    CommandType = "PIN_MESSAGE"
	CommandUnpinMessage  CommandType = "UNPIN_MESSAGE"
	CommandClearChat     CommandType = "CLEAR_CHAT"
)

// CommandLog stores command execution history
type CommandLog struct {
	ID              uuid.UUID
	CommandType     CommandType
	UserID          uuid.UUID
	ConversationID  *uuid.UUID
	Status          Status
	Payload         json.RawMessage
	UndoPayload     json.RawMessage
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
