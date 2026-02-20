package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Poll represents polls
type Poll struct {
	ID             uuid.UUID
	MessageID      uuid.NullUUID
	Question       string
	AllowsMultiple bool
	ClosesAt       sql.NullTime
	CreatedAt      time.Time
}

// PollOption represents poll_options
type PollOption struct {
	ID         uuid.UUID
	PollID     uuid.UUID
	OptionText string
	Position   int
}

// PollVote represents poll_votes
type PollVote struct {
	PollID   uuid.UUID
	OptionID uuid.UUID
	UserID   uuid.UUID
	VotedAt  time.Time
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
