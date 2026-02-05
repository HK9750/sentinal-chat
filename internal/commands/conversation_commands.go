package commands

import (
	"strings"

	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type CreateConversationCommand struct {
	Type           string
	Subject        string
	Description    string
	CreatorID      uuid.UUID
	ParticipantIDs []uuid.UUID
}

func (CreateConversationCommand) CommandType() string {
	return "conversation.create"
}

func (c CreateConversationCommand) Validate() error {
	if c.CreatorID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.Type != "DM" && c.Type != "GROUP" {
		return sentinal_errors.ErrInvalidInput
	}
	if c.Type == "GROUP" && strings.TrimSpace(c.Subject) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if len(c.ParticipantIDs) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateConversationCommand) IdempotencyKey() string {
	return ""
}
