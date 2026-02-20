package upload

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// UploadSession represents upload_sessions
type UploadSession struct {
	ID            uuid.UUID
	UploaderID    uuid.UUID
	Filename      string
	MimeType      string
	SizeBytes     int64
	ChunkSize     int
	UploadedBytes int64
	Status        string
	ObjectKey     string
	FileURL       sql.NullString
	CompletedAt   sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UploadSession) TableName() string {
	return "upload_sessions"
}
