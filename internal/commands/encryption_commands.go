package commands

import (
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// RegisterIdentityKeyCommand registers an identity key
type RegisterIdentityKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            uuid.UUID
	PublicKey           []byte
	IdempotencyKeyValue string
}

func (RegisterIdentityKeyCommand) CommandType() string { return "encryption.register_identity_key" }

func (c RegisterIdentityKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == uuid.Nil || len(c.PublicKey) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RegisterIdentityKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RegisterIdentityKeyCommand) ActorID() uuid.UUID { return c.UserID }

// UploadSignedPreKeyCommand uploads a signed pre-key
type UploadSignedPreKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            uuid.UUID
	KeyID               int
	PublicKey           []byte
	Signature           []byte
	IdempotencyKeyValue string
}

func (UploadSignedPreKeyCommand) CommandType() string { return "encryption.upload_signed_prekey" }

func (c UploadSignedPreKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == uuid.Nil || len(c.PublicKey) == 0 || len(c.Signature) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UploadSignedPreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UploadSignedPreKeyCommand) ActorID() uuid.UUID { return c.UserID }

// UploadOneTimePreKeysCommand uploads one-time pre-keys
type UploadOneTimePreKeysCommand struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	Keys     []OneTimePreKeyInput
}

type OneTimePreKeyInput struct {
	KeyID     int
	PublicKey []byte
}

func (UploadOneTimePreKeysCommand) CommandType() string { return "encryption.upload_onetime_prekeys" }

func (c UploadOneTimePreKeysCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == uuid.Nil || len(c.Keys) == 0 {
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
	TargetDeviceID      uuid.UUID
	ConsumerID          uuid.UUID
	ConsumerDeviceID    uuid.UUID
	IdempotencyKeyValue string
}

func (ConsumePreKeyCommand) CommandType() string { return "encryption.consume_prekey" }

func (c ConsumePreKeyCommand) Validate() error {
	if c.TargetUserID == uuid.Nil || c.TargetDeviceID == uuid.Nil || c.ConsumerID == uuid.Nil || c.ConsumerDeviceID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c ConsumePreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c ConsumePreKeyCommand) ActorID() uuid.UUID { return c.ConsumerID }

// RotateSignedPreKeyCommand rotates the signed pre-key
type RotateSignedPreKeyCommand struct {
	UserID              uuid.UUID
	DeviceID            uuid.UUID
	NewKeyID            int
	NewPublicKey        []byte
	NewSignature        []byte
	IdempotencyKeyValue string
}

func (RotateSignedPreKeyCommand) CommandType() string { return "encryption.rotate_signed_prekey" }

func (c RotateSignedPreKeyCommand) Validate() error {
	if c.UserID == uuid.Nil || c.DeviceID == uuid.Nil || len(c.NewPublicKey) == 0 || len(c.NewSignature) == 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c RotateSignedPreKeyCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c RotateSignedPreKeyCommand) ActorID() uuid.UUID { return c.UserID }

// CreateSessionCommand and UpdateSessionCommand removed; sessions are handled client-side.

// UpsertKeyBundleCommand removed; server derives bundles from stored keys.
