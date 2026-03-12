package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sentinal-chat/internal/domain/call"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type PostgresCallRepository struct {
	db DBTX
}

func NewCallRepository(db DBTX) CallRepository {
	return &PostgresCallRepository{db: db}
}

const callColumns = `id, conversation_id, initiated_by, type, is_group_call, started_at, connected_at, ended_at, end_reason, duration_seconds, created_at`

func scanCall(scanner interface {
	Scan(dest ...interface{}) error
}) (call.Call, error) {
	var c call.Call
	err := scanner.Scan(
		&c.ID,
		&c.ConversationID,
		&c.InitiatedBy,
		&c.Type,
		&c.IsGroupCall,
		&c.StartedAt,
		&c.ConnectedAt,
		&c.EndedAt,
		&c.EndReason,
		&c.DurationSeconds,
		&c.CreatedAt,
	)
	return c, err
}

func (r *PostgresCallRepository) Create(ctx context.Context, c *call.Call) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO calls (id, conversation_id, initiated_by, type, is_group_call, started_at, connected_at, ended_at, end_reason, duration_seconds, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `, c.ID, c.ConversationID, c.InitiatedBy, c.Type, c.IsGroupCall, c.StartedAt, c.ConnectedAt, c.EndedAt, c.EndReason, c.DurationSeconds, c.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresCallRepository) GetByID(ctx context.Context, id uuid.UUID) (call.Call, error) {
	c, err := scanCall(r.db.QueryRowContext(ctx, `
        SELECT `+callColumns+` FROM calls WHERE id = $1
    `, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return call.Call{}, sentinal_errors.ErrNotFound
		}
		return call.Call{}, err
	}
	return c, nil
}

func (r *PostgresCallRepository) Update(ctx context.Context, c call.Call) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE calls
        SET conversation_id = $1, initiated_by = $2, type = $3, is_group_call = $4,
            started_at = $5, connected_at = $6, ended_at = $7, end_reason = $8, duration_seconds = $9
        WHERE id = $10
    `,
		c.ConversationID,
		c.InitiatedBy,
		c.Type,
		c.IsGroupCall,
		c.StartedAt,
		c.ConnectedAt,
		c.EndedAt,
		c.EndReason,
		c.DurationSeconds,
		c.ID,
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

func (r *PostgresCallRepository) GetConversationCalls(ctx context.Context, conversationID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	var calls []call.Call
	var total int64

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM calls WHERE conversation_id = $1", conversationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+callColumns+`
        FROM calls
        WHERE conversation_id = $1
        ORDER BY started_at DESC
        OFFSET $2 LIMIT $3
    `, conversationID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, 0, err
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return calls, total, nil
}

func (r *PostgresCallRepository) GetUserCalls(ctx context.Context, userID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	var calls []call.Call
	var total int64

	if err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM calls
        WHERE initiated_by = $1 OR id IN (SELECT call_id FROM call_participants WHERE user_id = $1)
    `, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+callColumns+`
        FROM calls
        WHERE initiated_by = $1 OR id IN (SELECT call_id FROM call_participants WHERE user_id = $1)
        ORDER BY started_at DESC
        OFFSET $2 LIMIT $3
    `, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, 0, err
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return calls, total, nil
}

func (r *PostgresCallRepository) GetActiveCalls(ctx context.Context, userID uuid.UUID) ([]call.Call, error) {
	var calls []call.Call
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+callColumns+`
        FROM calls
        WHERE ended_at IS NULL AND (initiated_by = $1 OR id IN (
            SELECT call_id FROM call_participants WHERE user_id = $1 AND status IN ('INVITED','JOINED')
        ))
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return calls, nil
}

func (r *PostgresCallRepository) GetMissedCalls(ctx context.Context, userID uuid.UUID, since time.Time) ([]call.Call, error) {
	var calls []call.Call
	rows, err := r.db.QueryContext(ctx, `
        SELECT `+callColumns+`
        FROM calls
        WHERE id IN (
            SELECT call_id FROM call_participants
            WHERE user_id = $1 AND status = 'INVITED' AND joined_at IS NULL
        ) AND ended_at IS NOT NULL AND started_at > $2
        ORDER BY started_at DESC
    `, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return calls, nil
}

func (r *PostgresCallRepository) MarkConnected(ctx context.Context, callID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE calls SET connected_at = $1 WHERE id = $2", time.Now(), callID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresCallRepository) EndCall(ctx context.Context, callID uuid.UUID, reason string) error {
	now := time.Now()
	return WithTx(ctx, r.db, func(tx DBTX) error {
		var c call.Call
		c, err := scanCall(tx.QueryRowContext(ctx, `
            SELECT `+callColumns+` FROM calls WHERE id = $1
        `, callID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sentinal_errors.ErrNotFound
			}
			return err
		}

		var duration any
		if c.ConnectedAt.Valid {
			d := int32(now.Sub(c.ConnectedAt.Time).Seconds())
			duration = d
		}

		_, err = tx.ExecContext(ctx, `
            UPDATE calls
            SET ended_at = $1, end_reason = $2, duration_seconds = COALESCE($3, duration_seconds)
            WHERE id = $4
        `, now, reason, duration, callID)
		return err
	})
}

func (r *PostgresCallRepository) GetCallDuration(ctx context.Context, callID uuid.UUID) (int32, error) {
	var duration sql.NullInt32
	err := r.db.QueryRowContext(ctx, "SELECT duration_seconds FROM calls WHERE id = $1", callID).Scan(&duration)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, sentinal_errors.ErrNotFound
		}
		return 0, err
	}
	if duration.Valid {
		return duration.Int32, nil
	}
	return 0, nil
}

func (r *PostgresCallRepository) AddParticipant(ctx context.Context, p *call.CallParticipant) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO call_participants (call_id, user_id, status, joined_at, left_at, muted_audio, muted_video)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
    `, p.CallID, p.UserID, p.Status, p.JoinedAt, p.LeftAt, p.MutedAudio, p.MutedVideo)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresCallRepository) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
        UPDATE call_participants
        SET status = 'LEFT', left_at = $1
        WHERE call_id = $2 AND user_id = $3
    `, now, callID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresCallRepository) GetCallParticipants(ctx context.Context, callID uuid.UUID) ([]call.CallParticipant, error) {
	var participants []call.CallParticipant
	rows, err := r.db.QueryContext(ctx, `
        SELECT call_id, user_id, status, joined_at, left_at, muted_audio, muted_video
        FROM call_participants WHERE call_id = $1
    `, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p call.CallParticipant
		if err := rows.Scan(&p.CallID, &p.UserID, &p.Status, &p.JoinedAt, &p.LeftAt, &p.MutedAudio, &p.MutedVideo); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *PostgresCallRepository) IsCallParticipant(ctx context.Context, callID, userID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM call_participants WHERE call_id = $1 AND user_id = $2", callID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

var (
	StatusJoined = "CONNECTED"
	StatusLeft   = "LEFT"
)

func (r *PostgresCallRepository) UpdateParticipantStatus(ctx context.Context, callID, userID uuid.UUID, status string) error {
	updates := map[string]any{
		"status": status,
	}
	switch status {
	case StatusJoined:
		updates["joined_at"] = time.Now()
	case StatusLeft:
		updates["left_at"] = time.Now()
	}

	res, err := r.db.ExecContext(ctx, `
        UPDATE call_participants
        SET status = $1, joined_at = COALESCE($2, joined_at), left_at = COALESCE($3, left_at)
        WHERE call_id = $4 AND user_id = $5
    `, updates["status"], updates["joined_at"], updates["left_at"], callID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresCallRepository) UpdateParticipantMuteStatus(ctx context.Context, callID, userID uuid.UUID, audioMuted, videoMuted bool) error {
	res, err := r.db.ExecContext(ctx, `
        UPDATE call_participants
        SET muted_audio = $1, muted_video = $2
        WHERE call_id = $3 AND user_id = $4
    `, audioMuted, videoMuted, callID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sentinal_errors.ErrNotFound
	}
	return err
}

func (r *PostgresCallRepository) GetActiveParticipantCount(ctx context.Context, callID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM call_participants WHERE call_id = $1 AND status = 'CONNECTED'", callID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
