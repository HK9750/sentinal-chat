package message

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Attachment represents attachments
type Attachment struct {
	ID              uuid.UUID
	UploaderID      uuid.NullUUID
	EncryptedURL    string
	Filename        sql.NullString
	MimeType        string
	SizeBytes       int64
	ViewOnce        bool
	ViewedAt        sql.NullTime
	ThumbnailURL    sql.NullString
	Width           sql.NullInt32
	Height          sql.NullInt32
	DurationSeconds sql.NullInt32
	CreatedAt       time.Time
}

// MessageAttachment represents message_attachments
type MessageAttachment struct {
	MessageID    uuid.UUID
	AttachmentID uuid.UUID
}

func (Attachment) TableName() string {
	return "attachments"
}

func (MessageAttachment) TableName() string {
	return "message_attachments"
}
