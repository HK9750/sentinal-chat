package commands

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// BulkArchiveResult represents the result for one conversation
type BulkArchiveResult struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Success        bool      `json:"success"`
	Error          string    `json:"error,omitempty"`
}

// BulkArchiveCommand archives multiple conversations
type BulkArchiveCommand struct {
	BaseCommand
	ConversationIDs []uuid.UUID         `json:"conversation_ids"`
	Results         []BulkArchiveResult `json:"results,omitempty"`
}

// NewBulkArchiveCommand creates a new bulk archive command
func NewBulkArchiveCommand(userID uuid.UUID, convIDs []uuid.UUID) *BulkArchiveCommand {
	return &BulkArchiveCommand{
		BaseCommand: BaseCommand{
			ID:        uuid.New(),
			Type:      "BulkArchive",
			UserID:    userID,
			CreatedAt: time.Now(),
		},
		ConversationIDs: convIDs,
	}
}

// Validate validates the command
func (c *BulkArchiveCommand) Validate() error {
	if len(c.ConversationIDs) == 0 {
		return errors.New("at least one conversation_id is required")
	}
	if len(c.ConversationIDs) > 100 {
		return errors.New("cannot archive more than 100 conversations at once")
	}
	return nil
}

// Execute executes the command
func (c *BulkArchiveCommand) Execute(ctx context.Context) error {
	return nil
}

// Undo undoes the archive (unarchives all)
func (c *BulkArchiveCommand) Undo(ctx context.Context) error {
	return nil
}

// ToJSON serializes to JSON
func (c *BulkArchiveCommand) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// CanUndo checks if can undo (30 minute window)
func (c *BulkArchiveCommand) CanUndo() bool {
	return time.Now().Before(c.CreatedAt.Add(30 * time.Minute))
}

// GetUndoDeadline returns the undo deadline (30 minutes)
func (c *BulkArchiveCommand) GetUndoDeadline() time.Time {
	return c.CreatedAt.Add(30 * time.Minute)
}
