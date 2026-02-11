package repository

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"sentinal-chat/internal/domain/command"
)

type commandRepository struct {
	db *gorm.DB
}

func NewCommandRepository(db *gorm.DB) CommandRepository {
	return &commandRepository{db: db}
}

func (r *commandRepository) CreateLog(ctx context.Context, log *command.CommandLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *commandRepository) UpdateLog(ctx context.Context, log *command.CommandLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

func (r *commandRepository) GetLogByID(ctx context.Context, id uuid.UUID) (command.CommandLog, error) {
	var log command.CommandLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	return log, err
}

func (r *commandRepository) GetPendingCommands(ctx context.Context, limit int) ([]command.CommandLog, error) {
	var logs []command.CommandLog
	err := r.db.WithContext(ctx).
		Where("status = ?", command.StatusPending).
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *commandRepository) GetCommandsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error) {
	var logs []command.CommandLog
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *commandRepository) CanUndo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) (bool, error) {
	var log command.CommandLog
	err := r.db.WithContext(ctx).
		First(&log, "id = ? AND user_id = ?", commandID, userID).Error
	if err != nil {
		return false, err
	}

	// Can undo if within 5 minutes and not already undone
	return log.Status == command.StatusCompleted && log.UndoneAt == nil, nil
}
