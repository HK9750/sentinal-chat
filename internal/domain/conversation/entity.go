package conversation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	// Import user package for FK references if needed, or just use uuid.UUID
)

// Conversation represents the conversations table
type Conversation struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Type                 string    `gorm:"type:conversation_type;not null"`
	Subject              sql.NullString
	Description          sql.NullString
	AvatarURL            sql.NullString
	ExpirySeconds        sql.NullInt32
	DisappearingMode     string `gorm:"type:disappearing_mode;default:'OFF'"`
	MessageExpirySeconds sql.NullInt32
	GroupPermissions     string `gorm:"type:jsonb"` // Using string for JSONB for simplicity, or define a specific struct/map
	InviteLink           sql.NullString
	InviteLinkRevokedAt  sql.NullTime
	CreatedBy            uuid.NullUUID `gorm:"type:uuid"`
	CreatedAt            time.Time     `gorm:"default:now()"`
	UpdatedAt            time.Time     `gorm:"default:now()"`

	// Relationships
	Participants []Participant `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
	// Sequence     ConversationSequence `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"` // Optional: 1-to-1
}

// Participant represents the participants table
type Participant struct {
	ConversationID   uuid.UUID     `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Role             string        `gorm:"type:participant_role;default:'MEMBER'"`
	JoinedAt         time.Time     `gorm:"default:now()"`
	AddedBy          uuid.NullUUID `gorm:"type:uuid"`
	MutedUntil       sql.NullTime
	PinnedAt         sql.NullTime
	Archived         bool   `gorm:"default:false"`
	LastReadSequence int64  `gorm:"default:0"`
	Permissions      string `gorm:"type:jsonb"`

	// Relationships
	// User user.User `gorm:"foreignKey:UserID"`
}

// ConversationSequence represents the conversation_sequences table
type ConversationSequence struct {
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LastSequence   int64     `gorm:"default:0"`
	UpdatedAt      time.Time `gorm:"default:now()"`
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
