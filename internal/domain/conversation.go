package domain

import (
	"time"
)

type Conversation struct {
	ID            string           `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Type          ConversationType `gorm:"type:conversation_type;not null" json:"type"`
	Subject       *string          `gorm:"type:text" json:"subject,omitempty"`
	Description   *string          `gorm:"type:text" json:"description,omitempty"`
	AvatarURL     *string          `gorm:"type:text" json:"avatar_url,omitempty"`
	ExpirySeconds *int             `json:"expiry_seconds,omitempty"`
	CreatedBy     *string          `gorm:"type:uuid" json:"created_by,omitempty"`
	CreatedAt     time.Time        `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time        `gorm:"default:CURRENT_TIMESTAMP;index:idx_conversations_updated,sort:desc" json:"updated_at"`

	// Relations
	Participants []Participant `gorm:"foreignKey:ConversationID" json:"participants,omitempty"`
}

type Participant struct {
	ConversationID string          `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	UserID         string          `gorm:"type:uuid;primaryKey;index:idx_participants_user" json:"user_id"`
	Role           ParticipantRole `gorm:"type:participant_role;default:'MEMBER';not null" json:"role"`
	JoinedAt       time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"joined_at"`
	AddedBy        *string         `gorm:"type:uuid" json:"added_by,omitempty"`

	// Chat Specific Settings
	MutedUntil       *time.Time `json:"muted_until,omitempty"`
	PinnedAt         *time.Time `json:"pinned_at,omitempty"`
	Archived         bool       `gorm:"default:false" json:"archived"`
	LastReadSequence int64      `gorm:"default:0" json:"last_read_sequence"`

	// Relations
	User         User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

type ConversationSequence struct {
	ConversationID string    `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	LastSequence   int64     `gorm:"not null;default:0" json:"last_sequence"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}
