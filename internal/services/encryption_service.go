package services

import (
	"context"
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
		owned, err := s.repo.IsDeviceOwnedByUser(ctx, c.UserID, c.DeviceID)
		if err != nil {
			return commands.Result{}, err
		}
		if !owned {
			return commands.Result{}, sentinal_errors.ErrForbidden
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
		owned, err := s.repo.IsDeviceOwnedByUser(ctx, c.UserID, c.DeviceID)
		if err != nil {
			return commands.Result{}, err
		}
		if !owned {
			return commands.Result{}, sentinal_errors.ErrForbidden
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
		owned, err := s.repo.IsDeviceOwnedByUser(ctx, c.UserID, c.DeviceID)
		if err != nil {
			return commands.Result{}, err
		}
		if !owned {
			return commands.Result{}, sentinal_errors.ErrForbidden
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
		if owned, err := s.repo.IsDeviceOwnedByUser(ctx, c.ConsumerID, c.ConsumerDeviceID); err != nil {
			return commands.Result{}, err
		} else if !owned {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		key, err := s.ConsumeOneTimePreKey(ctx, c.TargetUserID, c.TargetDeviceID, c.ConsumerID, c.ConsumerDeviceID)
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
		owned, err := s.repo.IsDeviceOwnedByUser(ctx, c.UserID, c.DeviceID)
		if err != nil {
			return commands.Result{}, err
		}
		if !owned {
			return commands.Result{}, sentinal_errors.ErrForbidden
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

}

func (s *EncryptionService) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	if err := s.repo.CreateIdentityKey(ctx, k); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "identity_key.created", k.ID, k)
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
	if err := s.repo.CreateSignedPreKey(ctx, k); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "encryption", "signed_prekey.created", k.ID, k)
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
	if err := s.repo.UploadOneTimePreKeys(ctx, keys); err != nil {
		return err
	}
	if len(keys) > 0 {
		return createOutboxEvent(ctx, s.eventRepo, "encryption", "onetime_prekeys.uploaded", keys[0].ID, map[string]any{"count": len(keys)})
	}
	return nil
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

	if err := createOutboxEvent(ctx, s.eventRepo, "encryption", "key_bundle.fetched", uuid.New(), map[string]any{
		"user_id":             userID,
		"device_id":           deviceID,
		"consumer_id":         consumerID,
		"consumer_device_id":  consumerDeviceID,
		"has_one_time_prekey": prekeyErr == nil,
	}); err != nil {
		return KeyBundle{}, err
	}

	return bundle, nil
}
