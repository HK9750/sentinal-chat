package conversation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	// Import user package for FK references if needed, or just use uuid.UUID
)

// Conversation represents the conversations table
type Conversation struct {
	ID                   uuid.UUID
	Type                 string
	Subject              sql.NullString
	Description          sql.NullString
	AvatarURL            sql.NullString
	ExpirySeconds        sql.NullInt32
	DisappearingMode     string
	MessageExpirySeconds sql.NullInt32
	GroupPermissions     *string
	InviteLink           sql.NullString
	InviteLinkRevokedAt  sql.NullTime
	CreatedBy            uuid.NullUUID
	CreatedAt            time.Time
	UpdatedAt            time.Time

	// Relationships
	Participants []Participant
	// Sequence     ConversationSequence
}

// Participant represents the participants table
type Participant struct {
	ConversationID   uuid.UUID
	UserID           uuid.UUID
	Role             string
	JoinedAt         time.Time
	AddedBy          uuid.NullUUID
	MutedUntil       sql.NullTime
	PinnedAt         sql.NullTime
	Archived         bool
	LastReadSequence int64
	Permissions      *string

	// Relationships
	// User user.User
}

// ConversationSequence represents the conversation_sequences table
type ConversationSequence struct {
	ConversationID uuid.UUID
	LastSequence   int64
	UpdatedAt      time.Time
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
