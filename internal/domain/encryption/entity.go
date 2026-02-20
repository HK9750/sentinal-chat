package encryption

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// IdentityKey represents identity_keys
type IdentityKey struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	PublicKey []byte
	IsActive  bool
	CreatedAt time.Time
	// Unique(device_id) - handled by idx/constraint in SQL
}

// SignedPreKey represents signed_prekeys
type SignedPreKey struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	KeyID     int
	PublicKey []byte
	Signature []byte
	CreatedAt time.Time
	IsActive  bool
}

// OneTimePreKey represents onetime_prekeys
type OneTimePreKey struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	DeviceID           uuid.UUID
	KeyID              int
	PublicKey          []byte
	UploadedAt         time.Time
	ConsumedAt         sql.NullTime
	ConsumedBy         uuid.NullUUID
	ConsumedByDeviceID uuid.NullUUID
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
