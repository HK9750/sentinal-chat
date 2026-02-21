package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/user"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresUserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) UserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, u *user.User) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO users (id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url, is_online, last_seen_at, is_active, is_verified, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    `,
		u.ID,
		u.PhoneNumber,
		u.Username,
		u.Email,
		u.PasswordHash,
		u.DisplayName,
		u.Role,
		u.Bio,
		u.AvatarURL,
		u.IsOnline,
		u.LastSeenAt,
		u.IsActive,
		u.IsVerified,
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetAllUsers(ctx context.Context, page, limit int) ([]user.User, int64, error) {
	var users []user.User
	var total int64

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role <> $1", "SUPER_ADMIN").Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users
        WHERE role <> $1
        ORDER BY created_at DESC
        OFFSET $2 LIMIT $3
    `, "SUPER_ADMIN", offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var u user.User
		var role, bio, avatarURL sql.NullString
		if err := rows.Scan(
			&u.ID,
			&u.PhoneNumber,
			&u.Username,
			&u.Email,
			&u.PasswordHash,
			&u.DisplayName,
			&role,
			&bio,
			&avatarURL,
			&u.IsOnline,
			&u.LastSeenAt,
			&u.IsActive,
			&u.IsVerified,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	var u user.User
	var role, bio, avatarURL sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users WHERE id = $1
    `, id).Scan(
		&u.ID,
		&u.PhoneNumber,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
		&role,
		&bio,
		&avatarURL,
		&u.IsOnline,
		&u.LastSeenAt,
		&u.IsActive,
		&u.IsVerified,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err == nil {
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
	}
	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) UpdateUser(ctx context.Context, u user.User) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE users
        SET phone_number = $1, username = $2, email = $3, password_hash = $4, display_name = $5, role = $6,
            bio = $7, avatar_url = $8, is_online = $9, last_seen_at = $10, is_active = $11, is_verified = $12,
            updated_at = $13
        WHERE id = $14
    `,
		u.PhoneNumber,
		u.Username,
		u.Email,
		u.PasswordHash,
		u.DisplayName,
		u.Role,
		u.Bio,
		u.AvatarURL,
		u.IsOnline,
		u.LastSeenAt,
		u.IsActive,
		u.IsVerified,
		u.UpdatedAt,
		u.ID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, email string) (user.User, error) {
	var u user.User
	var role, bio, avatarURL sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users WHERE email = $1
    `, email).Scan(
		&u.ID,
		&u.PhoneNumber,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
		&role,
		&bio,
		&avatarURL,
		&u.IsOnline,
		&u.LastSeenAt,
		&u.IsActive,
		&u.IsVerified,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err == nil {
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
	}
	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (user.User, error) {
	var u user.User
	var role, bio, avatarURL sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users WHERE username = $1
    `, username).Scan(
		&u.ID,
		&u.PhoneNumber,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
		&role,
		&bio,
		&avatarURL,
		&u.IsOnline,
		&u.LastSeenAt,
		&u.IsActive,
		&u.IsVerified,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err == nil {
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
	}
	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByPhoneNumber(ctx context.Context, phone string) (user.User, error) {
	var u user.User
	var role, bio, avatarURL sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users WHERE phone_number = $1
    `, phone).Scan(
		&u.ID,
		&u.PhoneNumber,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
		&role,
		&bio,
		&avatarURL,
		&u.IsOnline,
		&u.LastSeenAt,
		&u.IsActive,
		&u.IsVerified,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err == nil {
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
	}
	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
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
	if err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM users
        WHERE role <> $1 AND (display_name ILIKE $2 OR username ILIKE $2 OR email ILIKE $2)
    `, "SUPER_ADMIN", searchPattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, phone_number, username, email, password_hash, display_name, role, bio, avatar_url,
               is_online, last_seen_at, is_active, is_verified, created_at, updated_at
        FROM users
        WHERE role <> $1 AND (display_name ILIKE $2 OR username ILIKE $2 OR email ILIKE $2)
        ORDER BY display_name ASC
        OFFSET $3 LIMIT $4
    `, "SUPER_ADMIN", searchPattern, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var u user.User
		var role, bio, avatarURL sql.NullString
		if err := rows.Scan(
			&u.ID,
			&u.PhoneNumber,
			&u.Username,
			&u.Email,
			&u.PasswordHash,
			&u.DisplayName,
			&role,
			&bio,
			&avatarURL,
			&u.IsOnline,
			&u.LastSeenAt,
			&u.IsActive,
			&u.IsVerified,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		u.Role = role.String
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
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
	if isOnline {
		res, err := r.db.ExecContext(ctx, `
            UPDATE users SET is_online = $1, updated_at = $2 WHERE id = $3
        `, isOnline, updates["updated_at"], userID)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err == nil && rows == 0 {
			return sentinal_errors.ErrNotFound
		}
		return err
	}

	res, err := r.db.ExecContext(ctx, `
        UPDATE users SET is_online = $1, last_seen_at = $2, updated_at = $3 WHERE id = $4
    `, isOnline, updates["last_seen_at"], updates["updated_at"], userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) UpdateLastSeen(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error {
	res, err := r.db.ExecContext(ctx, "UPDATE users SET last_seen_at = $1 WHERE id = $2", lastSeen, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) GetUserContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error) {
	var contacts []user.UserContact
	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id, contact_user_id, nickname, is_blocked, created_at
        FROM user_contacts WHERE user_id = $1
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c user.UserContact
		if err := rows.Scan(&c.UserID, &c.ContactUserID, &c.Nickname, &c.IsBlocked, &c.CreatedAt); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *PostgresUserRepository) AddUserContact(ctx context.Context, c *user.UserContact) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO user_contacts (user_id, contact_user_id, nickname, is_blocked, created_at)
        VALUES ($1,$2,$3,$4,$5)
    `, c.UserID, c.ContactUserID, c.Nickname, c.IsBlocked, c.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) RemoveUserContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM user_contacts WHERE user_id = $1 AND contact_user_id = $2", userID, contactUserID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) BlockContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE user_contacts SET is_blocked = true WHERE user_id = $1 AND contact_user_id = $2", userID, contactUserID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		contact := &user.UserContact{
			UserID:        userID,
			ContactUserID: contactUserID,
			IsBlocked:     true,
			CreatedAt:     time.Now(),
		}
		_, err := r.db.ExecContext(ctx, `
            INSERT INTO user_contacts (user_id, contact_user_id, nickname, is_blocked, created_at)
            VALUES ($1,$2,$3,$4,$5)
        `, contact.UserID, contact.ContactUserID, contact.Nickname, contact.IsBlocked, contact.CreatedAt)
		return err
	}
	return err
}

func (r *PostgresUserRepository) UnblockContact(ctx context.Context, userID, contactUserID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE user_contacts SET is_blocked = false WHERE user_id = $1 AND contact_user_id = $2", userID, contactUserID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) GetBlockedContacts(ctx context.Context, userID uuid.UUID) ([]user.UserContact, error) {
	var contacts []user.UserContact
	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id, contact_user_id, nickname, is_blocked, created_at
        FROM user_contacts WHERE user_id = $1 AND is_blocked = true
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c user.UserContact
		if err := rows.Scan(&c.UserID, &c.ContactUserID, &c.Nickname, &c.IsBlocked, &c.CreatedAt); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *PostgresUserRepository) GetUserSettings(ctx context.Context, userID uuid.UUID) (user.UserSettings, error) {
	var s user.UserSettings
	var privacyLastSeen, privacyProfilePhoto, privacyAbout, privacyGroups, notificationSound, theme, language sql.NullString
	var readReceipts, notificationsEnabled, notificationVibrate, enterToSend, mediaAutoDownloadWiFi, mediaAutoDownloadMobile sql.NullBool
	err := r.db.QueryRowContext(ctx, `
        SELECT user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
               read_receipts, notifications_enabled, notification_sound, notification_vibrate,
               theme, language, enter_to_send, media_auto_download_wifi, media_auto_download_mobile, updated_at
        FROM user_settings WHERE user_id = $1
    `, userID).Scan(
		&s.UserID,
		&privacyLastSeen,
		&privacyProfilePhoto,
		&privacyAbout,
		&privacyGroups,
		&readReceipts,
		&notificationsEnabled,
		&notificationSound,
		&notificationVibrate,
		&theme,
		&language,
		&enterToSend,
		&mediaAutoDownloadWiFi,
		&mediaAutoDownloadMobile,
		&s.UpdatedAt,
	)
	if err == nil {
		s.PrivacyLastSeen = privacyLastSeen.String
		s.PrivacyProfilePhoto = privacyProfilePhoto.String
		s.PrivacyAbout = privacyAbout.String
		s.PrivacyGroups = privacyGroups.String
		s.ReadReceipts = readReceipts.Bool
		s.NotificationsEnabled = notificationsEnabled.Bool
		s.NotificationSound = notificationSound.String
		s.NotificationVibrate = notificationVibrate.Bool
		s.Theme = theme.String
		s.Language = language.String
		s.EnterToSend = enterToSend.Bool
		s.MediaAutoDownloadWiFi = mediaAutoDownloadWiFi.Bool
		s.MediaAutoDownloadMobile = mediaAutoDownloadMobile.Bool
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.UserSettings{}, sentinal_errors.ErrNotFound
		}
		return user.UserSettings{}, err
	}
	return s, nil
}

func (r *PostgresUserRepository) UpdateUserSettings(ctx context.Context, s user.UserSettings) error {
	s.UpdatedAt = time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE user_settings
        SET privacy_last_seen = $1, privacy_profile_photo = $2, privacy_about = $3, privacy_groups = $4,
            read_receipts = $5, notifications_enabled = $6, notification_sound = $7, notification_vibrate = $8,
            theme = $9, language = $10, enter_to_send = $11, media_auto_download_wifi = $12,
            media_auto_download_mobile = $13, updated_at = $14
        WHERE user_id = $15
    `,
		s.PrivacyLastSeen,
		s.PrivacyProfilePhoto,
		s.PrivacyAbout,
		s.PrivacyGroups,
		s.ReadReceipts,
		s.NotificationsEnabled,
		s.NotificationSound,
		s.NotificationVibrate,
		s.Theme,
		s.Language,
		s.EnterToSend,
		s.MediaAutoDownloadWiFi,
		s.MediaAutoDownloadMobile,
		s.UpdatedAt,
		s.UserID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) CreateUserSettings(ctx context.Context, s *user.UserSettings) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO user_settings (
            user_id, privacy_last_seen, privacy_profile_photo, privacy_about, privacy_groups,
            read_receipts, notifications_enabled, notification_sound, notification_vibrate,
            theme, language, enter_to_send, media_auto_download_wifi, media_auto_download_mobile, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    `,
		s.UserID,
		s.PrivacyLastSeen,
		s.PrivacyProfilePhoto,
		s.PrivacyAbout,
		s.PrivacyGroups,
		s.ReadReceipts,
		s.NotificationsEnabled,
		s.NotificationSound,
		s.NotificationVibrate,
		s.Theme,
		s.Language,
		s.EnterToSend,
		s.MediaAutoDownloadWiFi,
		s.MediaAutoDownloadMobile,
		s.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) AddDevice(ctx context.Context, d *user.Device) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO devices (id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, d.ID, d.UserID, d.DeviceID, d.DeviceName, d.DeviceType, d.IsActive, d.RegisteredAt, d.LastSeenAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]user.Device, error) {
	var devices []user.Device
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at
        FROM devices WHERE user_id = $1 AND is_active = true
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d user.Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.DeviceID, &d.DeviceName, &d.DeviceType, &d.IsActive, &d.RegisteredAt, &d.LastSeenAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *PostgresUserRepository) GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (user.Device, error) {
	var d user.Device
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, device_id, device_name, device_type, is_active, registered_at, last_seen_at
        FROM devices WHERE id = $1
    `, deviceID).Scan(&d.ID, &d.UserID, &d.DeviceID, &d.DeviceName, &d.DeviceType, &d.IsActive, &d.RegisteredAt, &d.LastSeenAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.Device{}, sentinal_errors.ErrNotFound
		}
		return user.Device{}, err
	}
	return d, nil
}

func (r *PostgresUserRepository) DeactivateDevice(ctx context.Context, deviceID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE devices SET is_active = false WHERE id = $1", deviceID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) UpdateDeviceLastSeen(ctx context.Context, deviceID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE devices SET last_seen_at = $1 WHERE id = $2", time.Now(), deviceID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) AddPushToken(ctx context.Context, pt *user.PushToken) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO push_tokens (id, user_id, device_id, platform, token, is_active, created_at, last_used_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, pt.ID, pt.UserID, pt.DeviceID, pt.Platform, pt.Token, pt.IsActive, pt.CreatedAt, pt.LastUsedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetUserPushTokens(ctx context.Context, userID uuid.UUID) ([]user.PushToken, error) {
	var tokens []user.PushToken
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, device_id, platform, token, is_active, created_at, last_used_at
        FROM push_tokens WHERE user_id = $1 AND is_active = true
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pt user.PushToken
		if err := rows.Scan(&pt.ID, &pt.UserID, &pt.DeviceID, &pt.Platform, &pt.Token, &pt.IsActive, &pt.CreatedAt, &pt.LastUsedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *PostgresUserRepository) DeactivatePushToken(ctx context.Context, tokenID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE push_tokens SET is_active = false WHERE id = $1", tokenID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) CreateSession(ctx context.Context, s *user.UserSession) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO user_sessions (id, user_id, device_id, refresh_token_hash, expires_at, is_revoked, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, s.ID, s.UserID, s.DeviceID, s.RefreshTokenHash, s.ExpiresAt, s.IsRevoked, s.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (user.UserSession, error) {
	var s user.UserSession
	var deviceID uuid.NullUUID
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, device_id, refresh_token_hash, expires_at, is_revoked, created_at
        FROM user_sessions
        WHERE id = $1 AND is_revoked = false AND expires_at > NOW()
    `, sessionID).Scan(&s.ID, &s.UserID, &deviceID, &s.RefreshTokenHash, &s.ExpiresAt, &s.IsRevoked, &s.CreatedAt)
	if err == nil && deviceID.Valid {
		s.DeviceID = &deviceID.UUID
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.UserSession{}, sentinal_errors.ErrNotFound
		}
		return user.UserSession{}, err
	}
	return s, nil
}

func (r *PostgresUserRepository) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]user.UserSession, error) {
	var sessions []user.UserSession
	rows, err := r.db.QueryContext(ctx, `
        SELECT s.id, s.user_id, s.device_id, s.refresh_token_hash, s.expires_at, s.is_revoked, s.created_at,
               d.id, d.user_id, d.device_id, d.device_name, d.device_type, d.is_active, d.registered_at, d.last_seen_at
        FROM user_sessions s
        LEFT JOIN devices d ON d.id = s.device_id
        WHERE s.user_id = $1 AND s.is_revoked = false AND s.expires_at > NOW()
        ORDER BY s.created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s user.UserSession
		var device user.Device
		var deviceID uuid.NullUUID
		var deviceName, deviceType sql.NullString
		var isActive sql.NullBool
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&deviceID,
			&s.RefreshTokenHash,
			&s.ExpiresAt,
			&s.IsRevoked,
			&s.CreatedAt,
			&device.ID,
			&device.UserID,
			&device.DeviceID,
			&deviceName,
			&deviceType,
			&isActive,
			&device.RegisteredAt,
			&device.LastSeenAt,
		); err != nil {
			return nil, err
		}
		device.DeviceName = deviceName.String
		device.DeviceType = deviceType.String
		device.IsActive = isActive.Bool
		if deviceID.Valid {
			s.DeviceID = &deviceID.UUID
			s.Device = &device
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUserRepository) UpdateSession(ctx context.Context, s user.UserSession) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE user_sessions
        SET refresh_token_hash = $1, expires_at = $2, is_revoked = $3
        WHERE id = $4
    `, s.RefreshTokenHash, s.ExpiresAt, s.IsRevoked, s.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE user_sessions SET is_revoked = true WHERE id = $1", sessionID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUserRepository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user_sessions SET is_revoked = true WHERE user_id = $1 AND is_revoked = false", userID)
	return err
}

func (r *PostgresUserRepository) CleanExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM user_sessions WHERE expires_at < NOW()")
	return err
}
