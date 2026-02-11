package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SendMessageCommand sends a message with optional scheduling
type SendMessageCommand struct {
	BaseCommand
	ConversationID uuid.UUID  `json:"conversation_id"`
	SenderID       uuid.UUID  `json:"sender_id"`
	Content        string     `json:"content"`
	ScheduledFor   *time.Time `json:"scheduled_for,omitempty"`
	MaxRetries     int        `json:"max_retries"`
}

// NewSendMessageCommand creates a new send message command
func NewSendMessageCommand(convID, senderID uuid.UUID, content string) *SendMessageCommand {
	return &SendMessageCommand{
		BaseCommand: BaseCommand{
			ID:        uuid.New(),
			Type:      "SendMessage",
			UserID:    senderID,
			CreatedAt: time.Now(),
		},
		ConversationID: convID,
		SenderID:       senderID,
		Content:        content,
		MaxRetries:     3,
	}
}

// Validate validates the command
func (c *SendMessageCommand) Validate() error {
	if c.ConversationID == uuid.Nil {
		return errors.New("conversation_id is required")
	}
	if c.SenderID == uuid.Nil {
		return errors.New("sender_id is required")
	}
	if strings.TrimSpace(c.Content) == "" {
		return errors.New("content cannot be empty")
	}
	if c.ScheduledFor != nil && c.ScheduledFor.Before(time.Now()) {
		return errors.New("scheduled time must be in the future")
	}
	return nil
}

// Execute executes the command
func (c *SendMessageCommand) Execute(ctx context.Context) error {
	// Implementation will be in CommandExecutor
	return nil
}

// Undo undoes the command (deletes the sent message)
func (c *SendMessageCommand) Undo(ctx context.Context) error {
	// Soft delete the message
	return nil
}

// ToJSON serializes to JSON
func (c *SendMessageCommand) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CanUndo checks if can undo
func (c *SendMessageCommand) CanUndo() bool {
	return time.Now().Before(c.GetUndoDeadline())
}
