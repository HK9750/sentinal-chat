package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/encryption"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresEncryptionRepository struct {
	db DBTX
}

func NewEncryptionRepository(db DBTX) EncryptionRepository {
	return &PostgresEncryptionRepository{db: db}
}

func (r *PostgresEncryptionRepository) IsDeviceOwnedByUser(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE id = $1 AND user_id = $2 AND is_active = true", deviceID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresEncryptionRepository) CreateIdentityKey(ctx context.Context, k *encryption.IdentityKey) error {
	return WithTx(ctx, r.db, func(tx DBTX) error {
		_, _ = tx.ExecContext(ctx, "DELETE FROM identity_keys WHERE user_id = $1 AND device_id = $2", k.UserID, k.DeviceID)
		_, err := tx.ExecContext(ctx, `
            INSERT INTO identity_keys (id, user_id, device_id, public_key, is_active, created_at)
            VALUES ($1,$2,$3,$4,$5,$6)
        `, k.ID, k.UserID, k.DeviceID, k.PublicKey, k.IsActive, k.CreatedAt)
		return err
	})
}

func (r *PostgresEncryptionRepository) GetIdentityKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.IdentityKey, error) {
	var k encryption.IdentityKey
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, device_id, public_key, is_active, created_at
        FROM identity_keys WHERE user_id = $1 AND device_id = $2 AND is_active = true
    `, userID, deviceID).Scan(&k.ID, &k.UserID, &k.DeviceID, &k.PublicKey, &k.IsActive, &k.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return encryption.IdentityKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.IdentityKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) GetUserIdentityKeys(ctx context.Context, userID uuid.UUID) ([]encryption.IdentityKey, error) {
	var keys []encryption.IdentityKey
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, device_id, public_key, is_active, created_at
        FROM identity_keys WHERE user_id = $1 AND is_active = true
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k encryption.IdentityKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.DeviceID, &k.PublicKey, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *PostgresEncryptionRepository) DeactivateIdentityKey(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE identity_keys SET is_active = false WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresEncryptionRepository) DeleteIdentityKey(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM identity_keys WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresEncryptionRepository) CreateSignedPreKey(ctx context.Context, k *encryption.SignedPreKey) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO signed_prekeys (id, user_id, device_id, key_id, public_key, signature, created_at, is_active)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, k.ID, k.UserID, k.DeviceID, k.KeyID, k.PublicKey, k.Signature, k.CreatedAt, k.IsActive)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresEncryptionRepository) GetSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, keyID int) (encryption.SignedPreKey, error) {
	var k encryption.SignedPreKey
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, device_id, key_id, public_key, signature, created_at, is_active
        FROM signed_prekeys WHERE user_id = $1 AND device_id = $2 AND key_id = $3
    `, userID, deviceID, keyID).Scan(&k.ID, &k.UserID, &k.DeviceID, &k.KeyID, &k.PublicKey, &k.Signature, &k.CreatedAt, &k.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return encryption.SignedPreKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.SignedPreKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) GetActiveSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (encryption.SignedPreKey, error) {
	var k encryption.SignedPreKey
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, device_id, key_id, public_key, signature, created_at, is_active
        FROM signed_prekeys WHERE user_id = $1 AND device_id = $2 AND is_active = true
        ORDER BY created_at DESC LIMIT 1
    `, userID, deviceID).Scan(&k.ID, &k.UserID, &k.DeviceID, &k.KeyID, &k.PublicKey, &k.Signature, &k.CreatedAt, &k.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return encryption.SignedPreKey{}, sentinal_errors.ErrNotFound
		}
		return encryption.SignedPreKey{}, err
	}
	return k, nil
}

func (r *PostgresEncryptionRepository) RotateSignedPreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, newKey *encryption.SignedPreKey) error {
	return WithTx(ctx, r.db, func(tx DBTX) error {
		if _, err := tx.ExecContext(ctx, "UPDATE signed_prekeys SET is_active = false WHERE user_id = $1 AND device_id = $2 AND is_active = true", userID, deviceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
            INSERT INTO signed_prekeys (id, user_id, device_id, key_id, public_key, signature, created_at, is_active)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        `, newKey.ID, newKey.UserID, newKey.DeviceID, newKey.KeyID, newKey.PublicKey, newKey.Signature, newKey.CreatedAt, newKey.IsActive)
		return err
	})
}

func (r *PostgresEncryptionRepository) DeactivateSignedPreKey(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE signed_prekeys SET is_active = false WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresEncryptionRepository) UploadOneTimePreKeys(ctx context.Context, keys []encryption.OneTimePreKey) error {
	if len(keys) == 0 {
		return nil
	}
	return WithTx(ctx, r.db, func(tx DBTX) error {
		for _, k := range keys {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO onetime_prekeys (id, user_id, device_id, key_id, public_key, uploaded_at, consumed_at, consumed_by, consumed_by_device_id)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
            `, k.ID, k.UserID, k.DeviceID, k.KeyID, k.PublicKey, k.UploadedAt, k.ConsumedAt, k.ConsumedBy, k.ConsumedByDeviceID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresEncryptionRepository) ConsumeOneTimePreKey(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID, consumedBy uuid.UUID, consumedByDeviceID uuid.UUID) (encryption.OneTimePreKey, error) {
	var key encryption.OneTimePreKey
	err := WithTx(ctx, r.db, func(tx DBTX) error {
		err := tx.QueryRowContext(ctx, `
            SELECT id, user_id, device_id, key_id, public_key, uploaded_at, consumed_at, consumed_by, consumed_by_device_id
            FROM onetime_prekeys
            WHERE user_id = $1 AND device_id = $2 AND consumed_at IS NULL
            ORDER BY uploaded_at ASC LIMIT 1
        `, userID, deviceID).Scan(
			&key.ID,
			&key.UserID,
			&key.DeviceID,
			&key.KeyID,
			&key.PublicKey,
			&key.UploadedAt,
			&key.ConsumedAt,
			&key.ConsumedBy,
			&key.ConsumedByDeviceID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		now := time.Now()
		_, err = tx.ExecContext(ctx, `
            UPDATE onetime_prekeys
            SET consumed_at = $1, consumed_by = $2, consumed_by_device_id = $3
            WHERE id = $4
        `, now, consumedBy, consumedByDeviceID, key.ID)
		return err
	})
	if err != nil {
		return encryption.OneTimePreKey{}, err
	}
	return key, nil
}

func (r *PostgresEncryptionRepository) GetAvailablePreKeyCount(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM onetime_prekeys WHERE user_id = $1 AND device_id = $2 AND consumed_at IS NULL", userID, deviceID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresEncryptionRepository) DeleteConsumedPreKeys(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM onetime_prekeys WHERE consumed_at IS NOT NULL AND consumed_at < $1", olderThan)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (r *PostgresEncryptionRepository) HasActiveKeys(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM identity_keys WHERE user_id = $1 AND device_id = $2 AND is_active = true", userID, deviceID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
