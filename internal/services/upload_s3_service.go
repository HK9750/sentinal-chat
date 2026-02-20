package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/repository"
	"sentinal-chat/internal/storage"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type UploadS3Service struct {
	repo    repository.UploadRepository
	storage *storage.Client
}

type PresignInput struct {
	UploaderID  uuid.UUID
	FileName    string
	ContentType string
	FileSize    int64
}

type PresignResult struct {
	Session   upload.UploadSession
	UploadURL string
	UploadKey string
	Headers   map[string]string
}

func NewUploadS3Service(repo repository.UploadRepository, storage *storage.Client) *UploadS3Service {
	return &UploadS3Service{repo: repo, storage: storage}
}

func (s *UploadS3Service) GetByID(ctx context.Context, id uuid.UUID) (upload.UploadSession, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadS3Service) Update(ctx context.Context, u upload.UploadSession) error {
	return s.repo.Update(ctx, u)
}

func (s *UploadS3Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *UploadS3Service) GetUserUploadSessions(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	return s.repo.GetUserUploadSessions(ctx, uploaderID)
}

func (s *UploadS3Service) GetInProgressUploads(ctx context.Context, uploaderID uuid.UUID) ([]upload.UploadSession, error) {
	return s.repo.GetInProgressUploads(ctx, uploaderID)
}

func (s *UploadS3Service) GetCompletedUploads(ctx context.Context, uploaderID uuid.UUID, page, limit int) ([]upload.UploadSession, int64, error) {
	return s.repo.GetCompletedUploads(ctx, uploaderID, page, limit)
}

func (s *UploadS3Service) UpdateProgress(ctx context.Context, sessionID uuid.UUID, uploadedBytes int64) error {
	return s.repo.UpdateProgress(ctx, sessionID, uploadedBytes)
}

func (s *UploadS3Service) MarkFailed(ctx context.Context, sessionID uuid.UUID) error {
	return s.repo.MarkFailed(ctx, sessionID)
}

func (s *UploadS3Service) GetStaleUploads(ctx context.Context, olderThan time.Duration) ([]upload.UploadSession, error) {
	return s.repo.GetStaleUploads(ctx, olderThan)
}

func (s *UploadS3Service) DeleteStaleUploads(ctx context.Context, olderThan time.Duration) (int64, error) {
	return s.repo.DeleteStaleUploads(ctx, olderThan)
}

func (s *UploadS3Service) CreatePresignedUpload(ctx context.Context, input PresignInput) (PresignResult, error) {
	if s.storage == nil {
		return PresignResult{}, errors.New("s3 storage is not configured")
	}
	if input.UploaderID == uuid.Nil || input.FileName == "" || input.ContentType == "" || input.FileSize <= 0 {
		return PresignResult{}, sentinal_errors.ErrInvalidInput
	}

	if err := s.storage.ValidateContentType(input.ContentType); err != nil {
		return PresignResult{}, sentinal_errors.ErrInvalidInput
	}

	session := upload.UploadSession{
		ID:            uuid.New(),
		UploaderID:    input.UploaderID,
		Filename:      input.FileName,
		MimeType:      input.ContentType,
		SizeBytes:     input.FileSize,
		ChunkSize:     0,
		UploadedBytes: 0,
		Status:        "IN_PROGRESS",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	key := buildObjectKey(session)
	session.ObjectKey = key

	presignedURL, headers, err := s.storage.PresignPut(ctx, key, input.ContentType, input.FileSize)
	if err != nil {
		return PresignResult{}, err
	}

	if err := s.repo.Create(ctx, &session); err != nil {
		return PresignResult{}, err
	}

	return PresignResult{
		Session:   session,
		UploadURL: presignedURL,
		UploadKey: key,
		Headers:   headers,
	}, nil
}

func (s *UploadS3Service) MarkCompletedWithS3(ctx context.Context, sessionID uuid.UUID) (upload.UploadSession, error) {
	if sessionID == uuid.Nil {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidInput
	}
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return upload.UploadSession{}, err
	}
	if session.Status == "COMPLETED" {
		return session, nil
	}
	if session.ObjectKey == "" {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidInput
	}

	fileURL := ""
	if s.storage != nil {
		fileURL = s.storage.FileURL(session.ObjectKey)
	}
	if fileURL != "" {
		session.FileURL = sql.NullString{String: fileURL, Valid: true}
	}
	session.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	session.Status = "COMPLETED"

	if err := s.repo.Update(ctx, session); err != nil {
		return upload.UploadSession{}, err
	}
	return session, nil
}

func buildObjectKey(session upload.UploadSession) string {
	ext := strings.ToLower(path.Ext(session.Filename))
	base := fmt.Sprintf("uploads/%s/%s", session.UploaderID.String(), session.ID.String())
	if ext == "" {
		return base
	}
	return base + ext
}

func hashFilename(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}
