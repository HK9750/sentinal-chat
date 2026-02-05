package commands

import (
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// CreateBroadcastListCommand creates a new broadcast list
type CreateBroadcastListCommand struct {
	OwnerID             uuid.UUID
	Name                string
	Description         string
	RecipientIDs        []uuid.UUID
	IdempotencyKeyValue string
}

func (CreateBroadcastListCommand) CommandType() string { return "broadcast.create" }

func (c CreateBroadcastListCommand) Validate() error {
	if c.OwnerID == uuid.Nil || c.Name == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateBroadcastListCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreateBroadcastListCommand) ActorID() uuid.UUID { return c.OwnerID }

// UpdateBroadcastListCommand updates a broadcast list
type UpdateBroadcastListCommand struct {
	BroadcastID         uuid.UUID
	OwnerID             uuid.UUID
	Name                string
	Description         string
	IdempotencyKeyValue string
}

func (UpdateBroadcastListCommand) CommandType() string { return "broadcast.update" }

func (c UpdateBroadcastListCommand) Validate() error {
	if c.BroadcastID == uuid.Nil || c.OwnerID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateBroadcastListCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateBroadcastListCommand) ActorID() uuid.UUID { return c.OwnerID }

// DeleteBroadcastListCommand deletes a broadcast list
type DeleteBroadcastListCommand struct {
	BroadcastID         uuid.UUID
	OwnerID             uuid.UUID
	IdempotencyKeyValue string
}

func (DeleteBroadcastListCommand) CommandType() string { return "broadcast.delete" }

func (c DeleteBroadcastListCommand) Validate() error {
	if c.BroadcastID == uuid.Nil || c.OwnerID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c DeleteBroadcastListCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c DeleteBroadcastListCommand) ActorID() uuid.UUID { return c.OwnerID }

// AddBroadcastRecipientCommand adds a recipient to broadcast list
type AddBroadcastRecipientCommand struct {
	BroadcastID         uuid.UUID
	OwnerID             uuid.UUID
	RecipientID         uuid.UUID
	IdempotencyKeyValue string
}

func (AddBroadcastRecipientCommand) CommandType() string { return "broadcast.add_recipient" }

func (c AddBroadcastRecipientCommand) Validate() error {
	if c.BroadcastID == uuid.Nil || c.OwnerID == uuid.Nil || c.RecipientID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c AddBroadcastRecipientCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c AddBroadcastRecipientCommand) ActorID() uuid.UUID { return c.OwnerID }

// RemoveBroadcastRecipientCommand removes a recipient from broadcast list
type RemoveBroadcastRecipientCommand struct {
	BroadcastID         uuid.UUID
	OwnerID             uuid.UUID
	RecipientID         uuid.UUID
	IdempotencyKeyValue string
}

func (RemoveBroadcastRecipientCommand) CommandType() string { return "broadcast.remove_recipient" }

func (c RemoveBroadcastRecipientCommand) Validate() error {
	if c.BroadcastID == uuid.Nil || c.OwnerID == uuid.Nil || c.RecipientID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RemoveBroadcastRecipientCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RemoveBroadcastRecipientCommand) ActorID() uuid.UUID { return c.OwnerID }

// SendBroadcastMessageCommand sends a message to broadcast list
type SendBroadcastMessageCommand struct {
	BroadcastID         uuid.UUID
	SenderID            uuid.UUID
	Content             string
	MessageType         string
	IdempotencyKeyValue string
}

func (SendBroadcastMessageCommand) CommandType() string { return "broadcast.send_message" }

func (c SendBroadcastMessageCommand) Validate() error {
	if c.BroadcastID == uuid.Nil || c.SenderID == uuid.Nil || c.Content == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendBroadcastMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c SendBroadcastMessageCommand) ActorID() uuid.UUID { return c.SenderID }
