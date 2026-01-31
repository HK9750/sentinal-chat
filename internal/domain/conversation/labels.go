package conversation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ChatLabel represents chat_labels
type ChatLabel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	Name      string    `gorm:"not null"`
	Color     sql.NullString
	Position  sql.NullInt32
	CreatedAt time.Time `gorm:"default:now()"`
}

// ConversationLabel represents conversation_labels
type ConversationLabel struct {
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LabelID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt      time.Time `gorm:"default:now()"`
}

// ConversationClear represents conversation_clears
type ConversationClear struct {
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ClearedAt      time.Time `gorm:"default:now()"`
}

func (ChatLabel) TableName() string {
	return "chat_labels"
}

func (ConversationLabel) TableName() string {
	return "conversation_labels"
}

func (ConversationClear) TableName() string {
	return "conversation_clears"
}
