package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/broadcast"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresBroadcastRepository struct {
	db DBTX
}

func NewBroadcastRepository(db DBTX) BroadcastRepository {
	return &PostgresBroadcastRepository{db: db}
}

func (r *PostgresBroadcastRepository) Create(ctx context.Context, b *broadcast.BroadcastList) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO broadcast_lists (id, owner_id, name, description, created_at)
        VALUES ($1,$2,$3,$4,$5)
    `, b.ID, b.OwnerID, b.Name, b.Description, b.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresBroadcastRepository) GetByID(ctx context.Context, id uuid.UUID) (broadcast.BroadcastList, error) {
	var b broadcast.BroadcastList
	err := r.db.QueryRowContext(ctx, `
        SELECT id, owner_id, name, description, created_at
        FROM broadcast_lists WHERE id = $1
    `, id).Scan(&b.ID, &b.OwnerID, &b.Name, &b.Description, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return broadcast.BroadcastList{}, sentinal_errors.ErrNotFound
		}
		return broadcast.BroadcastList{}, err
	}
	return b, nil
}

func (r *PostgresBroadcastRepository) Update(ctx context.Context, b broadcast.BroadcastList) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE broadcast_lists
        SET owner_id = $1, name = $2, description = $3
        WHERE id = $4
    `, b.OwnerID, b.Name, b.Description, b.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresBroadcastRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM broadcast_lists WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresBroadcastRepository) GetUserBroadcastLists(ctx context.Context, ownerID uuid.UUID) ([]broadcast.BroadcastList, error) {
	var lists []broadcast.BroadcastList
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, owner_id, name, description, created_at
        FROM broadcast_lists WHERE owner_id = $1
        ORDER BY created_at DESC
    `, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b broadcast.BroadcastList
		if err := rows.Scan(&b.ID, &b.OwnerID, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *PostgresBroadcastRepository) SearchBroadcastLists(ctx context.Context, ownerID uuid.UUID, query string) ([]broadcast.BroadcastList, error) {
	var lists []broadcast.BroadcastList
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, owner_id, name, description, created_at
        FROM broadcast_lists WHERE owner_id = $1 AND name ILIKE $2
        ORDER BY name ASC
    `, ownerID, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b broadcast.BroadcastList
		if err := rows.Scan(&b.ID, &b.OwnerID, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *PostgresBroadcastRepository) AddRecipient(ctx context.Context, rec *broadcast.BroadcastRecipient) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO broadcast_recipients (broadcast_id, user_id, added_at)
        VALUES ($1,$2,$3)
    `, rec.BroadcastID, rec.UserID, rec.AddedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresBroadcastRepository) RemoveRecipient(ctx context.Context, broadcastID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM broadcast_recipients WHERE broadcast_id = $1 AND user_id = $2", broadcastID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresBroadcastRepository) GetRecipients(ctx context.Context, broadcastID uuid.UUID) ([]broadcast.BroadcastRecipient, error) {
	var recipients []broadcast.BroadcastRecipient
	rows, err := r.db.QueryContext(ctx, `
        SELECT broadcast_id, user_id, added_at
        FROM broadcast_recipients WHERE broadcast_id = $1
        ORDER BY added_at ASC
    `, broadcastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rcp broadcast.BroadcastRecipient
		if err := rows.Scan(&rcp.BroadcastID, &rcp.UserID, &rcp.AddedAt); err != nil {
			return nil, err
		}
		recipients = append(recipients, rcp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return recipients, nil
}

func (r *PostgresBroadcastRepository) GetRecipientCount(ctx context.Context, broadcastID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM broadcast_recipients WHERE broadcast_id = $1", broadcastID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresBroadcastRepository) IsRecipient(ctx context.Context, broadcastID, userID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM broadcast_recipients WHERE broadcast_id = $1 AND user_id = $2", broadcastID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *PostgresBroadcastRepository) BulkAddRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	return WithTx(ctx, r.db, func(tx DBTX) error {
		now := time.Now()
		for _, userID := range userIDs {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO broadcast_recipients (broadcast_id, user_id, added_at)
                VALUES ($1,$2,$3)
            `, broadcastID, userID, now)
			if err != nil {
				if isUniqueViolation(err) {
					continue
				}
				return err
			}
		}
		return nil
	})
}

func (r *PostgresBroadcastRepository) BulkRemoveRecipients(ctx context.Context, broadcastID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}
	placeholders := buildPlaceholders(2, len(userIDs))
	args := make([]interface{}, 0, len(userIDs)+1)
	args = append(args, broadcastID)
	for _, id := range userIDs {
		args = append(args, id)
	}
	_, err := r.db.ExecContext(ctx, "DELETE FROM broadcast_recipients WHERE broadcast_id = $1 AND user_id IN ("+placeholders+")", args...)
	return err
}
