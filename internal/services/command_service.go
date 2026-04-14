package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/command"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type CommandService struct {
	repo repository.CommandRepository
}

func NewCommandService(repo repository.CommandRepository) *CommandService {
	return &CommandService{repo: repo}
}

func (s *CommandService) Record(ctx context.Context, commandType command.CommandType, userID uuid.UUID, conversationID *uuid.UUID, payload any, undoPayload any) (*command.CommandLog, error) {
	if s == nil || s.repo == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var undoBytes []byte
	if undoPayload != nil {
		undoBytes, err = json.Marshal(undoPayload)
		if err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	log := &command.CommandLog{
		ID:             uuid.New(),
		CommandType:    commandType,
		UserID:         userID,
		ConversationID: conversationID,
		Status:         command.StatusExecuted,
		Payload:        payloadBytes,
		UndoPayload:    undoBytes,
		CreatedAt:      now,
		ExecutedAt:     &now,
	}
	if err := s.repo.CreateLog(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *CommandService) GetByID(ctx context.Context, commandID uuid.UUID) (command.CommandLog, error) {
	if s == nil || s.repo == nil {
		return command.CommandLog{}, sentinal_errors.ErrServiceUnavailable
	}
	return s.repo.GetLogByID(ctx, commandID)
}

func (s *CommandService) GetLatestUndoable(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID) (command.CommandLog, error) {
	if s == nil || s.repo == nil {
		return command.CommandLog{}, sentinal_errors.ErrServiceUnavailable
	}
	return s.repo.GetLatestUndoableByUser(ctx, userID, conversationID)
}

func (s *CommandService) GetLatestRedoable(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID) (command.CommandLog, error) {
	if s == nil || s.repo == nil {
		return command.CommandLog{}, sentinal_errors.ErrServiceUnavailable
	}
	return s.repo.GetLatestRedoableByUser(ctx, userID, conversationID)
}

func (s *CommandService) MarkUndone(ctx context.Context, log *command.CommandLog) error {
	if s == nil || s.repo == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if log == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if log.Status != command.StatusExecuted {
		return sentinal_errors.ErrConflict
	}
	now := time.Now().UTC()
	log.Status = command.StatusUndone
	log.UndoneAt = &now
	log.ErrorMessage = ""
	return s.repo.UpdateLog(ctx, log)
}

func (s *CommandService) MarkRedone(ctx context.Context, log *command.CommandLog) error {
	if s == nil || s.repo == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if log == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if log.Status != command.StatusUndone || log.UndoneAt == nil {
		return sentinal_errors.ErrConflict
	}
	now := time.Now().UTC()
	log.Status = command.StatusExecuted
	log.ExecutedAt = &now
	log.UndoneAt = nil
	log.ErrorMessage = ""
	return s.repo.UpdateLog(ctx, log)
}

func (s *CommandService) MarkFailed(ctx context.Context, log *command.CommandLog, err error) error {
	if s == nil || s.repo == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	if log == nil {
		return sentinal_errors.ErrInvalidInput
	}
	log.Status = command.StatusFailed
	if err != nil {
		log.ErrorMessage = err.Error()
	}
	return s.repo.UpdateLog(ctx, log)
}
