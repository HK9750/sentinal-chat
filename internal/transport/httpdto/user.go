package httpdto

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
	Status      string `json:"status,omitempty"`
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
	Status      string `json:"status,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// ContactsResponse is returned when listing contacts
type ContactsResponse struct {
	Contacts []UserDTO `json:"contacts"`
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
