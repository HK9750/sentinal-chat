package httpdto

type NotificationQuery struct {
	Page       int  `form:"page"`
	Limit      int  `form:"limit"`
	UnreadOnly bool `form:"unread_only"`
}

type UpdateNotificationSettingsRequest struct {
	InAppEnabled       *bool `json:"in_app_enabled,omitempty"`
	SoundEnabled       *bool `json:"sound_enabled,omitempty"`
	ShowMessagePreview *bool `json:"show_message_preview,omitempty"`
}

type MarkNotificationReadPayload struct {
	Read bool `json:"read"`
}

type MarkAllNotificationsReadPayload struct {
	Updated int64 `json:"updated"`
}
