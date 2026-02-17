package httpdto

import (
	"sentinal-chat/internal/domain/user"
	"time"

	"github.com/google/uuid"
)

// AddContactRequest is used for POST /users/contacts
type AddContactRequest struct {
	ContactUserID string `json:"contact_user_id" binding:"required"`
}

// AddContactResponse is returned after adding a contact
type AddContactResponse struct {
	Success bool `json:"success"`
}

// UpdateProfileRequest is used for PUT /users/profile
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Bio         string `json:"bio,omitempty"`
}

// UpdateSettingsRequest is used for PUT /users/settings
type UpdateSettingsRequest struct {
	NotificationsEnabled bool   `json:"notifications_enabled,omitempty"`
	Theme                string `json:"theme,omitempty"`
	Language             string `json:"language,omitempty"`
}

// ListUsersRequest holds query parameters for listing users
type ListUsersRequest struct {
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
	Search string `form:"search"`
}

// ListUsersResponse is returned when listing users
type ListUsersResponse struct {
	Users []UserDTO `json:"users"`
	Total int64     `json:"total"`
}

// UserDTO represents a user in API responses
type UserDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// ContactsResponse is returned when listing contacts
type ContactsResponse struct {
	Contacts []UserContactDTO `json:"contacts"`
}

// DevicesResponse is returned when listing devices
type DevicesResponse struct {
	Devices []DeviceDTO `json:"devices"`
}

// DeviceDTO represents a device in API responses
type DeviceDTO struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	LastActive string `json:"last_active,omitempty"`
	IsActive   bool   `json:"is_active"`
}

// PushTokensResponse is returned when listing push tokens
type PushTokensResponse struct {
	Tokens []PushTokenDTO `json:"tokens"`
}

// PushTokenDTO represents a push token in API responses
type PushTokenDTO struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	Platform  string `json:"platform"`
	CreatedAt string `json:"created_at"`
}

// UserSettingsDTO represents user settings in API responses
type UserSettingsDTO struct {
	UserID                  string `json:"user_id"`
	PrivacyLastSeen         string `json:"privacy_last_seen"`
	PrivacyProfilePhoto     string `json:"privacy_profile_photo"`
	PrivacyAbout            string `json:"privacy_about"`
	PrivacyGroups           string `json:"privacy_groups"`
	ReadReceipts            bool   `json:"read_receipts"`
	NotificationsEnabled    bool   `json:"notifications_enabled"`
	NotificationSound       string `json:"notification_sound"`
	NotificationVibrate     bool   `json:"notification_vibrate"`
	Theme                   string `json:"theme"`
	Language                string `json:"language"`
	EnterToSend             bool   `json:"enter_to_send"`
	MediaAutoDownloadWiFi   bool   `json:"media_auto_download_wifi"`
	MediaAutoDownloadMobile bool   `json:"media_auto_download_mobile"`
	UpdatedAt               string `json:"updated_at"`
}

// UserContactDTO represents a user contact in API responses
type UserContactDTO struct {
	UserID        string `json:"user_id"`
	ContactUserID string `json:"contact_user_id"`
	Nickname      string `json:"nickname,omitempty"`
	IsBlocked     bool   `json:"is_blocked"`
	CreatedAt     string `json:"created_at"`
}

// FromUser converts a domain user to UserDTO
func FromUser(u user.User) UserDTO {
	dto := UserDTO{
		ID:          u.ID.String(),
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
	}
	if u.Email.Valid {
		dto.Email = u.Email.String
	}
	if u.Username.Valid {
		dto.Username = u.Username.String
	}
	return dto
}

// FromUserSlice converts a slice of domain users to UserDTO slice
func FromUserSlice(users []user.User) []UserDTO {
	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = FromUser(u)
	}
	return dtos
}

// FromDevice converts a domain device to DeviceDTO
func FromDevice(d user.Device) DeviceDTO {
	dto := DeviceDTO{
		ID:         d.ID.String(),
		DeviceName: d.DeviceName,
		DeviceType: d.DeviceType,
		IsActive:   d.IsActive,
	}
	if d.LastSeenAt.Valid {
		dto.LastActive = d.LastSeenAt.Time.Format(time.RFC3339)
	}
	return dto
}

// FromDeviceSlice converts a slice of domain devices to DeviceDTO slice
func FromDeviceSlice(devices []user.Device) []DeviceDTO {
	dtos := make([]DeviceDTO, len(devices))
	for i, d := range devices {
		dtos[i] = FromDevice(d)
	}
	return dtos
}

// FromPushToken converts a domain push token to PushTokenDTO
func FromPushToken(t user.PushToken) PushTokenDTO {
	return PushTokenDTO{
		ID:        t.ID.String(),
		Token:     t.Token,
		Platform:  t.Platform,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
}

// FromPushTokenSlice converts a slice of domain push tokens to PushTokenDTO slice
func FromPushTokenSlice(tokens []user.PushToken) []PushTokenDTO {
	dtos := make([]PushTokenDTO, len(tokens))
	for i, t := range tokens {
		dtos[i] = FromPushToken(t)
	}
	return dtos
}

// FromUserSettings converts domain user settings to UserSettingsDTO
func FromUserSettings(s user.UserSettings) UserSettingsDTO {
	return UserSettingsDTO{
		UserID:                  s.UserID.String(),
		PrivacyLastSeen:         s.PrivacyLastSeen,
		PrivacyProfilePhoto:     s.PrivacyProfilePhoto,
		PrivacyAbout:            s.PrivacyAbout,
		PrivacyGroups:           s.PrivacyGroups,
		ReadReceipts:            s.ReadReceipts,
		NotificationsEnabled:    s.NotificationsEnabled,
		NotificationSound:       s.NotificationSound,
		NotificationVibrate:     s.NotificationVibrate,
		Theme:                   s.Theme,
		Language:                s.Language,
		EnterToSend:             s.EnterToSend,
		MediaAutoDownloadWiFi:   s.MediaAutoDownloadWiFi,
		MediaAutoDownloadMobile: s.MediaAutoDownloadMobile,
		UpdatedAt:               s.UpdatedAt.Format(time.RFC3339),
	}
}

// FromUserContact converts a domain user contact to UserContactDTO
func FromUserContact(c user.UserContact) UserContactDTO {
	return UserContactDTO{
		UserID:        c.UserID.String(),
		ContactUserID: c.ContactUserID.String(),
		Nickname:      c.Nickname,
		IsBlocked:     c.IsBlocked,
		CreatedAt:     c.CreatedAt.Format(time.RFC3339),
	}
}

// FromUserContactSlice converts a slice of domain user contacts to UserContactDTO slice
func FromUserContactSlice(contacts []user.UserContact) []UserContactDTO {
	dtos := make([]UserContactDTO, len(contacts))
	for i, c := range contacts {
		dtos[i] = FromUserContact(c)
	}
	return dtos
}

// StringUUID converts a uuid.UUID to string
func StringUUID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
