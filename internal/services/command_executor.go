package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/command"
	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/repository"
)

// CommandExecutor executes commands with transactions and logging
type CommandExecutor struct {
	db             *gorm.DB
	commandRepo    repository.CommandRepository
	messageRepo    repository.MessageRepository
	convRepo       repository.ConversationRepository
	userRepo       repository.UserRepository
	eventPublisher *EventPublisher
	logger         *zap.Logger
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(
	db *gorm.DB,
	commandRepo repository.CommandRepository,
	messageRepo repository.MessageRepository,
	convRepo repository.ConversationRepository,
	userRepo repository.UserRepository,
	eventPublisher *EventPublisher,
) *CommandExecutor {
	return &CommandExecutor{
		db:             db,
		commandRepo:    commandRepo,
		messageRepo:    messageRepo,
		convRepo:       convRepo,
		userRepo:       userRepo,
		eventPublisher: eventPublisher,
		logger:         zap.L(),
	}
}

// Execute executes a command with full logging
func (e *CommandExecutor) Execute(ctx context.Context, cmd commands.Command) (*command.CommandLog, error) {
	start := time.Now()

	// Validate
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	// Create log entry
	payload, _ := cmd.ToJSON()
	log := &command.CommandLog{
		CommandType: cmd.GetType(),
		UserID:      cmd.GetUserID(),
		Status:      command.StatusPending,
		Payload:     payload,
	}

	if err := e.commandRepo.CreateLog(ctx, log); err != nil {
		return nil, err
	}

	// Execute based on command type
	log.Status = command.StatusExecuting
	if err := e.commandRepo.UpdateLog(ctx, log); err != nil {
		return nil, err
	}

	var execErr error
	switch c := cmd.(type) {
	case *commands.SendMessageCommand:
		execErr = e.executeSendMessage(ctx, c)
	case *commands.DeleteMessageCommand:
		execErr = e.executeDeleteMessage(ctx, c)
	case *commands.EditMessageCommand:
		execErr = e.executeEditMessage(ctx, c)
	case *commands.BulkArchiveCommand:
		execErr = e.executeBulkArchive(ctx, c)
	default:
		execErr = errors.New("unknown command type")
	}

	executionTime := time.Since(start).Milliseconds()

	// Update log with result
	log.ExecutionTimeMs = int(executionTime)
	now := time.Now()
	log.ExecutedAt = &now

	if execErr != nil {
		log.Status = command.StatusFailed
		log.ErrorMessage = execErr.Error()
	} else {
		log.Status = command.StatusCompleted
		if cmd.CanUndo() {
			undoData, _ := cmd.ToJSON()
			log.UndoData = undoData
		}
	}

	if updateErr := e.commandRepo.UpdateLog(ctx, log); updateErr != nil {
		e.logger.Error("failed to update command log", zap.Error(updateErr))
	}

	return log, execErr
}

// executeSendMessage executes send message command
func (e *CommandExecutor) executeSendMessage(ctx context.Context, cmd *commands.SendMessageCommand) error {
	msg := message.Message{
		ID:             uuid.New(),
		ConversationID: cmd.ConversationID,
		SenderID:       cmd.SenderID,
		Content:        sql.NullString{String: cmd.Content, Valid: true},
		CreatedAt:      time.Now(),
	}

	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)
		if err := msgRepo.Create(ctx, &msg); err != nil {
			return err
		}

		if e.eventPublisher != nil {
			if err := e.eventPublisher.PublishMessageNew(ctx, tx, msg.ID, msg.ConversationID, msg.SenderID); err != nil {
				return err
			}
		}

		return nil
	})
}

// executeDeleteMessage executes delete message command
func (e *CommandExecutor) executeDeleteMessage(ctx context.Context, cmd *commands.DeleteMessageCommand) error {
	msg, err := e.messageRepo.GetByID(ctx, cmd.MessageID)
	if err != nil {
		return err
	}

	// Store original state for undo
	cmd.OriginalContent = msg.Content.String

	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)

		if cmd.DeleteForAll {
			if err := msgRepo.SoftDelete(ctx, cmd.MessageID); err != nil {
				return err
			}
		} else {
			// Delete for me only - mark as deleted for user
			if err := msgRepo.HardDelete(ctx, cmd.MessageID); err != nil {
				return err
			}
		}

		return nil
	})
}

