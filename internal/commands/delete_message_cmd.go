package commands

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// DeleteMessageCommand deletes a message with undo support
type DeleteMessageCommand struct {
	BaseCommand
	MessageID       uuid.UUID `json:"message_id"`
	DeleteForAll    bool      `json:"delete_for_all"`
	OriginalContent string    `json:"original_content,omitempty"`
	OriginalState   []byte    `json:"original_state,omitempty"`
}

// NewDeleteMessageCommand creates a new delete message command
func NewDeleteMessageCommand(msgID, userID uuid.UUID, deleteForAll bool) *DeleteMessageCommand {
	return &DeleteMessageCommand{
		BaseCommand: BaseCommand{
			ID:        uuid.New(),
			Type:      "DeleteMessage",
			UserID:    userID,
			CreatedAt: time.Now(),
		},
		MessageID:    msgID,
		DeleteForAll: deleteForAll,
	}
}

// Validate validates the command
func (c *DeleteMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil {
		return errors.New("message_id is required")
	}
	return nil
}

// Execute executes the command
func (c *DeleteMessageCommand) Execute(ctx context.Context) error {
	return nil
}

// Undo undoes the deletion (restores the message)
func (c *DeleteMessageCommand) Undo(ctx context.Context) error {
	return nil
}

// ToJSON serializes to JSON
func (c *DeleteMessageCommand) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CanUndo checks if can undo
func (c *DeleteMessageCommand) CanUndo() bool {
	return time.Now().Before(c.GetUndoDeadline()) && c.OriginalState != nil
}
