package httpdto

import (
	"sentinal-chat/internal/domain/call"
	"strconv"
	"time"
)

// CreateCallRequest is used for POST /calls
type CreateCallRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
	Type           string `json:"type" binding:"required"` // "AUDIO" or "VIDEO"
	InitiatorID    string `json:"initiator_id" binding:"required"`
}

// CreateCallResponse is returned after creating a call
type CreateCallResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	InitiatorID    string `json:"initiator_id"`
	CreatedAt      string `json:"created_at"`
}

// AddCallParticipantRequest is used for POST /calls/:id/participants
type AddCallParticipantRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// UpdateParticipantStatusRequest is used for PUT /calls/:id/participants/:user_id/status
type UpdateParticipantStatusRequest struct {
	Status string `json:"status" binding:"required"` // "INVITED", "JOINED", "LEFT"
}

// UpdateParticipantMuteRequest is used for PUT /calls/:id/participants/:user_id/mute
type UpdateParticipantMuteRequest struct {
	AudioMuted bool `json:"audio_muted"`
	VideoMuted bool `json:"video_muted"`
}

// EndCallRequest is used for POST /calls/:id/end
type EndCallRequest struct {
	Reason string `json:"reason,omitempty"` // "COMPLETED", "MISSED", "DECLINED", "FAILED", "TIMEOUT", "NETWORK_ERROR"
}

// RecordCallQualityMetricRequest is used for POST /calls/quality
type RecordCallQualityMetricRequest struct {
	CallID           string  `json:"call_id" binding:"required"`
	UserID           string  `json:"user_id" binding:"required"`
	Timestamp        string  `json:"timestamp,omitempty"`
	PacketsSent      int64   `json:"packets_sent,omitempty"`
	PacketsReceived  int64   `json:"packets_received,omitempty"`
	PacketLoss       float64 `json:"packet_loss,omitempty"`
	PacketsLost      int64   `json:"packets_lost,omitempty"`
	Jitter           float64 `json:"jitter,omitempty"`
	Latency          float64 `json:"latency,omitempty"`
	Bitrate          int64   `json:"bitrate,omitempty"`
	FrameRate        int     `json:"frame_rate,omitempty"`
	Resolution       string  `json:"resolution,omitempty"`
	ResolutionWidth  int     `json:"resolution_width,omitempty"`
	ResolutionHeight int     `json:"resolution_height,omitempty"`
	AudioLevel       float64 `json:"audio_level,omitempty"`
	ConnectionType   string  `json:"connection_type,omitempty"`
	IceCandidateType string  `json:"ice_candidate_type,omitempty"`
}

// ListCallsRequest holds query parameters for listing calls
type ListCallsRequest struct {
	ConversationID string `form:"conversation_id"`
	UserID         string `form:"user_id"`
	Page           int    `form:"page"`
	Limit          int    `form:"limit"`
}

// ListCallsResponse is returned when listing calls
type ListCallsResponse struct {
	Calls []CallDTO `json:"calls"`
	Total int64     `json:"total"`
}

// CallDTO represents a call in API responses
type CallDTO struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	InitiatorID    string `json:"initiator_id"`
	StartedAt      string `json:"started_at,omitempty"`
	EndedAt        string `json:"ended_at,omitempty"`
	Duration       int64  `json:"duration,omitempty"`
}

// CallParticipantsResponse is returned when listing call participants
type CallParticipantsResponse struct {
	Participants []CallParticipantDTO `json:"participants"`
}

// CallParticipantDTO represents a call participant in API responses
type CallParticipantDTO struct {
	UserID     string `json:"user_id"`
	Status     string `json:"status"`
	AudioMuted bool   `json:"audio_muted"`
	VideoMuted bool   `json:"video_muted"`
	JoinedAt   string `json:"joined_at,omitempty"`
	LeftAt     string `json:"left_at,omitempty"`
}

// CallDurationResponse is returned when getting call duration
type CallDurationResponse struct {
	Duration int64 `json:"duration"` // in seconds
}

// CallQualityMetricsResponse is returned when getting quality metrics
type CallQualityMetricsResponse struct {
	Metrics []CallQualityMetricDTO `json:"metrics"`
}