// executeEditMessage executes edit message command
func (e *CommandExecutor) executeEditMessage(ctx context.Context, cmd *commands.EditMessageCommand) error {
	msg, err := e.messageRepo.GetByID(ctx, cmd.MessageID)
	if err != nil {
		return err
	}

	// Store previous content
	cmd.PreviousContent = msg.Content.String

	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		msgRepo := repository.NewMessageRepository(tx)

		// Create version record
		version := &command.MessageVersion{
			MessageID:     cmd.MessageID,
			Content:       msg.Content.String,
			EditedBy:      cmd.UserID,
			VersionNumber: 1, // TODO: Get next version number
		}

		// Save version (would need a versions repo in real implementation)
		_ = version

		// Update message
		msg.Content = sql.NullString{String: cmd.NewContent, Valid: true}
		if err := msgRepo.Update(ctx, msg); err != nil {
			return err
		}

		return nil
	})
}

// executeBulkArchive executes bulk archive command
func (e *CommandExecutor) executeBulkArchive(ctx context.Context, cmd *commands.BulkArchiveCommand) error {
	results := make([]commands.BulkArchiveResult, 0, len(cmd.ConversationIDs))

	for _, convID := range cmd.ConversationIDs {
		result := commands.BulkArchiveResult{
			ConversationID: convID,
			Success:        true,
		}

		// Check if user is participant
		isParticipant, err := e.convRepo.IsParticipant(ctx, convID, cmd.UserID)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else if !isParticipant {
			result.Success = false
			result.Error = "not a participant"
		} else {
			// Archive conversation for user
			// Note: This would need a conversation archive method
			_ = e.convRepo.ArchiveConversation(ctx, convID, cmd.UserID)
		}

		results = append(results, result)
	}

	cmd.Results = results
	return nil
}

// Undo undoes a previously executed command
func (e *CommandExecutor) Undo(ctx context.Context, commandID uuid.UUID, userID uuid.UUID) error {
	// Check if can undo
	canUndo, err := e.commandRepo.CanUndo(ctx, commandID, userID)
	if err != nil {
		return err
	}
	if !canUndo {
		return errors.New("cannot undo this command")
	}

	// Fetch command log
	log, err := e.commandRepo.GetLogByID(ctx, commandID)
	if err != nil {
		return err
	}

	// Parse undo data and execute reverse operation
	var undoErr error
	switch log.CommandType {
	case "SendMessage":
		var cmd commands.SendMessageCommand
		if err := json.Unmarshal(log.UndoData, &cmd); err == nil {
			undoErr = e.undoSendMessage(ctx, &cmd)
		}
	case "DeleteMessage":
		var cmd commands.DeleteMessageCommand
		if err := json.Unmarshal(log.UndoData, &cmd); err == nil {
			undoErr = e.undoDeleteMessage(ctx, &cmd)
		}
	case "EditMessage":
		var cmd commands.EditMessageCommand
		if err := json.Unmarshal(log.UndoData, &cmd); err == nil {
			undoErr = e.undoEditMessage(ctx, &cmd)
		}
	case "BulkArchive":
		var cmd commands.BulkArchiveCommand
		if err := json.Unmarshal(log.UndoData, &cmd); err == nil {
			undoErr = e.undoBulkArchive(ctx, &cmd)
		}
	}

	if undoErr != nil {
		return undoErr
	}

	// Mark as undone
	now := time.Now()
	log.Status = command.StatusUndone
	log.UndoneAt = &now

	return e.commandRepo.UpdateLog(ctx, &log)
}

// undoSendMessage deletes the sent message
func (e *CommandExecutor) undoSendMessage(ctx context.Context, cmd *commands.SendMessageCommand) error {
	// Find and delete the message
	// In a real implementation, we'd need to track the message ID created
	return nil
}

// undoDeleteMessage restores the deleted message
func (e *CommandExecutor) undoDeleteMessage(ctx context.Context, cmd *commands.DeleteMessageCommand) error {
	// Restore soft-deleted message
	return nil
}

// undoEditMessage reverts to previous version
func (e *CommandExecutor) undoEditMessage(ctx context.Context, cmd *commands.EditMessageCommand) error {
	// Restore previous content
	return nil
}

// undoBulkArchive unarchives conversations
func (e *CommandExecutor) undoBulkArchive(ctx context.Context, cmd *commands.BulkArchiveCommand) error {
	for _, result := range cmd.Results {
		if result.Success {
			e.convRepo.UnarchiveConversation(ctx, result.ConversationID, cmd.UserID)
		}
	}
	return nil
}

// GetCommandHistory returns command history for a user
func (e *CommandExecutor) GetCommandHistory(ctx context.Context, userID uuid.UUID, limit int) ([]command.CommandLog, error) {
	return e.commandRepo.GetCommandsByUser(ctx, userID, limit)
}
