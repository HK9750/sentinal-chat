package repository

import (
	"context"
	"errors"
	"time"

	"sentinal-chat/internal/domain/encryption"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresEncryptionRepository struct {
	db *gorm.DB
}

func NewEncryptionRepository(db *gorm.DB) EncryptionRepository {
	return &PostgresEncryptionRepository{db: db}
}

func (r *PostgresEncryptionRepository) IsDeviceOwnedByUser(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("devices").
		Where("id = ? AND user_id = ? AND is_active = true", deviceID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresEncryptionRepository) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove any existing identity key for this user+device to handle retries
		tx.Where("user_id = ? AND device_id = ?", k.UserID, k.DeviceID).
			Delete(&encryption.IdentityKey{})

		// Insert the new key
		if err := tx.Create(k).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *PostgresEncryptionRepository) GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.IdentityKey, error) {
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

func (r *PostgresEncryptionRepository) GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, keyID int) (encryption.SignedPreKey, error) {
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

func (r *PostgresEncryptionRepository) GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.SignedPreKey, error) {
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

func (r *PostgresEncryptionRepository) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, newKey *encryption.SignedPreKey) error {
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

func (r *PostgresEncryptionRepository) ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, consumedBy uuid.UUID, consumedByDeviceID uuid.UUID) (encryption.OneTimePreKey, error) {
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
				"consumed_at":           now,
				"consumed_by":           consumedBy,
				"consumed_by_device_id": consumedByDeviceID,
			}).Error
	})

	if err != nil {
		return encryption.OneTimePreKey{}, err
	}
	return key, nil
}

func (r *PostgresEncryptionRepository) GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (int64, error) {
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

func (r *PostgresEncryptionRepository) HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error) {
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
