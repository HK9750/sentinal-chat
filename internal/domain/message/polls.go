package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Poll represents polls
type Poll struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	MessageID      uuid.NullUUID `gorm:"type:uuid"`
	Question       string        `gorm:"not null"`
	AllowsMultiple bool          `gorm:"default:false"`
	ClosesAt       sql.NullTime
	CreatedAt      time.Time `gorm:"default:now()"`
}

// PollOption represents poll_options
type PollOption struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	PollID     uuid.UUID `gorm:"type:uuid;not null"`
	OptionText string    `gorm:"not null"`
	Position   int       `gorm:"not null"`
}

// PollVote represents poll_votes
type PollVote struct {
	PollID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	OptionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	VotedAt  time.Time `gorm:"default:now()"`
}

func (Poll) TableName() string {
	return "polls"
}

func (PollOption) TableName() string {
	return "poll_options"
}

func (PollVote) TableName() string {
	return "poll_votes"
}
