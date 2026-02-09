package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Message represents the messages table
type Message struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConversationID     uuid.UUID `gorm:"type:uuid;not null"`
	SenderID           uuid.UUID `gorm:"type:uuid;not null"`
	ClientMessageID    sql.NullString
	IdempotencyKey     sql.NullString
	SeqID              sql.NullInt64  // Managed by trigger, but model needs it
	Type               string         `gorm:"type:message_type;default:'TEXT'"`
	Content            sql.NullString `gorm:"-"`
	Metadata           string         `gorm:"type:jsonb"`
	IsForwarded        bool           `gorm:"default:false"`
	ForwardedFromMsgID uuid.NullUUID  `gorm:"type:uuid"`
	ReplyToMsgID       uuid.NullUUID  `gorm:"type:uuid"`
	PollID             uuid.NullUUID  `gorm:"type:uuid"`
	LinkPreviewID      uuid.NullUUID  `gorm:"type:uuid"`
	MentionCount       int            `gorm:"default:0"`
	CreatedAt          time.Time      `gorm:"default:now()"`
	EditedAt           sql.NullTime
	DeletedAt          sql.NullTime
	ExpiresAt          sql.NullTime
	Ciphertext         []byte        `gorm:"column:ciphertext;->"`
	Header             string        `gorm:"column:header;->"`
	RecipientDeviceID  uuid.NullUUID `gorm:"column:recipient_device_id;->"`
	RecipientUserID    uuid.NullUUID `gorm:"column:recipient_user_id;->"`
	SenderDeviceID     uuid.NullUUID `gorm:"column:sender_device_id;->"`
}

// MessageReaction represents message_reactions
type MessageReaction struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	MessageID    uuid.UUID `gorm:"type:uuid;not null"`
	UserID       uuid.UUID `gorm:"type:uuid;not null"`
	ReactionCode string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"default:now()"`
}

// MessageReceipt represents message_receipts
type MessageReceipt struct {
	MessageID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	Status      string    `gorm:"type:delivery_status;default:'PENDING'"`
	DeliveredAt sql.NullTime
	ReadAt      sql.NullTime
	PlayedAt    sql.NullTime
	UpdatedAt   time.Time `gorm:"default:now()"`
}

// MessageMention represents message_mentions
type MessageMention struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Offset    int       `gorm:"column:offset;primaryKey"` // 'offset' is a reserved word in PostgreSQL
	Length    int       `gorm:"not null"`
}

// StarredMessage represents starred_messages
type StarredMessage struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	StarredAt time.Time `gorm:"default:now()"`
}

// MessageCiphertext represents message_ciphertexts
type MessageCiphertext struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	MessageID         uuid.UUID     `gorm:"type:uuid;not null"`
	RecipientUserID   uuid.UUID     `gorm:"type:uuid;not null"`
	RecipientDeviceID uuid.UUID     `gorm:"type:uuid;not null"`
	SenderDeviceID    uuid.NullUUID `gorm:"type:uuid"`
	Ciphertext        []byte        `gorm:"not null"`
	Header            string        `gorm:"type:jsonb"`
	CreatedAt         time.Time     `gorm:"default:now()"`
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
