package services

import (
	"context"
	"time"

	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

type UploadService struct {
	repo repository.UploadRepository
}

func NewUploadService(repo repository.UploadRepository) *UploadService {
	return &UploadService{repo: repo}
}

func (s *UploadService) Create(ctx context.Context, u *upload.UploadSession) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return s.repo.Create(ctx, u)
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
	return s.repo.MarkCompleted(ctx, sessionID)
}

func (s *UploadService) MarkFailed(ctx context.Context, sessionID uuid.UUID) error {
	return s.repo.MarkFailed(ctx, sessionID)
}

func (s *UploadService) GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error) {
	return s.repo.GetStaleUploads(ctx, olderThan)
}

func (s *UploadService) DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error) {
	return s.repo.DeleteStaleUploads(ctx, olderThan)
}
