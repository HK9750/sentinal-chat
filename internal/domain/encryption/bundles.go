package encryption

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// KeyBundle represents key_bundles
type KeyBundle struct {
	UserID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	DeviceID              string    `gorm:"primaryKey;not null"`
	IdentityKey           []byte    `gorm:"not null"`
	SignedPreKeyID        int       `gorm:"not null"`
	SignedPreKey          []byte    `gorm:"not null"`
	SignedPreKeySignature []byte    `gorm:"not null"`
	OneTimePreKeyID       sql.NullInt32
	OneTimePreKey         []byte
	UpdatedAt             time.Time `gorm:"default:now()"`
}

func (KeyBundle) TableName() string {
	return "key_bundles"
}
