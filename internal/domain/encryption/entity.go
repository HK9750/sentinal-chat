package encryption

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// IdentityKey represents identity_keys
type IdentityKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID  uuid.UUID `gorm:"type:uuid;not null"`
	PublicKey []byte    `gorm:"not null"`
	IsActive  bool      `gorm:"default:true"`
	CreatedAt time.Time `gorm:"default:now()"`
	// Unique(device_id) - handled by idx/constraint in SQL
}

// SignedPreKey represents signed_prekeys
type SignedPreKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID  uuid.UUID `gorm:"type:uuid;not null"`
	KeyID     int       `gorm:"not null"`
	PublicKey []byte    `gorm:"not null"`
	Signature []byte    `gorm:"not null"`
	CreatedAt time.Time `gorm:"default:now()"`
	IsActive  bool      `gorm:"default:true"`
}

// OneTimePreKey represents onetime_prekeys
type OneTimePreKey struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID             uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID           uuid.UUID `gorm:"type:uuid;not null"`
	KeyID              int       `gorm:"not null"`
	PublicKey          []byte    `gorm:"not null"`
	UploadedAt         time.Time `gorm:"default:now()"`
	ConsumedAt         sql.NullTime
	ConsumedBy         uuid.NullUUID `gorm:"type:uuid"`
	ConsumedByDeviceID uuid.NullUUID `gorm:"type:uuid"`
}

func (IdentityKey) TableName() string {
	return "identity_keys"
}

func (SignedPreKey) TableName() string {
	return "signed_prekeys"
}

func (OneTimePreKey) TableName() string {
	return "onetime_prekeys"
}
