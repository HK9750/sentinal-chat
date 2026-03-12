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
