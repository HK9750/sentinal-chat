package httpdto

import (
	"encoding/base64"
	"sentinal-chat/internal/domain/encryption"
	"time"
)

// UploadIdentityKeyRequest is used for POST /encryption/identity-keys
type UploadIdentityKeyRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	DeviceID  string `json:"device_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

// UploadIdentityKeyResponse is returned after uploading an identity key
type UploadIdentityKeyResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
	CreatedAt string `json:"created_at"`
}

// GetIdentityKeyRequest holds query parameters for getting identity key
type GetIdentityKeyRequest struct {
	UserID   string `form:"user_id" binding:"required"`
	DeviceID string `form:"device_id" binding:"required"`
}

// UploadSignedPreKeyRequest is used for POST /encryption/signed-prekeys
type UploadSignedPreKeyRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	DeviceID  string `json:"device_id" binding:"required"`
	KeyID     int    `json:"key_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

// GetSignedPreKeyRequest holds query parameters for getting signed prekey
type GetSignedPreKeyRequest struct {
	UserID   string `form:"user_id" binding:"required"`
	DeviceID string `form:"device_id" binding:"required"`
	KeyID    int    `form:"key_id"`
}

// RotateSignedPreKeyRequest is used for POST /encryption/signed-prekeys/rotate
type RotateSignedPreKeyRequest struct {
	UserID   string                `json:"user_id" binding:"required"`
	DeviceID string                `json:"device_id" binding:"required"`
	Key      SignedPreKeyUploadDTO `json:"key" binding:"required"`
}

// SignedPreKeyUploadDTO represents signed prekey data for upload
type SignedPreKeyUploadDTO struct {
	KeyID     int    `json:"key_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

// UploadOneTimePreKeysRequest is used for POST /encryption/one-time-prekeys
type UploadOneTimePreKeysRequest struct {
	Keys []OneTimePreKeyUploadDTO `json:"keys" binding:"required"`
}

// OneTimePreKeyUploadDTO represents one-time prekey data for upload
type OneTimePreKeyUploadDTO struct {
	UserID    string `json:"user_id" binding:"required"`
	DeviceID  string `json:"device_id" binding:"required"`
	KeyID     int    `json:"key_id" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

// UploadedKeysCountResponse is returned after uploading keys
type UploadedKeysCountResponse struct {
	Uploaded int `json:"uploaded"`
}

// ConsumeOneTimePreKeyRequest holds query parameters for consuming prekey
type ConsumeOneTimePreKeyRequest struct {
	UserID           string `form:"user_id" binding:"required"`
	DeviceID         string `form:"device_id" binding:"required"`
	ConsumedBy       string `form:"consumed_by" binding:"required"`
	ConsumedDeviceID string `form:"consumed_device_id" binding:"required"`
}

// PreKeyCountRequest holds query parameters for getting prekey count
type PreKeyCountRequest struct {
	UserID   string `form:"user_id" binding:"required"`
	DeviceID string `form:"device_id" binding:"required"`
}

// PreKeyCountResponse is returned when getting prekey count
type PreKeyCountResponse struct {
	Count int `json:"count"`
}

// GetKeyBundleRequest holds query parameters for getting key bundle
type GetKeyBundleRequest struct {
	UserID           string `form:"user_id" binding:"required"`
	DeviceID         string `form:"device_id" binding:"required"`
	ConsumerDeviceID string `form:"consumer_device_id" binding:"required"`
}

// HasActiveKeysRequest holds query parameters for checking active keys
type HasActiveKeysRequest struct {
	UserID   string `form:"user_id" binding:"required"`
	DeviceID string `form:"device_id" binding:"required"`
}

// HasActiveKeysResponse is returned when checking for active keys
type HasActiveKeysResponse struct {
	HasActiveKeys bool `json:"has_active_keys"`
}

// IdentityKeyDTO represents an identity key in API responses
type IdentityKeyDTO struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key,omitempty"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// SignedPreKeyDTO represents a signed prekey in API responses
type SignedPreKeyDTO struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	KeyID     int    `json:"key_id"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
	CreatedAt string `json:"created_at"`
	IsActive  bool   `json:"is_active"`
}

// OneTimePreKeyDTO represents a one-time prekey in API responses
type OneTimePreKeyDTO struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	DeviceID           string `json:"device_id"`
	KeyID              int    `json:"key_id"`
	PublicKey          string `json:"public_key,omitempty"`
	UploadedAt         string `json:"uploaded_at"`
	ConsumedAt         string `json:"consumed_at,omitempty"`
	ConsumedBy         string `json:"consumed_by,omitempty"`
	ConsumedByDeviceID string `json:"consumed_by_device_id,omitempty"`
}

// KeyBundleDTO represents a key bundle in API responses
type KeyBundleDTO struct {
	IdentityKey   IdentityKeyDTO   `json:"identity_key"`
	SignedPreKey  SignedPreKeyDTO  `json:"signed_pre_key"`
	OneTimePreKey OneTimePreKeyDTO `json:"one_time_pre_key,omitempty"`
}

// FromIdentityKey converts a domain identity key to IdentityKeyDTO
func FromIdentityKey(k encryption.IdentityKey) IdentityKeyDTO {
	return IdentityKeyDTO{
		ID:        k.ID.String(),
		UserID:    k.UserID.String(),
		DeviceID:  k.DeviceID.String(),
		PublicKey: base64.StdEncoding.EncodeToString(k.PublicKey),
		IsActive:  k.IsActive,
		CreatedAt: k.CreatedAt.Format(time.RFC3339),
	}
}

// FromSignedPreKey converts a domain signed prekey to SignedPreKeyDTO
func FromSignedPreKey(k encryption.SignedPreKey) SignedPreKeyDTO {
	return SignedPreKeyDTO{
		ID:        k.ID.String(),
		UserID:    k.UserID.String(),
		DeviceID:  k.DeviceID.String(),
		KeyID:     k.KeyID,
		PublicKey: base64.StdEncoding.EncodeToString(k.PublicKey),
		Signature: base64.StdEncoding.EncodeToString(k.Signature),
		CreatedAt: k.CreatedAt.Format(time.RFC3339),
		IsActive:  k.IsActive,
	}
}

// FromOneTimePreKey converts a domain one-time prekey to OneTimePreKeyDTO
func FromOneTimePreKey(k encryption.OneTimePreKey) OneTimePreKeyDTO {
	dto := OneTimePreKeyDTO{
		ID:         k.ID.String(),
		UserID:     k.UserID.String(),
		DeviceID:   k.DeviceID.String(),
		KeyID:      k.KeyID,
		PublicKey:  base64.StdEncoding.EncodeToString(k.PublicKey),
		UploadedAt: k.UploadedAt.Format(time.RFC3339),
	}
	if k.ConsumedAt.Valid {
		dto.ConsumedAt = k.ConsumedAt.Time.Format(time.RFC3339)
	}
	if k.ConsumedBy.Valid {
		dto.ConsumedBy = k.ConsumedBy.UUID.String()
	}
	if k.ConsumedByDeviceID.Valid {
		dto.ConsumedByDeviceID = k.ConsumedByDeviceID.UUID.String()
	}
	return dto
}
