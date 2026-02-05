package commands

import (
	"time"

	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// InitiateCallCommand starts a new call
type InitiateCallCommand struct {
	ConversationID      uuid.UUID
	InitiatorID         uuid.UUID
	CallType            string // AUDIO, VIDEO, SCREEN_SHARE
	Topology            string // P2P, MESH, SFU
	IsGroupCall         bool
	IdempotencyKeyValue string
}

func (InitiateCallCommand) CommandType() string { return "call.initiate" }

func (c InitiateCallCommand) Validate() error {
	if c.ConversationID == uuid.Nil || c.InitiatorID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	validTypes := map[string]bool{"AUDIO": true, "VIDEO": true, "SCREEN_SHARE": true}
	if !validTypes[c.CallType] {
		return sentinal_errors.ErrInvalidInput
	}
	validTopologies := map[string]bool{"P2P": true, "MESH": true, "SFU": true}
	if !validTopologies[c.Topology] {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c InitiateCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c InitiateCallCommand) ActorID() uuid.UUID { return c.InitiatorID }

// AcceptCallCommand accepts an incoming call
type AcceptCallCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (AcceptCallCommand) CommandType() string { return "call.accept" }

func (c AcceptCallCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c AcceptCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c AcceptCallCommand) ActorID() uuid.UUID { return c.UserID }

// RejectCallCommand rejects an incoming call
type RejectCallCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (RejectCallCommand) CommandType() string { return "call.reject" }

func (c RejectCallCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RejectCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RejectCallCommand) ActorID() uuid.UUID { return c.UserID }

// EndCallCommand ends an active call
type EndCallCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	Reason              string // COMPLETED, MISSED, DECLINED, FAILED, TIMEOUT, NETWORK_ERROR
	IdempotencyKeyValue string
}

func (EndCallCommand) CommandType() string { return "call.end" }

func (c EndCallCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	validReasons := map[string]bool{
		"COMPLETED": true, "MISSED": true, "DECLINED": true,
		"FAILED": true, "TIMEOUT": true, "NETWORK_ERROR": true,
	}
	if c.Reason != "" && !validReasons[c.Reason] {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c EndCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c EndCallCommand) ActorID() uuid.UUID { return c.UserID }

// JoinCallCommand joins an ongoing call
type JoinCallCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	DeviceType          string
	IdempotencyKeyValue string
}

func (JoinCallCommand) CommandType() string { return "call.join" }

func (c JoinCallCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c JoinCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c JoinCallCommand) ActorID() uuid.UUID { return c.UserID }

// LeaveCallCommand leaves an ongoing call
type LeaveCallCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	IdempotencyKeyValue string
}

func (LeaveCallCommand) CommandType() string { return "call.leave" }

func (c LeaveCallCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c LeaveCallCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c LeaveCallCommand) ActorID() uuid.UUID { return c.UserID }

// ToggleMuteCommand toggles audio/video mute
type ToggleMuteCommand struct {
	CallID              uuid.UUID
	UserID              uuid.UUID
	MuteAudio           *bool
	MuteVideo           *bool
	IdempotencyKeyValue string
}

func (ToggleMuteCommand) CommandType() string { return "call.toggle_mute" }

func (c ToggleMuteCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.MuteAudio == nil && c.MuteVideo == nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ToggleMuteCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ToggleMuteCommand) ActorID() uuid.UUID { return c.UserID }

// SendOfferCommand sends WebRTC SDP offer
type SendOfferCommand struct {
	CallID uuid.UUID
	FromID uuid.UUID
	ToID   uuid.UUID
	SDP    string
}

func (SendOfferCommand) CommandType() string { return "call.offer" }

func (c SendOfferCommand) Validate() error {
	if c.CallID == uuid.Nil || c.FromID == uuid.Nil || c.ToID == uuid.Nil || c.SDP == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendOfferCommand) IdempotencyKey() string { return "" }

func (c SendOfferCommand) ActorID() uuid.UUID { return c.FromID }

// SendAnswerCommand sends WebRTC SDP answer
type SendAnswerCommand struct {
	CallID uuid.UUID
	FromID uuid.UUID
	ToID   uuid.UUID
	SDP    string
}

func (SendAnswerCommand) CommandType() string { return "call.answer" }

func (c SendAnswerCommand) Validate() error {
	if c.CallID == uuid.Nil || c.FromID == uuid.Nil || c.ToID == uuid.Nil || c.SDP == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendAnswerCommand) IdempotencyKey() string { return "" }

func (c SendAnswerCommand) ActorID() uuid.UUID { return c.FromID }

// SendICECandidateCommand sends WebRTC ICE candidate
type SendICECandidateCommand struct {
	CallID        uuid.UUID
	FromID        uuid.UUID
	ToID          uuid.UUID
	Candidate     string
	SDPMid        string
	SDPMLineIndex int
}

func (SendICECandidateCommand) CommandType() string { return "call.ice" }

func (c SendICECandidateCommand) Validate() error {
	if c.CallID == uuid.Nil || c.FromID == uuid.Nil || c.ToID == uuid.Nil || c.Candidate == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c SendICECandidateCommand) IdempotencyKey() string { return "" }

func (c SendICECandidateCommand) ActorID() uuid.UUID { return c.FromID }

// RecordQualityMetricCommand records call quality metrics
type RecordQualityMetricCommand struct {
	CallID           uuid.UUID
	UserID           uuid.UUID
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
	ICECandidateType string
	RecordedAt       time.Time
}

func (RecordQualityMetricCommand) CommandType() string { return "call.record_quality" }

func (c RecordQualityMetricCommand) Validate() error {
	if c.CallID == uuid.Nil || c.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RecordQualityMetricCommand) IdempotencyKey() string { return "" }

func (c RecordQualityMetricCommand) ActorID() uuid.UUID { return c.UserID }
