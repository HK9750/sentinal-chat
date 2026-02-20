package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// LinkPreview represents link_previews
type LinkPreview struct {
	ID          uuid.UUID
	URL         string
	URLHash     string
	Title       sql.NullString
	Description sql.NullString
	ImageURL    sql.NullString
	SiteName    sql.NullString
	FetchedAt   time.Time
}

// Attachment represents attachments
type Attachment struct {
	ID                uuid.UUID
	UploaderID        uuid.NullUUID
	URL               string
	Filename          sql.NullString
	MimeType          string
	SizeBytes         int64
	ViewOnce          bool
	ViewedAt          sql.NullTime
	ThumbnailURL      sql.NullString
	Width             sql.NullInt32
	Height            sql.NullInt32
	DurationSeconds   sql.NullInt32
	EncryptionKeyHash sql.NullString
	EncryptionIV      sql.NullString
	CreatedAt         time.Time
}

// MessageAttachment represents message_attachments
type MessageAttachment struct {
	MessageID    uuid.UUID
	AttachmentID uuid.UUID
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
