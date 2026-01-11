package domain

import (
	"time"
)

type Message struct {
	ID             string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ConversationID string `gorm:"type:uuid;not null;index:idx_messages_history,priority:1" json:"conversation_id"`
	SenderID       string `gorm:"type:uuid;not null" json:"sender_id"`
	// SeqID is managed by the database trigger 'fn_assign_message_sequence'
	SeqID              int64                  `gorm:"<-:false;not null;index:idx_messages_history,priority:2,sort:desc" json:"seq_id"`
	Type               MessageType            `gorm:"type:message_type;default:'TEXT';not null" json:"type"`
	Content            *string                `gorm:"type:text;index:idx_messages_content_gin,type:gin" json:"content,omitempty"`
	Metadata           map[string]interface{} `gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"`
	IsForwarded        bool                   `gorm:"default:false" json:"is_forwarded"`
	ForwardedFromMsgID *string                `gorm:"type:uuid" json:"forwarded_from_msg_id,omitempty"`
	ReplyToMsgID       *string                `gorm:"type:uuid" json:"reply_to_msg_id,omitempty"`
	CreatedAt          time.Time              `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	EditedAt           *time.Time             `json:"edited_at,omitempty"`
	DeletedAt          *time.Time             `json:"deleted_at,omitempty"`

	// Relations
	Reactions   []MessageReaction `gorm:"foreignKey:MessageID" json:"reactions,omitempty"`
	Attachments []Attachment      `gorm:"many2many:message_attachments;" json:"attachments,omitempty"`
	Receipts    []MessageReceipt  `gorm:"foreignKey:MessageID" json:"receipts,omitempty"`
}

type MessageReaction struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	MessageID    string    `gorm:"type:uuid;not null;index:idx_reactions_message" json:"message_id"`
	UserID       string    `gorm:"type:uuid;not null" json:"user_id"`
	ReactionCode string    `gorm:"type:varchar(64);not null" json:"reaction_code"`
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

type MessageReceipt struct {
	MessageID   string         `gorm:"type:uuid;primaryKey;index:idx_receipts_message" json:"message_id"`
	UserID      string         `gorm:"type:uuid;primaryKey" json:"user_id"`
	Status      DeliveryStatus `gorm:"type:delivery_status;default:'PENDING';not null" json:"status"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	ReadAt      *time.Time     `json:"read_at,omitempty"`
	UpdatedAt   time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}
