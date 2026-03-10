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

// scanUser is a helper to scan a user row (without role column).
func scanUser(scanner interface {
	Scan(dest ...any) error
}) (user.User, error) {
	var u user.User
	var bio, avatarURL sql.NullString
	err := scanner.Scan(
		&u.ID,
		&u.PhoneNumber,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.DisplayName,
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
		u.Bio = bio.String
		u.AvatarURL = avatarURL.String
	}
	return u, err
}

const userColumns = `id, phone_number, username, email, password_hash, display_name, bio, avatar_url,
       is_online, last_seen_at, is_active, is_verified, created_at, updated_at`

func (r *PostgresUserRepository) Create(ctx context.Context, u *user.User) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO users (phone_number, username, email, password_hash, display_name, bio, avatar_url, is_online, last_seen_at, is_active, is_verified, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
    `,
		u.PhoneNumber,
		u.Username,
		u.Email,
		u.PasswordHash,
		u.DisplayName,
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

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+userColumns+`
        FROM users
        ORDER BY created_at DESC
        OFFSET $1 LIMIT $2
    `, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `
        SELECT `+userColumns+` FROM users WHERE id = $1
    `, id))
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
        SET phone_number = $1, username = $2, email = $3, password_hash = $4, display_name = $5,
            bio = $6, avatar_url = $7, is_online = $8, last_seen_at = $9, is_active = $10, is_verified = $11,
            updated_at = $12
        WHERE id = $13
    `,
		u.PhoneNumber,
		u.Username,
		u.Email,
		u.PasswordHash,
		u.DisplayName,
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
	u, err := scanUser(r.db.QueryRowContext(ctx, `
        SELECT `+userColumns+` FROM users WHERE email = $1
    `, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (user.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `
        SELECT `+userColumns+` FROM users WHERE username = $1
    `, username))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, sentinal_errors.ErrNotFound
		}
		return user.User{}, err
	}
	return u, nil
}

func (r *PostgresUserRepository) GetUserByPhoneNumber(ctx context.Context, phone string) (user.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, `
        SELECT `+userColumns+` FROM users WHERE phone_number = $1
    `, phone))
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
        WHERE display_name ILIKE $1 OR username ILIKE $1 OR email ILIKE $1
    `, searchPattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+userColumns+`
        FROM users
        WHERE display_name ILIKE $1 OR username ILIKE $1 OR email ILIKE $1
        ORDER BY display_name ASC
        OFFSET $2 LIMIT $3
    `, searchPattern, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *PostgresUserRepository) UpdateOnlineStatus(ctx context.Context, userID uuid.UUID, isOnline bool) error {
	now := time.Now()
	var res sql.Result
	var err error
	if isOnline {
		res, err = r.db.ExecContext(ctx, `UPDATE users SET is_online = $1, updated_at = $2 WHERE id = $3`, isOnline, now, userID)
	} else {
		res, err = r.db.ExecContext(ctx, `UPDATE users SET is_online = $1, last_seen_at = $2, updated_at = $3 WHERE id = $4`, isOnline, now, now, userID)
	}
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

func (r *PostgresUserRepository) AddFcmToken(ctx context.Context, ft *user.FcmToken) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO fcm_tokens (id, user_id, device_id, platform, token, is_active, created_at, last_used_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, ft.ID, ft.UserID, ft.DeviceID, ft.Platform, ft.Token, ft.IsActive, ft.CreatedAt, ft.LastUsedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetUserFcmTokens(ctx context.Context, userID uuid.UUID) ([]user.FcmToken, error) {
	var tokens []user.FcmToken
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, device_id, platform, token, is_active, created_at, last_used_at
        FROM fcm_tokens WHERE user_id = $1 AND is_active = true
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ft user.FcmToken
		if err := rows.Scan(&ft.ID, &ft.UserID, &ft.DeviceID, &ft.Platform, &ft.Token, &ft.IsActive, &ft.CreatedAt, &ft.LastUsedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, ft)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *PostgresUserRepository) DeactivateFcmToken(ctx context.Context, tokenID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE fcm_tokens SET is_active = false WHERE id = $1", tokenID)
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
