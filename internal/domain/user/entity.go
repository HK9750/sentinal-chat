package user

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// User represents the users table
type User struct {
	ID           uuid.UUID
	PhoneNumber  sql.NullString
	Username     sql.NullString
	Email        sql.NullString
	PasswordHash string
	DisplayName  string
	Role         string // SUPER_ADMIN, ADMIN, MODERATOR, USER
	Bio          string
	AvatarURL    string
	IsOnline     bool
	LastSeenAt   sql.NullTime
	IsActive     bool
	IsVerified   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Relationships
	Settings UserSettings
	Devices  []Device
	Sessions []UserSession
}

// UserSettings represents the user_settings table
type UserSettings struct {
	UserID                  uuid.UUID
	PrivacyLastSeen         string
	PrivacyProfilePhoto     string
	PrivacyAbout            string
	PrivacyGroups           string
	ReadReceipts            bool
	NotificationsEnabled    bool
	NotificationSound       string
	NotificationVibrate     bool
	Theme                   string
	Language                string
	EnterToSend             bool
	MediaAutoDownloadWiFi   bool
	MediaAutoDownloadMobile bool
	UpdatedAt               time.Time
}

// Device represents the devices table
type Device struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	DeviceID     string
	DeviceName   string
	DeviceType   string
	IsActive     bool
	RegisteredAt time.Time
	LastSeenAt   sql.NullTime

	// Unique constraint handled by index in SQL, but good to note: UNIQUE(user_id, device_id)
}

// PushToken represents the push_tokens table
type PushToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DeviceID   uuid.UUID
	Platform   string
	Token      string
	IsActive   bool
	CreatedAt  time.Time
	LastUsedAt sql.NullTime
}

// UserSession represents the user_sessions table
type UserSession struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	DeviceID         *uuid.UUID
	Device           *Device
	RefreshTokenHash string
	ExpiresAt        time.Time
	IsRevoked        bool
	CreatedAt        time.Time
}

// UserContact represents the user_contacts table
type UserContact struct {
	UserID        uuid.UUID
	ContactUserID uuid.UUID
	Nickname      string
	IsBlocked     bool
	CreatedAt     time.Time
}

func (User) TableName() string {
	return "users"
}

func (UserSettings) TableName() string {
	return "user_settings"
}

func (Device) TableName() string {
	return "devices"
}

func (PushToken) TableName() string {
	return "push_tokens"
}

func (UserSession) TableName() string {
	return "user_sessions"
}

func (UserContact) TableName() string {
	return "user_contacts"
}
