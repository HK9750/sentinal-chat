package services

import (
	"context"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

type EncryptionService struct {
	repo      repository.EncryptionRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewEncryptionService(repo repository.EncryptionRepository, eventRepo repository.EventRepository, bus *commands.Bus) *EncryptionService {
	return &EncryptionService{repo: repo, eventRepo: eventRepo, bus: bus}
}

func (s *EncryptionService) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	if err := s.repo.CreateIdentityKey(ctx, k); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "identity_key.created", k.ID, k)
}

func (s *EncryptionService) GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.IdentityKey, error) {
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
	if err := s.repo.CreateSignedPreKey(ctx, k); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "signed_prekey.created", k.ID, k)
}

func (s *EncryptionService) GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string, keyID int) (encryption.SignedPreKey, error) {
	return s.repo.GetSignedPreKey(ctx, userID, deviceID, keyID)
}

func (s *EncryptionService) GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.SignedPreKey, error) {
	return s.repo.GetActiveSignedPreKey(ctx, userID, deviceID)
}

func (s *EncryptionService) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string, newKey *encryption.SignedPreKey) error {
	return s.repo.RotateSignedPreKey(ctx, userID, deviceID, newKey)
}

func (s *EncryptionService) DeactivateSignedPreKey(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeactivateSignedPreKey(ctx, id)
}

func (s *EncryptionService) UploadOneTimePreKeys(ctx context.Context, keys []encryption.OneTimePreKey) error {
	if err := s.repo.UploadOneTimePreKeys(ctx, keys); err != nil {
		return err
	}
	if len(keys) > 0 {
		return createOutboxEvent(ctx, s.eventRepo, "encryption", "onetime_prekeys.uploaded", keys[0].ID, map[string]any{"count": len(keys)})
	}
	return nil
}

func (s *EncryptionService) ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID string, consumedBy uuid.UUID) (encryption.OneTimePreKey, error) {
	return s.repo.ConsumeOneTimePreKey(ctx, userID, deviceID, consumedBy)
}

func (s *EncryptionService) GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID string) (int64, error) {
	return s.repo.GetAvailablePreKeyCount(ctx, userID, deviceID)
}

func (s *EncryptionService) DeleteConsumedPreKeys(ctx context.Context, olderThan time.Time) (int64, error) {
	return s.repo.DeleteConsumedPreKeys(ctx, olderThan)
}

func (s *EncryptionService) CreateSession(ctx context.Context, sess *encryption.EncryptedSession) error {
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "session.created", sess.ID, sess)
}

func (s *EncryptionService) GetSession(ctx context.Context, localUserID uuid.UUID, localDeviceID string, remoteUserID uuid.UUID, remoteDeviceID string) (encryption.EncryptedSession, error) {
	return s.repo.GetSession(ctx, localUserID, localDeviceID, remoteUserID, remoteDeviceID)
}

func (s *EncryptionService) UpdateSession(ctx context.Context, sess encryption.EncryptedSession) error {
	if err := s.repo.UpdateSession(ctx, sess); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "session.updated", sess.ID, sess)
}

func (s *EncryptionService) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSession(ctx, id)
}

func (s *EncryptionService) GetUserSessions(ctx context.Context, userID uuid.UUID, deviceID string) ([]encryption.EncryptedSession, error) {
	return s.repo.GetUserSessions(ctx, userID, deviceID)
}

func (s *EncryptionService) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID, deviceID string) error {
	return s.repo.DeleteAllUserSessions(ctx, userID, deviceID)
}

func (s *EncryptionService) UpsertKeyBundle(ctx context.Context, b *encryption.KeyBundle) error {
	if err := s.repo.UpsertKeyBundle(ctx, b); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "key_bundle.upserted", uuid.New(), b)
}

func (s *EncryptionService) GetKeyBundle(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.KeyBundle, error) {
	return s.repo.GetKeyBundle(ctx, userID, deviceID)
}

func (s *EncryptionService) GetUserKeyBundles(ctx context.Context, userID uuid.UUID) ([]encryption.KeyBundle, error) {
	return s.repo.GetUserKeyBundles(ctx, userID)
}

func (s *EncryptionService) DeleteKeyBundle(ctx context.Context, userID uuid.UUID, deviceID string) error {
	return s.repo.DeleteKeyBundle(ctx, userID, deviceID)
}

func (s *EncryptionService) HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID string) (bool, error) {
	return s.repo.HasActiveKeys(ctx, userID, deviceID)
}
