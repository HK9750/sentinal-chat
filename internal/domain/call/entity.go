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
	Topology        string
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
	DeviceType string
}

func (Call) TableName() string {
	return "calls"
}

func (CallParticipant) TableName() string {
	return "call_participants"
}

// CallQualityMetric represents call_quality_metrics
type CallQualityMetric struct {
	ID               uuid.UUID
	CallID           uuid.UUID
	UserID           uuid.UUID
	RecordedAt       time.Time
	PacketsSent      int64
	PacketsReceived  int64
	PacketsLost      int64
	JitterMs         float64
	RoundTripTimeMs  float64
	BitrateKbps      int
	FrameRate        int
	ResolutionWidth  int
	ResolutionHeight int
	AudioLevel       float64
	ConnectionType   string
	IceCandidateType string
}

func (CallQualityMetric) TableName() string {
	return "call_quality_metrics"
}
