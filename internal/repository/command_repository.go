package repository

import (
	"context"

	"github.com/google/uuid"
	"sentinal-chat/internal/domain/command"
)

type commandRepository struct {
	db DBTX
}

func NewCommandRepository(db DBTX) CommandRepository {
	return &commandRepository{db: db}
}

func (r *commandRepository) CreateLog(ctx context.Context, log *command.CommandLog) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO command_logs (id, command_type, user_id, status, payload, result, undo_data, error_message, execution_time_ms, created_at, executed_at, undone_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
    `,
		log.ID,
		log.CommandType,
		log.UserID,
		log.Status,
		log.Payload,
		log.Result,
		log.UndoData,
		log.ErrorMessage,
		log.ExecutionTimeMs,
		log.CreatedAt,
		log.ExecutedAt,
		log.UndoneAt,
	)
	return err
}

func (r *commandRepository) UpdateLog(ctx context.Context, log *command.CommandLog) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE command_logs
        SET command_type = $1, user_id = $2, status = $3, payload = $4, result = $5, undo_data = $6,
            error_message = $7, execution_time_ms = $8, created_at = $9, executed_at = $10, undone_at = $11
        WHERE id = $12
    `,
		log.CommandType,
		log.UserID,
		log.Status,
		log.Payload,
		log.Result,
		log.UndoData,
		log.ErrorMessage,
		log.ExecutionTimeMs,
		log.CreatedAt,
		log.ExecutedAt,
		log.UndoneAt,
		log.ID,
	)
	return err
}

func (r *commandRepository) GetLogByID(ctx context.Context, id uuid.UUID) (command.CommandLog, error) {
	var log command.CommandLog
	err := r.db.QueryRowContext(ctx, `
        SELECT id, command_type, user_id, status, payload, result, undo_data, error_message, execution_time_ms,
               created_at, executed_at, undone_at
        FROM command_logs WHERE id = $1
    `, id).Scan(
		&log.ID,
		&log.CommandType,
		&log.UserID,
		&log.Status,
		&log.Payload,
		&log.Result,
		&log.UndoData,
		&log.ErrorMessage,
		&log.ExecutionTimeMs,
		&log.CreatedAt,
		&log.ExecutedAt,
		&log.UndoneAt,
	)
	if err != nil {
		return command.CommandLog{}, err
	}
	return log, nil
}

func (r *commandRepository) GetPendingCommands(ctx context.Context, limit int) ([]command.CommandLog, error) {
	var logs []command.CommandLog
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, command_type, user_id, status, payload, result, undo_data, error_message, execution_time_ms,
               created_at, executed_at, undone_at
        FROM command_logs WHERE status = $1
        LIMIT $2
    `, command.StatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var log command.CommandLog
		if err := rows.Scan(
			&log.ID,
			&log.CommandType,
			&log.UserID,
			&log.Status,
			&log.Payload,
			&log.Result,
			&log.UndoData,
			&log.ErrorMessage,
			&log.ExecutionTimeMs,
			&log.CreatedAt,
			&log.ExecutedAt,
			&log.UndoneAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *commandRepository) GetCommandsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error) {
	var logs []command.CommandLog
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, command_type, user_id, status, payload, result, undo_data, error_message, execution_time_ms,
               created_at, executed_at, undone_at
        FROM command_logs
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2
    `, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var log command.CommandLog
		if err := rows.Scan(
			&log.ID,
			&log.CommandType,
			&log.UserID,
			&log.Status,
			&log.Payload,
			&log.Result,
			&log.UndoData,
			&log.ErrorMessage,
			&log.ExecutionTimeMs,
			&log.CreatedAt,
			&log.ExecutedAt,
			&log.UndoneAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *commandRepository) CanUndo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) (bool, error) {
	var log command.CommandLog
	err := r.db.QueryRowContext(ctx, `
        SELECT id, command_type, user_id, status, payload, result, undo_data, error_message, execution_time_ms,
               created_at, executed_at, undone_at
        FROM command_logs WHERE id = $1 AND user_id = $2
    `, commandID, userID).Scan(
		&log.ID,
		&log.CommandType,
		&log.UserID,
		&log.Status,
		&log.Payload,
		&log.Result,
		&log.UndoData,
		&log.ErrorMessage,
		&log.ExecutionTimeMs,
		&log.CreatedAt,
		&log.ExecutedAt,
		&log.UndoneAt,
	)
	if err != nil {
		return false, err
	}
	return log.Status == command.StatusCompleted && log.UndoneAt == nil, nil
}
