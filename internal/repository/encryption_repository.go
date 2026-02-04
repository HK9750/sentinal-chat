package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/encryption"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresEncryptionRepository struct {
	db *gorm.DB
}

func NewEncryptionRepository(db *gorm.DB) EncryptionRepository {
	return &PostgresEncryptionRepository{db: db}
}

func (r *PostgresEncryptionRepository) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	res := r.db.WithContext(ctx).Create(k)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.IdentityKey, error) {
	var k encryption.IdentityKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ? AND is_active = true", userID, deviceID).
		First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return encryption.IdentityKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.IdentityKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) GetUserIdentityKeys(ctx context.Context, userID uuid.UUID) ([]encryption.IdentityKey, error) {
	var keys []encryption.IdentityKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Find(&keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *PostgresEncryptionRepository) DeactivateIdentityKey(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&encryption.IdentityKey{}).
		Where("id = ?", id).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) DeleteIdentityKey(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&encryption.IdentityKey{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) CreateSignedPreKey(ctx context.Context, k *encryption.SignedPreKey) error {
	res := r.db.WithContext(ctx).Create(k)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string, keyID int) (encryption.SignedPreKey, error) {
	var k encryption.SignedPreKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ? AND key_id = ?", userID, deviceID, keyID).
		First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return encryption.SignedPreKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.SignedPreKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.SignedPreKey, error) {
	var k encryption.SignedPreKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ? AND is_active = true", userID, deviceID).
		Order("created_at DESC").
		First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return encryption.SignedPreKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.SignedPreKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID string, newKey *encryption.SignedPreKey) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Deactivate old keys
		if err := tx.Model(&encryption.SignedPreKey{}).
			Where("user_id = ? AND device_id = ? AND is_active = true", userID, deviceID).
			Update("is_active", false).Error; err != nil {
			return err
		}

		// Create new key
		return tx.Create(newKey).Error
	})
}

func (r *PostgresEncryptionRepository) DeactivateSignedPreKey(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&encryption.SignedPreKey{}).
		Where("id = ?", id).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) UploadOneTimePreKeys(ctx context.Context, keys []encryption.OneTimePreKey) error {
	if len(keys) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Create(&keys)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID string, consumedBy uuid.UUID) (encryption.OneTimePreKey, error) {
	var key encryption.OneTimePreKey

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Find an unconsumed key
		err := tx.Where("user_id = ? AND device_id = ? AND consumed_at IS NULL", userID, deviceID).
			Order("uploaded_at ASC").
			First(&key).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		// Mark as consumed
		now := time.Now()
		return tx.Model(&encryption.OneTimePreKey{}).
			Where("id = ?", key.ID).
			Updates(map[string]interface{}{
				"consumed_at": now,
				"consumed_by": consumedBy,
			}).Error
	})

	if err != nil {
		return encryption.OneTimePreKey{}, err
	}
	return key, nil
}

func (r *PostgresEncryptionRepository) GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&encryption.OneTimePreKey{}).
		Where("user_id = ? AND device_id = ? AND consumed_at IS NULL", userID, deviceID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresEncryptionRepository) DeleteConsumedPreKeys(ctx context.Context, olderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Delete(&encryption.OneTimePreKey{}, "consumed_at IS NOT NULL AND consumed_at < ?", olderThan)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *PostgresEncryptionRepository) CreateSession(ctx context.Context, s *encryption.EncryptedSession) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetSession(ctx context.Context, localUserID uuid.UUID, localDeviceID string, remoteUserID uuid.UUID, remoteDeviceID string) (encryption.EncryptedSession, error) {
	var s encryption.EncryptedSession
	err := r.db.WithContext(ctx).
		Where("local_user_id = ? AND local_device_id = ? AND remote_user_id = ? AND remote_device_id = ?",
			localUserID, localDeviceID, remoteUserID, remoteDeviceID).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return encryption.EncryptedSession{}, sentinal_errors.ErrNotFound
		}
		return encryption.EncryptedSession{}, err
	}
	return s, nil
}

func (r *PostgresEncryptionRepository) UpdateSession(ctx context.Context, s encryption.EncryptedSession) error {
	s.UpdatedAt = time.Now()
	res := r.db.WithContext(ctx).Save(&s)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&encryption.EncryptedSession{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetUserSessions(ctx context.Context, userID uuid.UUID, deviceID string) ([]encryption.EncryptedSession, error) {
	var sessions []encryption.EncryptedSession
	err := r.db.WithContext(ctx).
		Where("local_user_id = ? AND local_device_id = ?", userID, deviceID).
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresEncryptionRepository) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID, deviceID string) error {
	res := r.db.WithContext(ctx).
		Delete(&encryption.EncryptedSession{}, "local_user_id = ? AND local_device_id = ?", userID, deviceID)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) UpsertKeyBundle(ctx context.Context, b *encryption.KeyBundle) error {
	b.UpdatedAt = time.Now()
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
			UpdateAll: true,
		}).
		Create(b)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetKeyBundle(ctx context.Context, userID uuid.UUID, deviceID string) (encryption.KeyBundle, error) {
	var b encryption.KeyBundle
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return encryption.KeyBundle{}, sentinal_errors.ErrNotFound
		}
		return encryption.KeyBundle{}, err
	}
	return b, nil
}

func (r *PostgresEncryptionRepository) GetUserKeyBundles(ctx context.Context, userID uuid.UUID) ([]encryption.KeyBundle, error) {
	var bundles []encryption.KeyBundle
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&bundles).Error
	if err != nil {
		return nil, err
	}
	return bundles, nil
}

func (r *PostgresEncryptionRepository) DeleteKeyBundle(ctx context.Context, userID uuid.UUID, deviceID string) error {
	res := r.db.WithContext(ctx).
		Delete(&encryption.KeyBundle{}, "user_id = ? AND device_id = ?", userID, deviceID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresEncryptionRepository) HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&encryption.IdentityKey{}).
		Where("user_id = ? AND device_id = ? AND is_active = true", userID, deviceID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
