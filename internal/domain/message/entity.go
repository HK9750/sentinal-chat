package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Message represents the messages table
type Message struct {
	ID                 uuid.UUID
	ConversationID     uuid.UUID
	SenderID           uuid.UUID
	ClientMessageID    sql.NullString
	IdempotencyKey     sql.NullString
	SeqID              sql.NullInt64 // Managed by trigger, but model needs it
	Type               string
	Content            sql.NullString
	Metadata           string
	IsForwarded        bool
	ForwardedFromMsgID uuid.NullUUID
	ReplyToMsgID       uuid.NullUUID
	PollID             uuid.NullUUID
	LinkPreviewID      uuid.NullUUID
	MentionCount       int
	CreatedAt          time.Time
	EditedAt           sql.NullTime
	DeletedAt          sql.NullTime
	ExpiresAt          sql.NullTime
	Ciphertext         []byte
	Header             string
	RecipientDeviceID  uuid.NullUUID
	RecipientUserID    uuid.NullUUID
	SenderDeviceID     uuid.NullUUID
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

// MessageCiphertext represents message_ciphertexts
type MessageCiphertext struct {
	ID                uuid.UUID
	MessageID         uuid.UUID
	RecipientUserID   uuid.UUID
	RecipientDeviceID uuid.UUID
	SenderDeviceID    uuid.NullUUID
	Ciphertext        []byte
	Header            string
	CreatedAt         time.Time
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

func (MessageCiphertext) TableName() string {
	return "message_ciphertexts"
}