// CallQualityMetricDTO represents a quality metric in API responses
type CallQualityMetricDTO struct {
	ID         string  `json:"id"`
	CallID     string  `json:"call_id"`
	UserID     string  `json:"user_id"`
	Timestamp  string  `json:"timestamp"`
	PacketLoss float64 `json:"packet_loss"`
	Jitter     float64 `json:"jitter"`
	Latency    float64 `json:"latency"`
	Bitrate    int64   `json:"bitrate"`
	FrameRate  int     `json:"frame_rate,omitempty"`
	Resolution string  `json:"resolution,omitempty"`
	AudioLevel float64 `json:"audio_level,omitempty"`
}

// AverageCallQualityResponse is returned for average quality
type AverageCallQualityResponse struct {
	Average float64 `json:"average"`
}

// DeletedCountResponse is a generic response for delete operations
type DeletedCountResponse struct {
	Deleted int64 `json:"deleted"`
}

// FromCall converts a domain call to CallDTO
func FromCall(c call.Call) CallDTO {
	dto := CallDTO{
		ID:             c.ID.String(),
		ConversationID: c.ConversationID.String(),
		Type:           c.Type,
		InitiatorID:    c.InitiatedBy.String(),
	}
	if !c.StartedAt.IsZero() {
		dto.StartedAt = c.StartedAt.Format(time.RFC3339)
	}
	if c.ConnectedAt.Valid {
		dto.Status = "CONNECTED"
	}
	if c.EndedAt.Valid {
		dto.EndedAt = c.EndedAt.Time.Format(time.RFC3339)
		dto.Status = "ENDED"
	}
	if dto.Status == "" {
		if !c.StartedAt.IsZero() {
			dto.Status = "RINGING"
		} else {
			dto.Status = "INITIATED"
		}
	}
	if c.DurationSeconds.Valid {
		dto.Duration = int64(c.DurationSeconds.Int32)
	}
	return dto
}

// FromCallSlice converts a slice of domain calls to CallDTO slice
func FromCallSlice(calls []call.Call) []CallDTO {
	dtos := make([]CallDTO, len(calls))
	for i, c := range calls {
		dtos[i] = FromCall(c)
	}
	return dtos
}

// FromCallParticipant converts a domain call participant to CallParticipantDTO
func FromCallParticipant(p call.CallParticipant) CallParticipantDTO {
	dto := CallParticipantDTO{
		UserID:     p.UserID.String(),
		Status:     p.Status,
		AudioMuted: p.MutedAudio,
		VideoMuted: p.MutedVideo,
	}
	if p.JoinedAt.Valid {
		dto.JoinedAt = p.JoinedAt.Time.Format(time.RFC3339)
	}
	if p.LeftAt.Valid {
		dto.LeftAt = p.LeftAt.Time.Format(time.RFC3339)
	}
	return dto
}

// FromCallParticipantSlice converts a slice of domain call participants to CallParticipantDTO slice
func FromCallParticipantSlice(participants []call.CallParticipant) []CallParticipantDTO {
	dtos := make([]CallParticipantDTO, len(participants))
	for i, p := range participants {
		dtos[i] = FromCallParticipant(p)
	}
	return dtos
}

// FromCallQualityMetric converts a domain call quality metric to CallQualityMetricDTO
func FromCallQualityMetric(m call.CallQualityMetric) CallQualityMetricDTO {
	dto := CallQualityMetricDTO{
		ID:         m.ID.String(),
		CallID:     m.CallID.String(),
		UserID:     m.UserID.String(),
		Timestamp:  m.RecordedAt.Format(time.RFC3339),
		Jitter:     m.JitterMs,
		Latency:    m.RoundTripTimeMs,
		Bitrate:    int64(m.BitrateKbps),
		FrameRate:  m.FrameRate,
		AudioLevel: m.AudioLevel,
	}
	if m.PacketsSent > 0 {
		dto.PacketLoss = float64(m.PacketsLost) / float64(m.PacketsSent)
	} else if m.PacketsLost > 0 {
		dto.PacketLoss = float64(m.PacketsLost)
	}
	if m.ResolutionWidth > 0 && m.ResolutionHeight > 0 {
		dto.Resolution = strconv.Itoa(m.ResolutionWidth) + "x" + strconv.Itoa(m.ResolutionHeight)
	}
	return dto
}

// FromCallQualityMetricSlice converts a slice of domain call quality metrics to CallQualityMetricDTO slice
func FromCallQualityMetricSlice(metrics []call.CallQualityMetric) []CallQualityMetricDTO {
	dtos := make([]CallQualityMetricDTO, len(metrics))
	for i, m := range metrics {
		dtos[i] = FromCallQualityMetric(m)
	}
	return dtos
}
