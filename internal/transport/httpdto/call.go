package httpdto

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
