package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Message represents the messages table
type Message struct {
	ID              uuid.UUID
	ConversationID  uuid.UUID
	SenderID        uuid.UUID
	ClientMessageID sql.NullString
	SeqID           sql.NullInt64
	Type            string
	Content         sql.NullString
	IsForwarded     bool
	ReplyToMsgID    uuid.NullUUID
	PollID          uuid.NullUUID
	MentionCount    int
	CreatedAt       time.Time
	EditedAt        sql.NullTime
	DeletedAt       sql.NullTime
	ExpiresAt       sql.NullTime
}

// MessageReaction represents message_reactions
type MessageReaction struct {
	ID           uuid.UUID
	MessageID    uuid.UUID
	UserID       uuid.UUID
	ReactionCode string
	CreatedAt    time.Time
}

// MessageReceipt represents message_receipts
type MessageReceipt struct {
	MessageID   uuid.UUID
	UserID      uuid.UUID
	Status      string
	DeliveredAt sql.NullTime
	ReadAt      sql.NullTime
	PlayedAt    sql.NullTime
	UpdatedAt   time.Time
}

// MessageMention represents message_mentions
type MessageMention struct {
	MessageID uuid.UUID
	UserID    uuid.UUID
	Offset    int
	Length    int
}

// StarredMessage represents starred_messages
type StarredMessage struct {
	UserID    uuid.UUID
	MessageID uuid.UUID
	StarredAt time.Time
}

// PinnedMessage represents pinned_messages
type PinnedMessage struct {
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	PinnedBy       uuid.UUID
	PinnedAt       time.Time
}

// MessageEdit represents message_edits
type MessageEdit struct {
	ID            uuid.UUID
	MessageID     uuid.UUID
	Content       string
	EditedBy      uuid.UUID
	EditedAt      time.Time
	VersionNumber int
}

func (Message) TableName() string {
	return "messages"
}

func (MessageReaction) TableName() string {
	return "message_reactions"
}

func (MessageReceipt) TableName() string {
	return "message_receipts"
}

func (MessageMention) TableName() string {
	return "message_mentions"
}

func (StarredMessage) TableName() string {
	return "starred_messages"
}

func (PinnedMessage) TableName() string {
	return "pinned_messages"
}

func (MessageEdit) TableName() string {
	return "message_edits"
}
