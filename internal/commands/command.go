package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Command interface that all commands must implement
type Command interface {
	GetID() uuid.UUID
	GetType() string
	GetUserID() uuid.UUID
	Validate() error
	Execute(ctx context.Context) error
	Undo(ctx context.Context) error
	CanUndo() bool
	GetUndoDeadline() time.Time
	ToJSON() ([]byte, error)
}

// CommandResult stores execution outcome
type CommandResult struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// BaseCommand provides common fields
type BaseCommand struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// GetID returns the command ID
func (b *BaseCommand) GetID() uuid.UUID {
	return b.ID
}

// GetType returns the command type
func (b *BaseCommand) GetType() string {
	return b.Type
}

// GetUserID returns the user ID
func (b *BaseCommand) GetUserID() uuid.UUID {
	return b.UserID
}

// GetUndoDeadline returns the deadline for undoing (5 minutes)
func (b *BaseCommand) GetUndoDeadline() time.Time {
	return b.CreatedAt.Add(5 * time.Minute)
}

// ToJSON serializes the command to JSON
func (b *BaseCommand) ToJSON() ([]byte, error) {
	return json.Marshal(b)
}

// CanUndo checks if the command can be undone
func (b *BaseCommand) CanUndo() bool {
	return time.Now().Before(b.GetUndoDeadline())
}
