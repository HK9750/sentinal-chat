package user

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// User represents the users table
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	PhoneNumber  sql.NullString `gorm:"type:citext;unique"`
	Username     sql.NullString `gorm:"type:citext;unique"`
	Email        sql.NullString `gorm:"type:citext;unique"`
	PasswordHash string         `gorm:"not null"`
	DisplayName  string         `gorm:"not null"`
	Bio          string
	AvatarURL    string
	IsOnline     bool `gorm:"default:false"`
	LastSeenAt   sql.NullTime
	IsActive     bool      `gorm:"default:true"`
	IsVerified   bool      `gorm:"default:false"`
	CreatedAt    time.Time `gorm:"default:now()"`
	UpdatedAt    time.Time `gorm:"default:now()"`

	// Relationships
	Settings UserSettings  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Devices  []Device      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Sessions []UserSession `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// UserSettings represents the user_settings table
type UserSettings struct {
	UserID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	PrivacyLastSeen         string    `gorm:"type:privacy_setting;default:'EVERYONE'"`
	PrivacyProfilePhoto     string    `gorm:"type:privacy_setting;default:'EVERYONE'"`
	PrivacyAbout            string    `gorm:"type:privacy_setting;default:'EVERYONE'"`
	PrivacyGroups           string    `gorm:"type:privacy_setting;default:'EVERYONE'"`
	ReadReceipts            bool      `gorm:"default:true"`
	NotificationsEnabled    bool      `gorm:"default:true"`
	NotificationSound       string    `gorm:"default:'default'"`
	NotificationVibrate     bool      `gorm:"default:true"`
	Theme                   string    `gorm:"type:theme_mode;default:'SYSTEM'"`
	Language                string    `gorm:"type:language_code;default:'en'"`
	EnterToSend             bool      `gorm:"default:true"`
	MediaAutoDownloadWiFi   bool      `gorm:"default:true"`
	MediaAutoDownloadMobile bool      `gorm:"default:false"`
	UpdatedAt               time.Time `gorm:"default:now()"`
}

// Device represents the devices table
type Device struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID       uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID     string    `gorm:"not null"`
	DeviceName   string
	DeviceType   string
	IsActive     bool      `gorm:"default:true"`
	RegisteredAt time.Time `gorm:"default:now()"`
	LastSeenAt   sql.NullTime

	// Unique constraint handled by index in SQL, but good to note: UNIQUE(user_id, device_id)
}

// PushToken represents the push_tokens table
type PushToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID   uuid.UUID `gorm:"type:uuid;not null"`
	Platform   string    `gorm:"not null"`
	Token      string    `gorm:"not null"`
	IsActive   bool      `gorm:"default:true"`
	CreatedAt  time.Time `gorm:"default:now()"`
	LastUsedAt sql.NullTime
}

// UserSession represents the user_sessions table
type UserSession struct {
	ID               uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID           uuid.UUID     `gorm:"type:uuid;not null"`
	DeviceID         uuid.NullUUID `gorm:"type:uuid"`
	RefreshTokenHash string        `gorm:"not null"`
	ExpiresAt        time.Time     `gorm:"not null"`
	IsRevoked        bool          `gorm:"default:false"`
	CreatedAt        time.Time     `gorm:"default:now()"`
}

// UserContact represents the user_contacts table
type UserContact struct {
	UserID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContactUserID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Nickname      string
	IsBlocked     bool      `gorm:"default:false"`
	CreatedAt     time.Time `gorm:"default:now()"`
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
