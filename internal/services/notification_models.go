package services

import "time"

type NotificationSettingsView struct {
	InAppEnabled       bool `json:"in_app_enabled"`
	SoundEnabled       bool `json:"sound_enabled"`
	ShowMessagePreview bool `json:"show_message_preview"`
}

type UpdateNotificationSettingsInput struct {
	InAppEnabled       *bool
	SoundEnabled       *bool
	ShowMessagePreview *bool
}

type NotificationView struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	ActorID        *string        `json:"actor_id,omitempty"`
	ConversationID *string        `json:"conversation_id,omitempty"`
	MessageID      *string        `json:"message_id,omitempty"`
	CallID         *string        `json:"call_id,omitempty"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	DeepLink       string         `json:"deep_link"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IsRead         bool           `json:"is_read"`
	ReadAt         *time.Time     `json:"read_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
