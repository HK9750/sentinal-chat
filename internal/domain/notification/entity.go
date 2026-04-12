package notification

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ActorID        uuid.NullUUID
	ConversationID uuid.NullUUID
	MessageID      uuid.NullUUID
	CallID         uuid.NullUUID
	Type           string
	Title          string
	Body           string
	DeepLink       string
	Metadata       []byte
	DedupeKey      sql.NullString
	IsRead         bool
	ReadAt         sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UserNotificationSettings struct {
	UserID             uuid.UUID
	InAppEnabled       bool
	SoundEnabled       bool
	ShowMessagePreview bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (Notification) TableName() string {
	return "notifications"
}

func (UserNotificationSettings) TableName() string {
	return "user_notification_settings"
}
