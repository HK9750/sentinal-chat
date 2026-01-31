package upload

import (
	"time"

	"github.com/google/uuid"
)

// UploadSession represents upload_sessions
type UploadSession struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UploaderID    uuid.UUID `gorm:"type:uuid;not null"`
	Filename      string    `gorm:"not null"`
	MimeType      string    `gorm:"not null"`
	SizeBytes     int64     `gorm:"not null"`
	ChunkSize     int       `gorm:"not null"`
	UploadedBytes int64     `gorm:"default:0"`
	Status        string    `gorm:"type:upload_status;default:'IN_PROGRESS'"`
	CreatedAt     time.Time `gorm:"default:now()"`
	UpdatedAt     time.Time `gorm:"default:now()"`
}

func (UploadSession) TableName() string {
	return "upload_sessions"
}
