package services

import (
	"context"
	"database/sql"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/encryption"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type EncryptionService struct {
	repo      repository.EncryptionRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewEncryptionService(repo repository.EncryptionRepository, eventRepo repository.EventRepository, bus *commands.Bus) *EncryptionService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &EncryptionService{repo: repo, eventRepo: eventRepo, bus: bus}
	svc.RegisterHandlers(bus)
	return svc
}

func (s *EncryptionService) RegisterHandlers(bus *commands.Bus) {
	if bus == nil {
		return
	}

	// encryption.register_identity_key - Register an identity key
	bus.Register("encryption.register_identity_key", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RegisterIdentityKeyCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		k := &encryption.IdentityKey{
			ID:        uuid.New(),
			UserID:    c.UserID,
			DeviceID:  c.DeviceID,
			PublicKey: c.PublicKey,
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		if err := s.CreateIdentityKey(ctx, k); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: k.ID.String(), Payload: k}, nil
	}))

	// encryption.upload_signed_prekey - Upload a signed pre-key
	bus.Register("encryption.upload_signed_prekey", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UploadSignedPreKeyCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		k := &encryption.SignedPreKey{
			ID:        uuid.New(),
			UserID:    c.UserID,
			DeviceID:  c.DeviceID,
			KeyID:     c.KeyID,
			PublicKey: c.PublicKey,
			Signature: c.Signature,
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		if err := s.CreateSignedPreKey(ctx, k); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: k.ID.String(), Payload: k}, nil
	}))

	// encryption.upload_onetime_prekeys - Upload one-time pre-keys
	bus.Register("encryption.upload_onetime_prekeys", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UploadOneTimePreKeysCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		keys := make([]encryption.OneTimePreKey, 0, len(c.Keys))
		for _, k := range c.Keys {
			keys = append(keys, encryption.OneTimePreKey{
				ID:         uuid.New(),
				UserID:     c.UserID,
				DeviceID:   c.DeviceID,
				KeyID:      k.KeyID,
				PublicKey:  k.PublicKey,
				UploadedAt: time.Now(),
			})
		}
		if err := s.UploadOneTimePreKeys(ctx, keys); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.UserID.String(), Payload: map[string]int{"count": len(keys)}}, nil
	}))

	// encryption.consume_prekey - Consume a one-time pre-key
	bus.Register("encryption.consume_prekey", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.ConsumePreKeyCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		key, err := s.ConsumeOneTimePreKey(ctx, c.TargetUserID, c.TargetDeviceID, c.ConsumerID)
		if err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: key.ID.String(), Payload: key}, nil
	}))

	// encryption.rotate_signed_prekey - Rotate the signed pre-key
	bus.Register("encryption.rotate_signed_prekey", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RotateSignedPreKeyCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		newKey := &encryption.SignedPreKey{
			ID:        uuid.New(),
			UserID:    c.UserID,
			DeviceID:  c.DeviceID,
			KeyID:     c.NewKeyID,
			PublicKey: c.NewPublicKey,
			Signature: c.NewSignature,
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		if err := s.RotateSignedPreKey(ctx, c.UserID, c.DeviceID, newKey); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: newKey.ID.String(), Payload: newKey}, nil
	}))

	// encryption.create_session - Create an encrypted session
	bus.Register("encryption.create_session", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.CreateSessionCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		sess := &encryption.EncryptedSession{
			ID:             uuid.New(),
			LocalUserID:    c.LocalUserID,
			LocalDeviceID:  c.LocalDeviceID,
			RemoteUserID:   c.RemoteUserID,
			RemoteDeviceID: c.RemoteDeviceID,
			EncryptedState: c.EncryptedState,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.CreateSession(ctx, sess); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: sess.ID.String(), Payload: sess}, nil
	}))

	// encryption.update_session - Update an encrypted session
	bus.Register("encryption.update_session", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdateSessionCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		sess := encryption.EncryptedSession{
			ID:             c.SessionID,
			EncryptedState: c.EncryptedState,
			UpdatedAt:      time.Now(),
		}
		if err := s.UpdateSession(ctx, sess); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.SessionID.String(), Payload: sess}, nil
	}))

	// encryption.upsert_key_bundle - Upsert a key bundle
	bus.Register("encryption.upsert_key_bundle", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpsertKeyBundleCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		bundle := &encryption.KeyBundle{
			UserID:                c.UserID,
			DeviceID:              c.DeviceID,
			IdentityKey:           c.IdentityKey,
			SignedPreKeyID:        c.SignedPreKeyID,
			SignedPreKey:          c.SignedPreKey,
			SignedPreKeySignature: c.SignedPreKeySignature,
			UpdatedAt:             time.Now(),
		}
		if c.OneTimePreKeyID != nil {
			bundle.OneTimePreKeyID = sql.NullInt32{Int32: int32(*c.OneTimePreKeyID), Valid: true}
			bundle.OneTimePreKey = c.OneTimePreKey
		}
		if err := s.UpsertKeyBundle(ctx, bundle); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.UserID.String(), Payload: bundle}, nil
	}))
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
