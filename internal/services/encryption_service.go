package services

import (
	"context"
	"time"

	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type EncryptionService struct {
	repo repository.EncryptionRepository
}

type KeyBundle struct {
	UserID                uuid.UUID `json:"user_id"`
	DeviceID              uuid.UUID `json:"device_id"`
	IdentityKey           []byte    `json:"identity_key"`
	SignedPreKeyID        int       `json:"signed_prekey_id"`
	SignedPreKey          []byte    `json:"signed_prekey"`
	SignedPreKeySignature []byte    `json:"signed_prekey_signature"`
	OneTimePreKeyID       *int      `json:"one_time_prekey_id,omitempty"`
	OneTimePreKey         []byte    `json:"one_time_prekey,omitempty"`
}

func NewEncryptionService(repo repository.EncryptionRepository) *EncryptionService {
	return &EncryptionService{repo: repo}
}

func (s *EncryptionService) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	return s.repo.CreateIdentityKey(ctx, k)
}

func (s *EncryptionService) GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.IdentityKey, error) {
	return s.repo.GetIdentityKey(ctx, userID, deviceID)
}

func (s *EncryptionService) GetUserIdentityKeys(ctx context.Context, userID uuid.UUID) ([]encryption.IdentityKey, error) {
	return s.repo.GetUserIdentityKeys(ctx, userID)
}

func (s *EncryptionService) DeactivateIdentityKey(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeactivateIdentityKey(ctx, id)
}

func (s *EncryptionService) DeleteIdentityKey(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteIdentityKey(ctx, id)
}

func (s *EncryptionService) CreateSignedPreKey(ctx context.Context, k *encryption.SignedPreKey) error {
	return s.repo.CreateSignedPreKey(ctx, k)
}

func (s *EncryptionService) GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, keyID int) (encryption.SignedPreKey, error) {
	return s.repo.GetSignedPreKey(ctx, userID, deviceID, keyID)
}

func (s *EncryptionService) GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.SignedPreKey, error) {
	return s.repo.GetActiveSignedPreKey(ctx, userID, deviceID)
}

func (s *EncryptionService) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, newKey *encryption.SignedPreKey) error {
	return s.repo.RotateSignedPreKey(ctx, userID, deviceID, newKey)
}

func (s *EncryptionService) DeactivateSignedPreKey(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeactivateSignedPreKey(ctx, id)
}

func (s *EncryptionService) UploadOneTimePreKeys(ctx context.Context, keys []encryption.OneTimePreKey) error {
	return s.repo.UploadOneTimePreKeys(ctx, keys)
}

func (s *EncryptionService) ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, consumedBy uuid.UUID, consumedByDeviceID uuid.UUID) (encryption.OneTimePreKey, error) {
	return s.repo.ConsumeOneTimePreKey(ctx, userID, deviceID, consumedBy, consumedByDeviceID)
}

func (s *EncryptionService) GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (int64, error) {
	return s.repo.GetAvailablePreKeyCount(ctx, userID, deviceID)
}

func (s *EncryptionService) DeleteConsumedPreKeys(ctx context.Context, olderThan time.Time) (int64, error) {
	return s.repo.DeleteConsumedPreKeys(ctx, olderThan)
}

func (s *EncryptionService) HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error) {
	return s.repo.HasActiveKeys(ctx, userID, deviceID)
}

func (s *EncryptionService) GetKeyBundle(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, consumerID uuid.UUID, consumerDeviceID uuid.UUID) (KeyBundle, error) {
	if owned, err := s.repo.IsDeviceOwnedByUser(ctx, consumerID, consumerDeviceID); err != nil {
		return KeyBundle{}, err
	} else if !owned {
		return KeyBundle{}, sentinal_errors.ErrForbidden
	}
	if userID == consumerID {
		return KeyBundle{}, sentinal_errors.ErrInvalidInput
	}
	identity, err := s.repo.GetIdentityKey(ctx, userID, deviceID)
	if err != nil {
		return KeyBundle{}, err
	}

	signed, err := s.repo.GetActiveSignedPreKey(ctx, userID, deviceID)
	if err != nil {
		return KeyBundle{}, err
	}

	var bundle KeyBundle
	bundle.UserID = userID
	bundle.DeviceID = deviceID
	bundle.IdentityKey = identity.PublicKey
	bundle.SignedPreKeyID = signed.KeyID
	bundle.SignedPreKey = signed.PublicKey
	bundle.SignedPreKeySignature = signed.Signature

	prekey, prekeyErr := s.repo.ConsumeOneTimePreKey(ctx, userID, deviceID, consumerID, consumerDeviceID)
	if prekeyErr == nil {
		bundle.OneTimePreKeyID = &prekey.KeyID
		bundle.OneTimePreKey = prekey.PublicKey
	}
	if prekeyErr != nil && prekeyErr != sentinal_errors.ErrNotFound {
		return KeyBundle{}, prekeyErr
	}

	return bundle, nil
}
