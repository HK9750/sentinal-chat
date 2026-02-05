package commands

import (
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// RegisterIdentityKeyCommand registers an identity key
type RegisterIdentityKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            string
	PublicKey           []byte
	IdempotencyKeyValue string
}

func (RegisterIdentityKeyCommand) CommandType() string { return "encryption.register_identity_key" }

func (c RegisterIdentityKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" || len(c.PublicKey) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RegisterIdentityKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RegisterIdentityKeyCommand) ActorID() uuid.UUID { return c.UserID }

// UploadSignedPreKeyCommand uploads a signed pre-key
type UploadSignedPreKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            string
	KeyID               int
	PublicKey           []byte
	Signature           []byte
	IdempotencyKeyValue string
}

func (UploadSignedPreKeyCommand) CommandType() string { return "encryption.upload_signed_prekey" }

func (c UploadSignedPreKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" || len(c.PublicKey) == 0 || len(c.Signature) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UploadSignedPreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UploadSignedPreKeyCommand) ActorID() uuid.UUID { return c.UserID }

// UploadOneTimePreKeysCommand uploads one-time pre-keys
type UploadOneTimePreKeysCommand struct {
	UserID   uuid.UUID
	DeviceID string
	Keys     []OneTimePreKeyInput
}

type OneTimePreKeyInput struct {
	KeyID     int
	PublicKey []byte
}

func (UploadOneTimePreKeysCommand) CommandType() string { return "encryption.upload_onetime_prekeys" }

func (c UploadOneTimePreKeysCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" || len(c.Keys) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	for _, k := range c.Keys {
		if len(k.PublicKey) == 0 {
			return sentinal_errors.ErrInvalidInput
		}
	}
	return nil
}

func (c UploadOneTimePreKeysCommand) IdempotencyKey() string { return "" }

func (c UploadOneTimePreKeysCommand) ActorID() uuid.UUID { return c.UserID }

// ConsumePreKeyCommand consumes a one-time pre-key
type ConsumePreKeyCommand struct {
	TargetUserID        uuid.UUID
	TargetDeviceID      string
	ConsumerID          uuid.UUID
	IdempotencyKeyValue string
}

func (ConsumePreKeyCommand) CommandType() string { return "encryption.consume_prekey" }

func (c ConsumePreKeyCommand) Validate() error {
	if c.TargetUserID == uuid.Nil || c.TargetDeviceID == "" || c.ConsumerID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ConsumePreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ConsumePreKeyCommand) ActorID() uuid.UUID { return c.ConsumerID }

// RotateSignedPreKeyCommand rotates the signed pre-key
type RotateSignedPreKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            string
	NewKeyID            int
	NewPublicKey        []byte
	NewSignature        []byte
	IdempotencyKeyValue string
}

func (RotateSignedPreKeyCommand) CommandType() string { return "encryption.rotate_signed_prekey" }

func (c RotateSignedPreKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" || len(c.NewPublicKey) == 0 || len(c.NewSignature) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RotateSignedPreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RotateSignedPreKeyCommand) ActorID() uuid.UUID { return c.UserID }

// CreateSessionCommand creates an encrypted session
type CreateSessionCommand struct {
	LocalUserID         uuid.UUID
	LocalDeviceID       string
	RemoteUserID        uuid.UUID
	RemoteDeviceID      string
	EncryptedState      []byte
	IdempotencyKeyValue string
}

func (CreateSessionCommand) CommandType() string { return "encryption.create_session" }

func (c CreateSessionCommand) Validate() error {
	if c.LocalUserID == uuid.Nil || c.LocalDeviceID == "" ||
		c.RemoteUserID == uuid.Nil || c.RemoteDeviceID == "" ||
		len(c.EncryptedState) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateSessionCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreateSessionCommand) ActorID() uuid.UUID { return c.LocalUserID }

// UpdateSessionCommand updates an encrypted session
type UpdateSessionCommand struct {
	SessionID           uuid.UUID
	LocalUserID         uuid.UUID
	EncryptedState      []byte
	IdempotencyKeyValue string
}

func (UpdateSessionCommand) CommandType() string { return "encryption.update_session" }

func (c UpdateSessionCommand) Validate() error {
	if c.SessionID == uuid.Nil || c.LocalUserID == uuid.Nil || len(c.EncryptedState) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateSessionCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateSessionCommand) ActorID() uuid.UUID { return c.LocalUserID }

// UpsertKeyBundleCommand upserts a key bundle
type UpsertKeyBundleCommand struct {
	UserID                uuid.UUID
	DeviceID              string
	IdentityKey           []byte
	SignedPreKeyID        int
	SignedPreKey          []byte
	SignedPreKeySignature []byte
	OneTimePreKeyID       *int
	OneTimePreKey         []byte
	IdempotencyKeyValue   string
}

func (UpsertKeyBundleCommand) CommandType() string { return "encryption.upsert_key_bundle" }

func (c UpsertKeyBundleCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == "" || len(c.IdentityKey) == 0 ||
		len(c.SignedPreKey) == 0 || len(c.SignedPreKeySignature) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpsertKeyBundleCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpsertKeyBundleCommand) ActorID() uuid.UUID { return c.UserID }
