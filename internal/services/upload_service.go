package services

import (
	"context"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type UploadService struct {
	repo      repository.UploadRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewUploadService(repo repository.UploadRepository, eventRepo repository.EventRepository, bus *commands.Bus) *UploadService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &UploadService{repo: repo, eventRepo: eventRepo, bus: bus}
	svc.RegisterHandlers(bus)
	return svc
}

func (s *UploadService) RegisterHandlers(bus *commands.Bus) {
	if bus == nil {
		return
	}

	// upload.create - Create a new upload session
	bus.Register("upload.create", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.CreateUploadSessionCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		u := &upload.UploadSession{
			ID:         uuid.New(),
			UploaderID: c.UploaderID,
			Filename:   c.Filename,
			MimeType:   c.MimeType,
			SizeBytes:  c.SizeBytes,
			ChunkSize:  c.ChunkSize,
			Status:     "IN_PROGRESS",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.Create(ctx, u); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: u.ID.String(), Payload: u}, nil
	}))

	// upload.progress - Update upload progress
	bus.Register("upload.progress", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.UpdateUploadProgressCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.SessionID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.UploaderID != c.UploaderID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.UpdateProgress(ctx, c.SessionID, c.UploadedBytes); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "upload", "upload.progress", c.SessionID, map[string]any{
			"upload_id":      c.SessionID,
			"uploaded_bytes": c.UploadedBytes,
			"total_bytes":    existing.SizeBytes,
		})
		return commands.Result{AggregateID: c.SessionID.String()}, nil
	}))

	// upload.complete - Mark upload as completed
	bus.Register("upload.complete", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.CompleteUploadCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.SessionID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.UploaderID != c.UploaderID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.MarkCompleted(ctx, c.SessionID); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.SessionID.String()}, nil
	}))

	// upload.fail - Mark upload as failed
	bus.Register("upload.fail", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.FailUploadCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.SessionID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.UploaderID != c.UploaderID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.MarkFailed(ctx, c.SessionID); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.SessionID.String()}, nil
	}))

	// upload.delete - Delete an upload session
	bus.Register("upload.delete", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.DeleteUploadCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		existing, err := s.GetByID(ctx, c.SessionID)
		if err != nil {
			return commands.Result{}, err
		}
		if existing.UploaderID != c.UploaderID {
			return commands.Result{}, sentinal_errors.ErrForbidden
		}
		if err := s.Delete(ctx, c.SessionID); err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "upload", "upload.deleted", c.SessionID, map[string]any{"upload_id": c.SessionID})
		return commands.Result{AggregateID: c.SessionID.String()}, nil
	}))
}

func (s *UploadService) Create(ctx context.Context, u *upload.UploadSession) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "upload", "upload.created", u.ID, u)
}

func (s *UploadService) GetByID(ctx context.Context, id uuid.UUID) (upload.UploadSession, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) Update(ctx context.Context, u upload.UploadSession) error {
	return s.repo.Update(ctx, u)
}

func (s *UploadService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *UploadService) GetUserUploadSessions(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	return s.repo.GetUserUploadSessions(ctx, uploaderID)
}

func (s *UploadService) GetInProgressUploads(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	return s.repo.GetInProgressUploads(ctx, uploaderID)
}

func (s *UploadService) GetCompletedUploads(ctx context.Context, uploaderID uuid.UUID, page, limit int) ([]upload.UploadSession, int64, error) {
	return s.repo.GetCompletedUploads(ctx, uploaderID, page, limit)
}

func (s *UploadService) UpdateProgress(ctx context.Context, sessionID uuid.UUID, uploadedBytes int64) error {
	return s.repo.UpdateProgress(ctx, sessionID, uploadedBytes)
}

func (s *UploadService) MarkCompleted(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.repo.MarkCompleted(ctx, sessionID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "upload", "upload.completed", sessionID, map[string]any{"upload_id": sessionID})
}

func (s *UploadService) MarkFailed(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.repo.MarkFailed(ctx, sessionID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "upload", "upload.failed", sessionID, map[string]any{"upload_id": sessionID})
}

func (s *UploadService) GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error) {
	return s.repo.GetStaleUploads(ctx, olderThan)
}

func (s *UploadService) DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error) {
	return s.repo.DeleteStaleUploads(ctx, olderThan)
}
