package conversation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ChatLabel represents chat_labels
type ChatLabel struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Color     sql.NullString
	Position  sql.NullInt32
	CreatedAt time.Time
}

// ConversationLabel represents conversation_labels
type ConversationLabel struct {
	ConversationID uuid.UUID
	LabelID        uuid.UUID
	UserID         uuid.UUID
	CreatedAt      time.Time
}

// ConversationClear represents conversation_clears
type ConversationClear struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	ClearedAt      time.Time
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
