package call

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Call represents calls table
type Call struct {
	ID              uuid.UUID
	ConversationID  uuid.UUID
	InitiatedBy     uuid.UUID
	Type            string
	IsGroupCall     bool
	StartedAt       time.Time
	ConnectedAt     sql.NullTime
	EndedAt         sql.NullTime
	EndReason       sql.NullString
	DurationSeconds sql.NullInt32
	CreatedAt       time.Time
}

// CallParticipant represents call_participants
type CallParticipant struct {
	CallID     uuid.UUID
	UserID     uuid.UUID
	Status     string
	JoinedAt   sql.NullTime
	LeftAt     sql.NullTime
	MutedAudio bool
	MutedVideo bool
}

func (Call) TableName() string {
	return "calls"
}

func (CallParticipant) TableName() string {
	return "call_participants"
}
