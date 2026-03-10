package conversation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Conversation represents the conversations table
type Conversation struct {
	ID                  uuid.UUID
	Type                string
	Subject             sql.NullString
	Description         sql.NullString
	AvatarURL           sql.NullString
	InviteLink          sql.NullString
	InviteLinkRevokedAt sql.NullTime
	DMUserIDA           uuid.NullUUID
	DMUserIDB           uuid.NullUUID
	DisappearingMode    string
	CreatedBy           uuid.NullUUID
	CreatedAt           time.Time
	UpdatedAt           time.Time

	// Computed fields (populated via subquery, not stored in conversations table)
	LastMessageAt *time.Time

	// Relationships
	Participants []Participant
}

// Participant represents the participants table
type Participant struct {
	ConversationID   uuid.UUID
	UserID           uuid.UUID
	Role             string
	JoinedAt         time.Time
	AddedBy          uuid.NullUUID
	MutedUntil       sql.NullTime
	Archived         bool
	LastReadSequence int64

	// Joined user fields (populated via JOIN, not stored in participants table)
	DisplayName string
	Username    string
	AvatarURL   string
	IsOnline    bool
}

// ConversationSequence represents the conversation_sequences table
type ConversationSequence struct {
	ConversationID uuid.UUID
	LastSequence   int64
	UpdatedAt      time.Time
}

// ConversationClear represents the conversation_clears table
type ConversationClear struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	ClearedAt      time.Time
}

func (Conversation) TableName() string {
	return "conversations"
}

func (Participant) TableName() string {
	return "participants"
}

func (ConversationSequence) TableName() string {
	return "conversation_sequences"
}

func (ConversationClear) TableName() string {
	return "conversation_clears"
}
