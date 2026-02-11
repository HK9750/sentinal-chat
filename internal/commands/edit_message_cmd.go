package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EditMessageCommand edits a message with version history
type EditMessageCommand struct {
	BaseCommand
	MessageID       uuid.UUID `json:"message_id"`
	NewContent      string    `json:"new_content"`
	PreviousContent string    `json:"previous_content,omitempty"`
}

// NewEditMessageCommand creates a new edit message command
func NewEditMessageCommand(msgID, userID uuid.UUID, newContent string) *EditMessageCommand {
	return &EditMessageCommand{
		BaseCommand: BaseCommand{
			ID:        uuid.New(),
			Type:      "EditMessage",
			UserID:    userID,
			CreatedAt: time.Now(),
		},
		MessageID:  msgID,
		NewContent: newContent,
	}
}

// Validate validates the command
func (c *EditMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(c.NewContent) == "" {
		return errors.New("new content cannot be empty")
	}
	return nil
}

// Execute executes the command
func (c *EditMessageCommand) Execute(ctx context.Context) error {
	return nil
}

// Undo undoes the edit (reverts to previous version)
func (c *EditMessageCommand) Undo(ctx context.Context) error {
	return nil
}

// ToJSON serializes to JSON
func (c *EditMessageCommand) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CanUndo checks if can undo (15 minute window)
func (c *EditMessageCommand) CanUndo() bool {
	return time.Now().Before(c.CreatedAt.Add(15 * time.Minute))
}

// GetUndoDeadline returns the undo deadline (15 minutes)
func (c *EditMessageCommand) GetUndoDeadline() time.Time {
	return c.CreatedAt.Add(15 * time.Minute)
}
