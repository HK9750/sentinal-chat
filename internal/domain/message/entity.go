package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Message represents the messages table
type Message struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConversationID     uuid.UUID `gorm:"type:uuid;not null"`
	SenderID           uuid.UUID `gorm:"type:uuid;not null"`
	ClientMessageID    sql.NullString
	IdempotencyKey     sql.NullString
	SeqID              sql.NullInt64 // Managed by trigger, but model needs it
	Type               string        `gorm:"type:message_type;default:'TEXT'"`
	Content            sql.NullString
	Metadata           string        `gorm:"type:jsonb"`
	IsForwarded        bool          `gorm:"default:false"`
	ForwardedFromMsgID uuid.NullUUID `gorm:"type:uuid"`
	ReplyToMsgID       uuid.NullUUID `gorm:"type:uuid"`
	PollID             uuid.NullUUID `gorm:"type:uuid"`
	LinkPreviewID      uuid.NullUUID `gorm:"type:uuid"`
	MentionCount       int           `gorm:"default:0"`
	CreatedAt          time.Time     `gorm:"default:now()"`
	EditedAt           sql.NullTime
	DeletedAt          gorm.DeletedAt `gorm:"index"` // Using gorm.DeletedAt for soft delete if desired, OR manual sql.NullTime matching DB spec. DB spec says "deleted_at TIMESTAMP", so sql.NullTime is safer if we don't want GORM magic.
	// Actually spec says "deleted_at TIMESTAMP", let's use sql.NullTime to be strict with schema.
	// But wait, GORM `DeletedAt` is standard. Let's stick to sql.NullTime to match `database.md` exactly which might not imply GORM's specific soft delete behavior (though it likely does).
	// Let's use sql.NullTime for explicit control as per spec.
	ExpiresAt sql.NullTime
}

// Special handling for DeletedAt if using sql.NullTime to avoid GORM auto-hook if not desired,
// OR use gorm.DeletedAt if we WANT GORM soft deletes.
// Impl plan says "GORM models", usually implies using GORM features. But for "deleted_at" column, the spec is clear.
// I'll use `sql.NullTime` for `DeletedAt` field name, GORM might auto-detect it as soft delete if named `DeletedAt`.
// To be safe and just map to DB, `DeletedAt` field is fine.

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
	Offset    int       `gorm:"primaryKey"` // Spec says PK is (message_id, user_id, offset)
	Length    int       `gorm:"not null"`
}

// StarredMessage represents starred_messages
type StarredMessage struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	StarredAt time.Time `gorm:"default:now()"`
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
