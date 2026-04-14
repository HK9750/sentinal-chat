package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/notification"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type notificationRepository struct {
	db DBTX
}

func NewNotificationRepository(db DBTX) NotificationRepository {
	return &notificationRepository{db: db}
}

const notificationColumns = `id, user_id, actor_id, conversation_id, message_id, call_id,
	   type, title, body, deep_link, metadata, dedupe_key, is_read, read_at, created_at, updated_at`

func scanNotification(scanner interface {
	Scan(dest ...any) error
}) (notification.Notification, error) {
	var item notification.Notification
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.ActorID,
		&item.ConversationID,
		&item.MessageID,
		&item.CallID,
		&item.Type,
		&item.Title,
		&item.Body,
		&item.DeepLink,
		&item.Metadata,
		&item.DedupeKey,
		&item.IsRead,
		&item.ReadAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (r *notificationRepository) Create(ctx context.Context, item *notification.Notification) error {
	if item == nil {
		return sentinal_errors.ErrInvalidInput
	}
	now := time.Now().UTC()
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	if len(item.Metadata) == 0 {
		item.Metadata = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (
			id, user_id, actor_id, conversation_id, message_id, call_id,
			type, title, body, deep_link, metadata, dedupe_key,
			is_read, read_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		item.ID,
		item.UserID,
		item.ActorID,
		item.ConversationID,
		item.MessageID,
		item.CallID,
		item.Type,
		item.Title,
		item.Body,
		item.DeepLink,
		item.Metadata,
		item.DedupeKey,
		item.IsRead,
		item.ReadAt,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *notificationRepository) UpsertByDedupeKey(ctx context.Context, item *notification.Notification) (notification.Notification, error) {
	if item == nil {
		return notification.Notification{}, sentinal_errors.ErrInvalidInput
	}
	dedupeKey := item.DedupeKey
	if !dedupeKey.Valid || dedupeKey.String == "" {
		if err := r.Create(ctx, item); err != nil {
			return notification.Notification{}, err
		}
		return *item, nil
	}
	now := time.Now().UTC()
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	if len(item.Metadata) == 0 {
		item.Metadata = []byte("{}")
	}

	stored, err := scanNotification(r.db.QueryRowContext(ctx, `
		INSERT INTO notifications (
			id, user_id, actor_id, conversation_id, message_id, call_id,
			type, title, body, deep_link, metadata, dedupe_key,
			is_read, read_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,FALSE,NULL,$13,$14)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL
		DO UPDATE SET
			actor_id = COALESCE(EXCLUDED.actor_id, notifications.actor_id),
			conversation_id = COALESCE(EXCLUDED.conversation_id, notifications.conversation_id),
			message_id = COALESCE(EXCLUDED.message_id, notifications.message_id),
			call_id = COALESCE(EXCLUDED.call_id, notifications.call_id),
			type = EXCLUDED.type,
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			deep_link = EXCLUDED.deep_link,
			metadata = EXCLUDED.metadata,
			is_read = FALSE,
			read_at = NULL,
			updated_at = NOW()
		RETURNING `+notificationColumns,
		item.ID,
		item.UserID,
		item.ActorID,
		item.ConversationID,
		item.MessageID,
		item.CallID,
		item.Type,
		item.Title,
		item.Body,
		item.DeepLink,
		item.Metadata,
		item.DedupeKey,
		item.CreatedAt,
		item.UpdatedAt,
	))
	if err != nil {
		return notification.Notification{}, err
	}
	return stored, nil
}

func (r *notificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int, unreadOnly bool) ([]notification.Notification, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int64
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	if unreadOnly {
		countQuery += ` AND is_read = FALSE`
	}
	if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + notificationColumns + ` FROM notifications WHERE user_id = $1`
	args := []any{userID}
	if unreadOnly {
		query += ` AND is_read = FALSE`
	}
	offset := (page - 1) * limit
	args = append(args, offset, limit)
	query += ` ORDER BY created_at DESC OFFSET $2 LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]notification.Notification, 0)
	for rows.Next() {
		item, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *notificationRepository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notifications
		SET is_read = TRUE,
			read_at = COALESCE(read_at, NOW()),
			updated_at = NOW()
		WHERE user_id = $1 AND id = $2
	`, userID, notificationID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notifications
		SET is_read = TRUE,
			read_at = COALESCE(read_at, NOW()),
			updated_at = NOW()
		WHERE user_id = $1 AND is_read = FALSE
	`, userID)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (r *notificationRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND is_read = FALSE
	`, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *notificationRepository) GetSettings(ctx context.Context, userID uuid.UUID) (notification.UserNotificationSettings, error) {
	var settings notification.UserNotificationSettings
	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, in_app_enabled, sound_enabled, show_message_preview, created_at, updated_at
		FROM user_notification_settings
		WHERE user_id = $1
	`, userID).Scan(
		&settings.UserID,
		&settings.InAppEnabled,
		&settings.SoundEnabled,
		&settings.ShowMessagePreview,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notification.UserNotificationSettings{}, sentinal_errors.ErrNotFound
		}
		return notification.UserNotificationSettings{}, err
	}
	return settings, nil
}

func (r *notificationRepository) UpsertSettings(ctx context.Context, settings *notification.UserNotificationSettings) error {
	if settings == nil || settings.UserID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	now := time.Now().UTC()
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_notification_settings (
			user_id, in_app_enabled, sound_enabled, show_message_preview, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id)
		DO UPDATE SET
			in_app_enabled = EXCLUDED.in_app_enabled,
			sound_enabled = EXCLUDED.sound_enabled,
			show_message_preview = EXCLUDED.show_message_preview,
			updated_at = EXCLUDED.updated_at
	`,
		settings.UserID,
		settings.InAppEnabled,
		settings.SoundEnabled,
		settings.ShowMessagePreview,
		settings.CreatedAt,
		settings.UpdatedAt,
	)
	return err
}
