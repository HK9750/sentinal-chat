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
	Bio          string
	AvatarURL    string
	IsOnline     bool
	LastSeenAt   sql.NullTime
	IsActive     bool
	IsVerified   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Relationships
	Devices  []Device
	Sessions []UserSession
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
}

// FcmToken represents the fcm_tokens table
type FcmToken struct {
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
	AuthProvider     string
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

func (Device) TableName() string {
	return "devices"
}

func (FcmToken) TableName() string {
	return "fcm_tokens"
}

func (UserSession) TableName() string {
	return "user_sessions"
}

func (UserContact) TableName() string {
	return "user_contacts"
}
