package commands

import (
	"errors"
	"strings"

	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// SendMessageCommand sends a new message
type SendMessageCommand struct {
	ConversationID      uuid.UUID
	SenderID            uuid.UUID
	Content             string
	MessageType         string // TEXT, IMAGE, VIDEO, AUDIO, FILE, LOCATION, CONTACT, STICKER, GIF, POLL
	ReplyToMessageID    uuid.UUID
	ForwardedFromMsgID  uuid.UUID
	AttachmentIDs       []uuid.UUID
	Metadata            map[string]interface{}
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
	if strings.TrimSpace(c.Content) == "" && len(c.AttachmentIDs) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendMessageCommand) IdempotencyKey() string {
	return c.IdempotencyKeyValue
}

func (c SendMessageCommand) ActorID() uuid.UUID {
	return c.SenderID
}

// EditMessageCommand edits an existing message
type EditMessageCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	ConversationID      uuid.UUID
	NewContent          string
	IdempotencyKeyValue string
}

func (EditMessageCommand) CommandType() string { return "message.edit" }

func (c EditMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if strings.TrimSpace(c.NewContent) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c EditMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c EditMessageCommand) ActorID() uuid.UUID { return c.UserID }

// DeleteMessageCommand deletes a message
type DeleteMessageCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	ConversationID      uuid.UUID
	DeleteForEveryone   bool
	IdempotencyKeyValue string
}

func (DeleteMessageCommand) CommandType() string { return "message.delete" }

func (c DeleteMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c DeleteMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c DeleteMessageCommand) ActorID() uuid.UUID { return c.UserID }

// ReactToMessageCommand adds a reaction to a message
type ReactToMessageCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	ConversationID      uuid.UUID
	ReactionCode        string
	IdempotencyKeyValue string
}

func (ReactToMessageCommand) CommandType() string { return "message.react" }

func (c ReactToMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil || c.ReactionCode == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ReactToMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ReactToMessageCommand) ActorID() uuid.UUID { return c.UserID }

// RemoveReactionCommand removes a reaction from a message
type RemoveReactionCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	ConversationID      uuid.UUID
	ReactionCode        string
	IdempotencyKeyValue string
}

func (RemoveReactionCommand) CommandType() string { return "message.remove_reaction" }

func (c RemoveReactionCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil || c.ReactionCode == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RemoveReactionCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RemoveReactionCommand) ActorID() uuid.UUID { return c.UserID }

// StarMessageCommand stars a message
type StarMessageCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (StarMessageCommand) CommandType() string { return "message.star" }

func (c StarMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c StarMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c StarMessageCommand) ActorID() uuid.UUID { return c.UserID }

// UnstarMessageCommand unstars a message
type UnstarMessageCommand struct {
	MessageID           uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (UnstarMessageCommand) CommandType() string { return "message.unstar" }

func (c UnstarMessageCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UnstarMessageCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UnstarMessageCommand) ActorID() uuid.UUID { return c.UserID }

// MarkMessageReadCommand marks a message as read
type MarkMessageReadCommand struct {
	MessageID      uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
}

func (MarkMessageReadCommand) CommandType() string { return "message.read" }

func (c MarkMessageReadCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c MarkMessageReadCommand) IdempotencyKey() string { return "" }

func (c MarkMessageReadCommand) ActorID() uuid.UUID { return c.UserID }

// MarkMessageDeliveredCommand marks a message as delivered
type MarkMessageDeliveredCommand struct {
	MessageID      uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
}

func (MarkMessageDeliveredCommand) CommandType() string { return "message.delivered" }

func (c MarkMessageDeliveredCommand) Validate() error {
	if c.MessageID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c MarkMessageDeliveredCommand) IdempotencyKey() string { return "" }

func (c MarkMessageDeliveredCommand) ActorID() uuid.UUID { return c.UserID }

// TypingCommand indicates typing status
type TypingCommand struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	IsTyping       bool
}

func (TypingCommand) CommandType() string { return "message.typing" }

func (c TypingCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c TypingCommand) IdempotencyKey() string { return "" }

func (c TypingCommand) ActorID() uuid.UUID { return c.UserID }

// CreatePollCommand creates a poll
type CreatePollCommand struct {
	ConversationID      uuid.UUID
	SenderID            uuid.UUID
	Question            string
	Options             []string
	AllowsMultiple      bool
	IdempotencyKeyValue string
}

func (CreatePollCommand) CommandType() string { return "message.create_poll" }

func (c CreatePollCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.SenderID == uuid.Nil || c.Question == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if len(c.Options) < 2 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreatePollCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreatePollCommand) ActorID() uuid.UUID { return c.SenderID }

// VotePollCommand votes on a poll
type VotePollCommand struct {
	PollID              uuid.UUID
	UserID              uuid.UUID
	OptionIDs           []uuid.UUID
	IdempotencyKeyValue string
}

func (VotePollCommand) CommandType() string { return "message.vote_poll" }

func (c VotePollCommand) Validate() error {
	if c.PollID == uuid.Nil || c.UserID == uuid.Nil || len(c.OptionIDs) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c VotePollCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c VotePollCommand) ActorID() uuid.UUID { return c.UserID }

// ClosePollCommand closes a poll
type ClosePollCommand struct {
	PollID              uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (ClosePollCommand) CommandType() string { return "message.close_poll" }

func (c ClosePollCommand) Validate() error {
	if c.PollID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ClosePollCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ClosePollCommand) ActorID() uuid.UUID { return c.UserID }

var ErrDuplicateCommand = errors.New("duplicate command")
var ErrHandlerNotFound = errors.New("handler not found")
