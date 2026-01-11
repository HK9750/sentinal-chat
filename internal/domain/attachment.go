package domain

import (
	"time"
)

type Attachment struct {
	ID         string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UploaderID *string   `gorm:"type:uuid" json:"uploader_id,omitempty"`
	URL        string    `gorm:"type:text;not null" json:"url"`
	Filename   *string   `gorm:"type:text" json:"filename,omitempty"`
	MimeType   string    `gorm:"type:text;not null" json:"mime_type"`
	SizeBytes  int64     `gorm:"not null" json:"size_bytes"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}
