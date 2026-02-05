package commands

import (
	"errors"
	"strings"

	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type SendMessageCommand struct {
	ConversationID      uuid.UUID
	SenderID            uuid.UUID
	Content             string
	IdempotencyKeyValue string
	ClientMsgID         string
}

func (SendMessageCommand) CommandType() string {
	return "message.send"
}

func (c SendMessageCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.SenderID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if strings.TrimSpace(c.Content) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendMessageCommand) IdempotencyKey() string {
	return c.IdempotencyKeyValue
}

var ErrDuplicateCommand = errors.New("duplicate command")
