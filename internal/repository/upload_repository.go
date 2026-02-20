package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/upload"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresUploadRepository struct {
	db DBTX
}

func NewUploadRepository(db DBTX) UploadRepository {
	return &PostgresUploadRepository{db: db}
}

func (r *PostgresUploadRepository) Create(ctx context.Context, u *upload.UploadSession) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO upload_sessions (id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
    `, u.ID, u.UploaderID, u.Filename, u.MimeType, u.SizeBytes, u.ChunkSize, u.UploadedBytes, u.Status, u.ObjectKey, u.FileURL, u.CompletedAt, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresUploadRepository) GetByID(ctx context.Context, id uuid.UUID) (upload.UploadSession, error) {
	var u upload.UploadSession
	err := r.db.QueryRowContext(ctx, `
        SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
        FROM upload_sessions WHERE id = $1
    `, id).Scan(
		&u.ID,
		&u.UploaderID,
		&u.Filename,
		&u.MimeType,
		&u.SizeBytes,
		&u.ChunkSize,
		&u.UploadedBytes,
		&u.Status,
		&u.ObjectKey,
		&u.FileURL,
		&u.CompletedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return upload.UploadSession{}, sentinal_errors.ErrNotFound
		}
		return upload.UploadSession{}, err
	}
	return u, nil
}

func (r *PostgresUploadRepository) Update(ctx context.Context, u upload.UploadSession) error {
	u.UpdatedAt = time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE upload_sessions
        SET uploader_id = $1, filename = $2, mime_type = $3, size_bytes = $4, chunk_size = $5,
            uploaded_bytes = $6, status = $7, object_key = $8, file_url = $9, completed_at = $10, updated_at = $11
        WHERE id = $12
    `, u.UploaderID, u.Filename, u.MimeType, u.SizeBytes, u.ChunkSize, u.UploadedBytes, u.Status, u.ObjectKey, u.FileURL, u.CompletedAt, u.UpdatedAt, u.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUploadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM upload_sessions WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUploadRepository) GetUserUploadSessions(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
        FROM upload_sessions WHERE uploader_id = $1
        ORDER BY created_at DESC
    `, uploaderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u upload.UploadSession
		if err := rows.Scan(
			&u.ID,
			&u.UploaderID,
			&u.Filename,
			&u.MimeType,
			&u.SizeBytes,
			&u.ChunkSize,
			&u.UploadedBytes,
			&u.Status,
			&u.ObjectKey,
			&u.FileURL,
			&u.CompletedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) GetInProgressUploads(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
        FROM upload_sessions WHERE uploader_id = $1 AND status = 'IN_PROGRESS'
        ORDER BY created_at DESC
    `, uploaderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u upload.UploadSession
		if err := rows.Scan(
			&u.ID,
			&u.UploaderID,
			&u.Filename,
			&u.MimeType,
			&u.SizeBytes,
			&u.ChunkSize,
			&u.UploadedBytes,
			&u.Status,
			&u.ObjectKey,
			&u.FileURL,
			&u.CompletedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) GetCompletedUploads(ctx context.Context, uploaderID uuid.UUID, page, limit int) ([]upload.UploadSession, int64, error) {
	var sessions []upload.UploadSession
	var total int64

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM upload_sessions WHERE uploader_id = $1 AND status = 'COMPLETED'", uploaderID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
        FROM upload_sessions
        WHERE uploader_id = $1 AND status = 'COMPLETED'
        ORDER BY updated_at DESC
        OFFSET $2 LIMIT $3
    `, uploaderID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var u upload.UploadSession
		if err := rows.Scan(
			&u.ID,
			&u.UploaderID,
			&u.Filename,
			&u.MimeType,
			&u.SizeBytes,
			&u.ChunkSize,
			&u.UploadedBytes,
			&u.Status,
			&u.ObjectKey,
			&u.FileURL,
			&u.CompletedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

func (r *PostgresUploadRepository) UpdateProgress(ctx context.Context, sessionID uuid.UUID, uploadedBytes int64) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE upload_sessions
        SET uploaded_bytes = $1, updated_at = $2
        WHERE id = $3
    `, uploadedBytes, time.Now(), sessionID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUploadRepository) MarkCompleted(ctx context.Context, sessionID uuid.UUID) error {
	return WithTx(ctx, r.db, func(tx DBTX) error {
		var session upload.UploadSession
		err := tx.QueryRowContext(ctx, `
            SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
            FROM upload_sessions WHERE id = $1
        `, sessionID).Scan(
			&session.ID,
			&session.UploaderID,
			&session.Filename,
			&session.MimeType,
			&session.SizeBytes,
			&session.ChunkSize,
			&session.UploadedBytes,
			&session.Status,
			&session.ObjectKey,
			&session.FileURL,
			&session.CompletedAt,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		completedAt := time.Now()
		_, err = tx.ExecContext(ctx, `
            UPDATE upload_sessions
            SET status = 'COMPLETED', uploaded_bytes = $1, completed_at = $2, updated_at = $3
            WHERE id = $4
        `, session.SizeBytes, completedAt, completedAt, sessionID)
		return err
	})
}

func (r *PostgresUploadRepository) MarkFailed(ctx context.Context, sessionID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE upload_sessions
        SET status = 'FAILED', updated_at = $1
        WHERE id = $2
    `, time.Now(), sessionID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresUploadRepository) GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error) {
	var sessions []upload.UploadSession
	cutoff := time.Now().Add(-olderThan)
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, uploader_id, filename, mime_type, size_bytes, chunk_size, uploaded_bytes, status, object_key, file_url, completed_at, created_at, updated_at
        FROM upload_sessions WHERE status = 'IN_PROGRESS' AND updated_at < $1
    `, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u upload.UploadSession
		if err := rows.Scan(
			&u.ID,
			&u.UploaderID,
			&u.Filename,
			&u.MimeType,
			&u.SizeBytes,
			&u.ChunkSize,
			&u.UploadedBytes,
			&u.Status,
			&u.ObjectKey,
			&u.FileURL,
			&u.CompletedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *PostgresUploadRepository) DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res, err := r.db.ExecContext(ctx, "DELETE FROM upload_sessions WHERE status = 'IN_PROGRESS' AND updated_at < $1", cutoff)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}
