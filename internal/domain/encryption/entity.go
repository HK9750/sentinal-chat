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
	DeviceID  string    `gorm:"not null"`
	PublicKey []byte    `gorm:"not null"`
	IsActive  bool      `gorm:"default:true"`
	CreatedAt time.Time `gorm:"default:now()"`
	// Unique(user_id, device_id) - handled by idx/constrain in SQL
}

// SignedPreKey represents signed_prekeys
type SignedPreKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID  string    `gorm:"not null"`
	KeyID     int       `gorm:"not null"`
	PublicKey []byte    `gorm:"not null"`
	Signature []byte    `gorm:"not null"`
	CreatedAt time.Time `gorm:"default:now()"`
	IsActive  bool      `gorm:"default:true"`
}

// OneTimePreKey represents onetime_prekeys
type OneTimePreKey struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"`
	DeviceID   string    `gorm:"not null"`
	KeyID      int       `gorm:"not null"`
	PublicKey  []byte    `gorm:"not null"`
	UploadedAt time.Time `gorm:"default:now()"`
	ConsumedAt sql.NullTime
	ConsumedBy uuid.NullUUID `gorm:"type:uuid"`
}

// EncryptedSession represents encrypted_sessions
type EncryptedSession struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	LocalUserID    uuid.UUID `gorm:"type:uuid;not null"`
	LocalDeviceID  string    `gorm:"not null"`
	RemoteUserID   uuid.UUID `gorm:"type:uuid;not null"`
	RemoteDeviceID string    `gorm:"not null"`
	EncryptedState []byte    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"default:now()"`
	UpdatedAt      time.Time `gorm:"default:now()"`
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

func (EncryptedSession) TableName() string {
	return "encrypted_sessions"
}
