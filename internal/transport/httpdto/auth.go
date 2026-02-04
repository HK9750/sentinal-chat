package httpdto

type RegisterRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	DeviceID    string `json:"device_id"`
	DeviceName  string `json:"device_name"`
	DeviceType  string `json:"device_type"`
}

type LoginRequest struct {
	Identity   string `json:"identity"`
	Password   string `json:"password"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
}

type RefreshRequest struct {
	SessionID    string `json:"session_id"`
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	SessionID string `json:"session_id"`
}

type PasswordForgotRequest struct {
	Identity string `json:"identity"`
}

type PasswordResetRequest struct {
	Identity    string `json:"identity"`
	NewPassword string `json:"new_password"`
}
