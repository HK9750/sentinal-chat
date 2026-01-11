package domain

import (
	"context"
	"time"
)

type User struct {
	ID          string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	PhoneNumber *string    `gorm:"type:citext;uniqueIndex:idx_users_phone" json:"phone_number,omitempty"`
	Username    *string    `gorm:"type:citext;uniqueIndex:idx_users_username" json:"username,omitempty"`
	DisplayName string     `gorm:"type:text;not null" json:"display_name"`
	Bio         *string    `gorm:"type:text" json:"bio,omitempty"`
	AvatarURL   *string    `gorm:"type:text" json:"avatar_url,omitempty"`
	IsOnline    bool       `gorm:"default:false" json:"is_online"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	// Relations
	Settings *UserSettings `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"settings,omitempty"`
}

type UserSettings struct {
	UserID string `gorm:"type:uuid;primaryKey" json:"user_id"`

	// Privacy
	PrivacyLastSeen     PrivacySetting `gorm:"type:privacy_setting;default:'EVERYONE'" json:"privacy_last_seen"`
	PrivacyProfilePhoto PrivacySetting `gorm:"type:privacy_setting;default:'EVERYONE'" json:"privacy_profile_photo"`
	PrivacyAbout        PrivacySetting `gorm:"type:privacy_setting;default:'EVERYONE'" json:"privacy_about"`
	PrivacyGroups       PrivacySetting `gorm:"type:privacy_setting;default:'EVERYONE'" json:"privacy_groups"`
	ReadReceipts        bool           `gorm:"default:true" json:"read_receipts"`

	// Notifications
	NotificationsEnabled bool   `gorm:"default:true" json:"notifications_enabled"`
	NotificationSound    string `gorm:"default:'default'" json:"notification_sound"`
	NotificationVibrate  bool   `gorm:"default:true" json:"notification_vibrate"`

	// App Preferences
	Theme                   ThemeMode    `gorm:"type:theme_mode;default:'SYSTEM'" json:"theme"`
	Language                LanguageCode `gorm:"type:language_code;default:'en'" json:"language"`
	EnterToSend             bool         `gorm:"default:true" json:"enter_to_send"`
	MediaAutoDownloadWifi   bool         `gorm:"default:true" json:"media_auto_download_wifi"`
	MediaAutoDownloadMobile bool         `gorm:"default:false" json:"media_auto_download_mobile"`

	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByPhoneNumber(ctx context.Context, phone string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	Update(ctx context.Context, user *User) error
}

type UserService interface {
	Register(ctx context.Context, username, phone, password string) (*User, error) // Password handling needs thought as it's not in User table?
	Login(ctx context.Context, identifier, password string) (string, error)
}
