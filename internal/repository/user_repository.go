package repository

import (
	"context"
	"errors"

	"sentinal-chat/internal/domain/user"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresUserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, u *user.User) error {
	res := r.db.WithContext(ctx).Create(u)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) GetAllUsers(ctx context.Context, page, limit int) ([]user.User, int64, error) {
	var users []user.User
	var total int64

	q := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("role <> ?", "SUPER_ADMIN")

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit

	if err := q.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND role <> ?", id, "SUPER_ADMIN").
		First(&u).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}

	return u, nil
}

func (r *PostgresUserRepository) UpdateUser(ctx context.Context, u user.User) error {
	res := r.db.WithContext(ctx).Save(&u)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&user.User{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) GetUserContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error) {
	var contacts []user.UserContact
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&contacts).Error
	if err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *PostgresUserRepository) AddUserContact(ctx context.Context, c *user.UserContact) error {
	res := r.db.WithContext(ctx).Create(c)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) RemoveUserContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Delete(&user.UserContact{}, "user_id = ? AND contact_user_id = ?", userID, contactUserID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) GetUserSettings(ctx context.Context, userID uuid.UUID) (user.UserSettings, error) {
	var s user.UserSettings
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.UserSettings{}, sentinal_errors.ErrNotFound
		}
		return user.UserSettings{}, err
	}
	return s, nil
}

func (r *PostgresUserRepository) UpdateUserSettings(ctx context.Context, s user.UserSettings) error {
	res := r.db.WithContext(ctx).Save(&s)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) AddDevice(ctx context.Context, d *user.Device) error {
	res := r.db.WithContext(ctx).Create(d)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]user.Device, error) {
	var devices []user.Device
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Find(&devices).Error
	if err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *PostgresUserRepository) DeactivateDevice(ctx context.Context, deviceID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.Device{}).
		Where("id = ?", deviceID).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}
