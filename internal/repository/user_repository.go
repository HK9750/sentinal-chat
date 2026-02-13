package repository

import (
	"context"
	"errors"
	"time"

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
		Where("id = ?", id).
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

func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, email string) (user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByPhoneNumber(ctx context.Context, phone string) (user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).
		Where("phone_number = ?", phone).
		First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) SearchUsers(ctx context.Context, query string, page, limit int) ([]user.User, int64, error) {
	var users []user.User
	var total int64

	searchPattern := "%" + query + "%"
	q := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("role <> ? AND (display_name ILIKE ? OR username ILIKE ? OR email ILIKE ?)",
			"SUPER_ADMIN", searchPattern, searchPattern, searchPattern)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("display_name ASC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *PostgresUserRepository) UpdateOnlineStatus(ctx context.Context, userID uuid.UUID, isOnline bool) error {
	updates := map[string]interface{}{
		"is_online":  isOnline,
		"updated_at": time.Now(),
	}
	if !isOnline {
		updates["last_seen_at"] = time.Now()
	}

	res := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) UpdateLastSeen(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Update("last_seen_at", lastSeen)
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

func (r *PostgresUserRepository) BlockContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.UserContact{}).
		Where("user_id = ? AND contact_user_id = ?", userID, contactUserID).
		Update("is_blocked", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// Create new blocked contact if doesn't exist
		contact := &user.UserContact{
			UserID:        userID,
			ContactUserID: contactUserID,
			IsBlocked:     true,
			CreatedAt:     time.Now(),
		}
		return r.db.WithContext(ctx).Create(contact).Error
	}
	return nil
}

func (r *PostgresUserRepository) UnblockContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.UserContact{}).
		Where("user_id = ? AND contact_user_id = ?", userID, contactUserID).
		Update("is_blocked", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) GetBlockedContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error) {
	var contacts []user.UserContact
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_blocked = true", userID).
		Find(&contacts).Error
	if err != nil {
		return nil, err
	}
	return contacts, nil
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

func (r *PostgresUserRepository) CreateUserSettings(ctx context.Context, s *user.UserSettings) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
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

func (r *PostgresUserRepository) GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (user.Device, error) {
	var d user.Device
	err := r.db.WithContext(ctx).
		Where("id = ?", deviceID).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.Device{}, sentinal_errors.ErrNotFound
		}
		return user.Device{}, err
	}
	return d, nil
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

func (r *PostgresUserRepository) UpdateDeviceLastSeen(ctx context.Context, deviceID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.Device{}).
		Where("id = ?", deviceID).
		Update("last_seen_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) AddPushToken(ctx context.Context, pt *user.PushToken) error {
	res := r.db.WithContext(ctx).Create(pt)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) GetUserPushTokens(ctx context.Context, userID uuid.UUID) ([]user.PushToken, error) {
	var tokens []user.PushToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *PostgresUserRepository) DeactivatePushToken(ctx context.Context, tokenID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.PushToken{}).
		Where("id = ?", tokenID).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) CreateSession(ctx context.Context, s *user.UserSession) error {
	res := r.db.WithContext(ctx).Create(s)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return sentinal_errors.ErrAlreadyExists
		}
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (user.UserSession, error) {
	var s user.UserSession
	err := r.db.WithContext(ctx).
		Where("id = ? AND is_revoked = false AND expires_at > NOW()", sessionID).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user.UserSession{}, sentinal_errors.ErrNotFound
		}
		return user.UserSession{}, err
	}
	return s, nil
}

func (r *PostgresUserRepository) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]user.UserSession, error) {
	var sessions []user.UserSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_revoked = false AND expires_at > NOW()", userID).
		Order("created_at DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUserRepository) UpdateSession(ctx context.Context, s user.UserSession) error {
	res := r.db.WithContext(ctx).Save(&s)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.UserSession{}).
		Where("id = ?", sessionID).
		Update("is_revoked", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sentinal_errors.ErrNotFound
	}
	return nil
}

func (r *PostgresUserRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&user.UserSession{}).
		Where("user_id = ? AND is_revoked = false", userID).
		Update("is_revoked", true)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *PostgresUserRepository) CleanExpiredSessions(ctx context.Context) error {
	res := r.db.WithContext(ctx).
		Delete(&user.UserSession{}, "expires_at < NOW()")
	if res.Error != nil {
		return res.Error
	}
	return nil
}
