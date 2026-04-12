package httpdto

import "time"

type CreateConversationRequest struct {
	Type             string   `json:"type" binding:"required"`
	Subject          string   `json:"subject,omitempty"`
	Description      string   `json:"description,omitempty"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	ParticipantIDs   []string `json:"participant_ids" binding:"required,min=1"`
	DisappearingMode string   `json:"disappearing_mode,omitempty"`
}

type AddParticipantRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
	Role   string `json:"role,omitempty"`
}

type UpdateDisappearingModeRequest struct {
	DisappearingMode string `json:"disappearing_mode" binding:"required"`
}

type UpdateMuteConversationRequest struct {
	MutedUntil *time.Time `json:"muted_until,omitempty"`
}

type DeleteConversationPayload struct {
	Deleted bool `json:"deleted"`
}

type CallHistoryQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

type ConversationQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

type MessageHistoryQuery struct {
	BeforeSeq int64 `form:"before_seq"`
	Limit     int   `form:"limit"`
}

type WebSocketInboundFrame struct {
	Type           string `json:"type"`
	RequestID      string `json:"request_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	CallID         string `json:"call_id,omitempty"`
	Data           any    `json:"data,omitempty"`
}

type ClearConversationPayload struct {
	Cleared bool `json:"cleared"`
}

type CallPayload struct {
	CallID         string    `json:"call_id"`
	ConversationID string    `json:"conversation_id"`
	Type           string    `json:"type"`
	InitiatedBy    string    `json:"initiated_by"`
	StartedAt      time.Time `json:"started_at"`
}
