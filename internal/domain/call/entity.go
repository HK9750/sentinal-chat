package call

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Call represents calls table
type Call struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ConversationID  uuid.UUID `gorm:"type:uuid;not null"`
	InitiatedBy     uuid.UUID `gorm:"type:uuid;not null"`
	Type            string    `gorm:"type:call_type;not null"`
	Topology        string    `gorm:"type:call_topology;not null"`
	IsGroupCall     bool      `gorm:"default:false"`
	StartedAt       time.Time `gorm:"default:now()"`
	ConnectedAt     sql.NullTime
	EndedAt         sql.NullTime
	EndReason       sql.NullString `gorm:"type:call_end_reason"` // Using NullString for nullable enum
	DurationSeconds sql.NullInt32
	CreatedAt       time.Time `gorm:"default:now()"`
}

// CallParticipant represents call_participants
type CallParticipant struct {
	CallID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Status     string    `gorm:"type:participant_call_status;default:'INVITED'"`
	JoinedAt   sql.NullTime
	LeftAt     sql.NullTime
	MutedAudio bool `gorm:"default:false"`
	MutedVideo bool `gorm:"default:false"`
	DeviceType string
}

// CallQualityMetric represents call_quality_metrics
type CallQualityMetric struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	CallID           uuid.UUID `gorm:"type:uuid;not null"`
	UserID           uuid.UUID `gorm:"type:uuid;not null"`
	RecordedAt       time.Time `gorm:"default:now()"`
	PacketsSent      int64
	PacketsReceived  int64
	PacketsLost      int64
	JitterMs         float64 `gorm:"type:decimal"`
	RoundTripTimeMs  float64 `gorm:"type:decimal"`
	BitrateKbps      int
	FrameRate        int
	ResolutionWidth  int
	ResolutionHeight int
	AudioLevel       float64 `gorm:"type:decimal"`
	ConnectionType   string
	IceCandidateType string
}

// TurnCredential represents turn_credentials
type TurnCredential struct {
	ID         uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.NullUUID `gorm:"type:uuid"`
	Username   string        `gorm:"not null"`
	Credential string        `gorm:"not null"`
	TTLSeconds int           `gorm:"not null"`
	Realm      string
	CreatedAt  time.Time `gorm:"default:now()"`
	ExpiresAt  time.Time `gorm:"not null"`
}

func (Call) TableName() string {
	return "calls"
}

func (CallParticipant) TableName() string {
	return "call_participants"
}

func (CallQualityMetric) TableName() string {
	return "call_quality_metrics"
}

func (TurnCredential) TableName() string {
	return "turn_credentials"
}
