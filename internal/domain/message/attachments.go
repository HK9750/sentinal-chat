package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// LinkPreview represents link_previews
type LinkPreview struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	URL         string    `gorm:"not null"`
	URLHash     string    `gorm:"not null"`
	Title       sql.NullString
	Description sql.NullString
	ImageURL    sql.NullString
	SiteName    sql.NullString
	FetchedAt   time.Time `gorm:"default:now()"`
}

// Attachment represents attachments
type Attachment struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UploaderID        uuid.NullUUID `gorm:"type:uuid"`
	URL               string        `gorm:"not null"`
	Filename          sql.NullString
	MimeType          string `gorm:"not null"`
	SizeBytes         int64  `gorm:"not null"`
	ViewOnce          bool   `gorm:"default:false"`
	ViewedAt          sql.NullTime
	ThumbnailURL      sql.NullString
	Width             sql.NullInt32
	Height            sql.NullInt32
	DurationSeconds   sql.NullInt32
	EncryptionKeyHash sql.NullString
	EncryptionIV      sql.NullString
	CreatedAt         time.Time `gorm:"default:now()"`
}

// MessageAttachment represents message_attachments
type MessageAttachment struct {
	MessageID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	AttachmentID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

func (LinkPreview) TableName() string {
	return "link_previews"
}

func (Attachment) TableName() string {
	return "attachments"
}

func (MessageAttachment) TableName() string {
	return "message_attachments"
}
