package httpdto

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
	UserID     string `form:"user_id" binding:"required"`
	DeviceID   string `form:"device_id" binding:"required"`
	ConsumedBy string `form:"consumed_by" binding:"required"`
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

// CreateSessionRequest is used for POST /encryption/sessions
type CreateSessionRequest struct {
	LocalUserID    string `json:"local_user_id" binding:"required"`
	LocalDeviceID  string `json:"local_device_id" binding:"required"`
	RemoteUserID   string `json:"remote_user_id" binding:"required"`
	RemoteDeviceID string `json:"remote_device_id" binding:"required"`
	SessionData    string `json:"session_data" binding:"required"`
}

// GetSessionRequest holds query parameters for getting session
type GetSessionRequest struct {
	LocalUserID    string `form:"local_user_id" binding:"required"`
	LocalDeviceID  string `form:"local_device_id" binding:"required"`
	RemoteUserID   string `form:"remote_user_id" binding:"required"`
	RemoteDeviceID string `form:"remote_device_id" binding:"required"`
}

// EncryptedSessionDTO represents an encrypted session in API responses
type EncryptedSessionDTO struct {
	ID             string `json:"id"`
	LocalUserID    string `json:"local_user_id"`
	LocalDeviceID  string `json:"local_device_id"`
	RemoteUserID   string `json:"remote_user_id"`
	RemoteDeviceID string `json:"remote_device_id"`
	SessionData    string `json:"session_data"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// UpsertKeyBundleRequest is used for POST /encryption/key-bundles
type UpsertKeyBundleRequest struct {
	UserID          string `json:"user_id" binding:"required"`
	DeviceID        string `json:"device_id" binding:"required"`
	IdentityKey     string `json:"identity_key" binding:"required"`
	SignedPreKey    string `json:"signed_prekey" binding:"required"`
	SignedPreKeySig string `json:"signed_prekey_sig" binding:"required"`
	OneTimePreKey   string `json:"one_time_prekey,omitempty"`
}

// GetKeyBundleRequest holds query parameters for getting key bundle
type GetKeyBundleRequest struct {
	UserID   string `form:"user_id" binding:"required"`
	DeviceID string `form:"device_id" binding:"required"`
}

// KeyBundleDTO represents a key bundle in API responses
type KeyBundleDTO struct {
	UserID          string `json:"user_id"`
	DeviceID        string `json:"device_id"`
	IdentityKey     string `json:"identity_key"`
	SignedPreKey    string `json:"signed_prekey"`
	SignedPreKeySig string `json:"signed_prekey_sig"`
	OneTimePreKey   string `json:"one_time_prekey,omitempty"`
	UpdatedAt       string `json:"updated_at"`
}

// KeyBundlesResponse is returned when listing key bundles
type KeyBundlesResponse struct {
	Bundles []KeyBundleDTO `json:"bundles"`
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
