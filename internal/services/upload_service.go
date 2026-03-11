package services

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/domain/upload"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type UploadStorage interface {
	PresignPut(ctx context.Context, key, contentType string, sizeBytes int64) (string, map[string]string, error)
	FileURL(key string) string
	ObjectExists(ctx context.Context, key string) (bool, error)
	ValidateContentType(contentType string) error
}

type UploadService struct {
	uploadRepo       repository.UploadRepository
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	s3               UploadStorage
}

type CreateUploadSessionInput struct {
	UploaderID uuid.UUID
	Filename   string
	MimeType   string
	SizeBytes  int64
	ChunkSize  int
}

type CreateUploadSessionOutput struct {
	Session upload.UploadSession
	URL     string
	Headers map[string]string
}

type ListUploadSessionsInput struct {
	UploaderID uuid.UUID
	Status     string
	Page       int
	Limit      int
}

type CreateAttachmentInput struct {
	UploaderID      uuid.UUID
	UploadSessionID uuid.UUID
	MessageID       *uuid.UUID
	EncryptedURL    string
	Filename        string
	MimeType        string
	SizeBytes       *int64
	ViewOnce        bool
	ThumbnailURL    string
	Width           *int32
	Height          *int32
	DurationSeconds *int32
}

func NewUploadService(
	uploadRepo repository.UploadRepository,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	s3 UploadStorage,
) *UploadService {
	return &UploadService{
		uploadRepo:       uploadRepo,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		s3:               s3,
	}
}

func (s *UploadService) CreateUploadSession(ctx context.Context, in CreateUploadSessionInput) (CreateUploadSessionOutput, error) {
	if s == nil || s.uploadRepo == nil || s.s3 == nil {
		return CreateUploadSessionOutput{}, sentinal_errors.ErrServiceUnavailable
	}
	if err := validateCreateUploadInput(in); err != nil {
		return CreateUploadSessionOutput{}, err
	}

	objectKey := buildObjectKey(in.UploaderID, in.Filename)
	presignedURL, headers, err := s.s3.PresignPut(ctx, objectKey, in.MimeType, in.SizeBytes)
	if err != nil {
		return CreateUploadSessionOutput{}, err
	}

	now := time.Now().UTC()
	session := upload.UploadSession{
		UploaderID:    in.UploaderID,
		Filename:      strings.TrimSpace(in.Filename),
		MimeType:      strings.TrimSpace(in.MimeType),
		SizeBytes:     in.SizeBytes,
		ChunkSize:     in.ChunkSize,
		UploadedBytes: 0,
		Status:        upload.StatusInProgress,
		ObjectKey:     objectKey,
		FileURL:       nullableString(s.s3.FileURL(objectKey)),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.uploadRepo.Create(ctx, &session); err != nil {
		return CreateUploadSessionOutput{}, err
	}

	return CreateUploadSessionOutput{
		Session: session,
		URL:     presignedURL,
		Headers: headers,
	}, nil
}

func (s *UploadService) GetUploadSession(ctx context.Context, uploaderID, sessionID uuid.UUID) (upload.UploadSession, error) {
	if s == nil || s.uploadRepo == nil {
		return upload.UploadSession{}, sentinal_errors.ErrServiceUnavailable
	}
	if uploaderID == uuid.Nil {
		return upload.UploadSession{}, sentinal_errors.ErrUnauthorized
	}
	if sessionID == uuid.Nil {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidInput
	}

	session, err := s.uploadRepo.GetByID(ctx, sessionID)
	if err != nil {
		return upload.UploadSession{}, err
	}
	if session.UploaderID != uploaderID {
		return upload.UploadSession{}, sentinal_errors.ErrForbidden
	}
	return session, nil
}

func (s *UploadService) ListUploadSessions(ctx context.Context, in ListUploadSessionsInput) ([]upload.UploadSession, int64, int, int, error) {
	if s == nil || s.uploadRepo == nil {
		return nil, 0, 0, 0, sentinal_errors.ErrServiceUnavailable
	}

	status, page, limit, err := normalizeListInput(in)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	if status == upload.StatusCompleted {
		items, total, err := s.uploadRepo.GetCompletedUploads(ctx, in.UploaderID, page, limit)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		return items, total, page, limit, nil
	}

	var rows []upload.UploadSession
	if status == upload.StatusInProgress {
		rows, err = s.uploadRepo.GetInProgressUploads(ctx, in.UploaderID)
	} else {
		rows, err = s.uploadRepo.GetUserUploadSessions(ctx, in.UploaderID)
	}
	if err != nil {
		return nil, 0, 0, 0, err
	}

	if status == upload.StatusFailed {
		rows = filterUploadsByStatus(rows, upload.StatusFailed)
	}

	items, total := paginate(rows, page, limit)
	return items, total, page, limit, nil
}

func (s *UploadService) UpdateUploadProgress(ctx context.Context, uploaderID, sessionID uuid.UUID, uploadedBytes int64) (upload.UploadSession, error) {
	if s == nil || s.uploadRepo == nil {
		return upload.UploadSession{}, sentinal_errors.ErrServiceUnavailable
	}
	if uploadedBytes < 0 {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidInput
	}

	session, err := s.GetUploadSession(ctx, uploaderID, sessionID)
	if err != nil {
		return upload.UploadSession{}, err
	}
	if session.Status != upload.StatusInProgress {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidTransition
	}
	if uploadedBytes < session.UploadedBytes || uploadedBytes > session.SizeBytes {
		return upload.UploadSession{}, sentinal_errors.ErrInvalidInput
	}

	if err := s.uploadRepo.UpdateProgress(ctx, sessionID, uploadedBytes); err != nil {
		return upload.UploadSession{}, err
	}
	return s.uploadRepo.GetByID(ctx, sessionID)
}

func (s *UploadService) CompleteUploadSession(ctx context.Context, uploaderID, sessionID uuid.UUID) (upload.UploadSession, error) {
	if s == nil || s.uploadRepo == nil || s.s3 == nil {
		return upload.UploadSession{}, sentinal_errors.ErrServiceUnavailable
	}

	session, err := s.GetUploadSession(ctx, uploaderID, sessionID)
	if err != nil {
		return upload.UploadSession{}, err
	}

	switch session.Status {
	case upload.StatusCompleted:
		return session, nil
	case upload.StatusInProgress:
		exists, err := s.s3.ObjectExists(ctx, session.ObjectKey)
		if err != nil {
			return upload.UploadSession{}, err
		}
		if !exists {
			return upload.UploadSession{}, sentinal_errors.ErrNotUploaded
		}
		if err := s.uploadRepo.MarkCompleted(ctx, sessionID); err != nil {
			return upload.UploadSession{}, err
		}
		return s.uploadRepo.GetByID(ctx, sessionID)
	default:
		return upload.UploadSession{}, sentinal_errors.ErrInvalidTransition
	}
}

func (s *UploadService) FailUploadSession(ctx context.Context, uploaderID, sessionID uuid.UUID) (upload.UploadSession, error) {
	if s == nil || s.uploadRepo == nil {
		return upload.UploadSession{}, sentinal_errors.ErrServiceUnavailable
	}

	session, err := s.GetUploadSession(ctx, uploaderID, sessionID)
	if err != nil {
		return upload.UploadSession{}, err
	}

	switch session.Status {
	case upload.StatusFailed:
		return session, nil
	case upload.StatusInProgress:
		if err := s.uploadRepo.MarkFailed(ctx, sessionID); err != nil {
			return upload.UploadSession{}, err
		}
		return s.uploadRepo.GetByID(ctx, sessionID)
	default:
		return upload.UploadSession{}, sentinal_errors.ErrInvalidTransition
	}
}

func (s *UploadService) CreateAttachment(ctx context.Context, in CreateAttachmentInput) (message.Attachment, error) {
	if s == nil || s.messageRepo == nil || s.uploadRepo == nil {
		return message.Attachment{}, sentinal_errors.ErrServiceUnavailable
	}

	session, err := s.GetUploadSession(ctx, in.UploaderID, in.UploadSessionID)
	if err != nil {
		return message.Attachment{}, err
	}
	if session.Status != upload.StatusCompleted {
		return message.Attachment{}, sentinal_errors.ErrNotUploaded
	}

	filename, mimeType, sizeBytes, err := normalizeAttachmentMetadata(in, session, s.s3)
	if err != nil {
		return message.Attachment{}, err
	}

	if in.MessageID != nil {
		msg, getErr := s.messageRepo.GetByID(ctx, *in.MessageID)
		if getErr != nil {
			return message.Attachment{}, getErr
		}
		if msg.SenderID != in.UploaderID {
			return message.Attachment{}, sentinal_errors.ErrForbidden
		}
		if msg.DeletedAt.Valid {
			return message.Attachment{}, sentinal_errors.ErrConflict
		}
	}

	now := time.Now().UTC()
	attachment := message.Attachment{
		ID:              uuid.New(),
		UploaderID:      uuid.NullUUID{UUID: in.UploaderID, Valid: true},
		EncryptedURL:    strings.TrimSpace(in.EncryptedURL),
		Filename:        nullableString(filename),
		MimeType:        mimeType,
		SizeBytes:       sizeBytes,
		ViewOnce:        in.ViewOnce,
		ThumbnailURL:    nullableString(strings.TrimSpace(in.ThumbnailURL)),
		Width:           nullableInt32(in.Width),
		Height:          nullableInt32(in.Height),
		DurationSeconds: nullableInt32(in.DurationSeconds),
		CreatedAt:       now,
	}

	if in.MessageID != nil {
		link := &message.MessageAttachment{MessageID: *in.MessageID, AttachmentID: attachment.ID}
		if err := s.messageRepo.CreateAttachmentWithLink(ctx, &attachment, link); err != nil {
			return message.Attachment{}, err
		}
		return attachment, nil
	}

	if err := s.messageRepo.CreateAttachment(ctx, &attachment); err != nil {
		return message.Attachment{}, err
	}
	return attachment, nil
}

func (s *UploadService) GetAttachment(ctx context.Context, userID, attachmentID uuid.UUID) (message.Attachment, error) {
	if s == nil || s.messageRepo == nil {
		return message.Attachment{}, sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil {
		return message.Attachment{}, sentinal_errors.ErrUnauthorized
	}
	if attachmentID == uuid.Nil {
		return message.Attachment{}, sentinal_errors.ErrInvalidInput
	}

	attachment, err := s.messageRepo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		return message.Attachment{}, err
	}

	if attachment.UploaderID.Valid && attachment.UploaderID.UUID == userID {
		return attachment, nil
	}

	allowed, err := s.messageRepo.CanUserAccessAttachment(ctx, attachmentID, userID)
	if err != nil {
		return message.Attachment{}, err
	}
	if !allowed {
		return message.Attachment{}, sentinal_errors.ErrForbidden
	}

	return attachment, nil
}

func (s *UploadService) MarkAttachmentViewed(ctx context.Context, userID, attachmentID uuid.UUID) (message.Attachment, error) {
	if s == nil || s.messageRepo == nil {
		return message.Attachment{}, sentinal_errors.ErrServiceUnavailable
	}

	attachment, err := s.GetAttachment(ctx, userID, attachmentID)
	if err != nil {
		return message.Attachment{}, err
	}
	if !attachment.ViewOnce {
		return message.Attachment{}, sentinal_errors.ErrInvalidInput
	}
	if attachment.ViewedAt.Valid {
		return attachment, nil
	}

	if err := s.messageRepo.MarkViewOnceViewed(ctx, attachmentID); err != nil {
		return message.Attachment{}, err
	}

	updated, err := s.messageRepo.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		return message.Attachment{}, err
	}
	return updated, nil
}

func (s *UploadService) GetMessageAttachments(ctx context.Context, userID, messageID uuid.UUID) ([]message.Attachment, error) {
	if s == nil || s.messageRepo == nil || s.conversationRepo == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}
	if userID == uuid.Nil {
		return nil, sentinal_errors.ErrUnauthorized
	}
	if messageID == uuid.Nil {
		return nil, sentinal_errors.ErrInvalidInput
	}

	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}

	allowed, err := s.conversationRepo.IsParticipant(ctx, msg.ConversationID, userID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, sentinal_errors.ErrForbidden
	}

	return s.messageRepo.GetMessageAttachments(ctx, messageID)
}

func validateCreateUploadInput(in CreateUploadSessionInput) error {
	if in.UploaderID == uuid.Nil {
		return sentinal_errors.ErrUnauthorized
	}
	in.Filename = strings.TrimSpace(in.Filename)
	in.MimeType = strings.TrimSpace(in.MimeType)
	if in.Filename == "" || in.MimeType == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if _, _, err := mime.ParseMediaType(in.MimeType); err != nil {
		return sentinal_errors.ErrInvalidInput
	}
	if in.SizeBytes <= 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if in.SizeBytes > upload.MaxUploadSizeBytes {
		return sentinal_errors.ErrTooLarge
	}
	if in.ChunkSize <= 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if int64(in.ChunkSize) > in.SizeBytes {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func validateCreateAttachmentInput(in CreateAttachmentInput, storage UploadStorage) error {
	if in.UploaderID == uuid.Nil {
		return sentinal_errors.ErrUnauthorized
	}
	if in.UploadSessionID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if in.Width != nil && *in.Width < 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if in.Height != nil && *in.Height < 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if in.DurationSeconds != nil && *in.DurationSeconds < 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if strings.TrimSpace(in.EncryptedURL) == "" {
		return sentinal_errors.ErrInvalidInput
	}

	if strings.TrimSpace(in.MimeType) != "" {
		if storage != nil {
			if err := storage.ValidateContentType(in.MimeType); err != nil {
				return sentinal_errors.ErrInvalidInput
			}
		} else if _, _, err := mime.ParseMediaType(in.MimeType); err != nil {
			return sentinal_errors.ErrInvalidInput
		}
	}

	if in.SizeBytes != nil {
		if *in.SizeBytes <= 0 {
			return sentinal_errors.ErrInvalidInput
		}
		if *in.SizeBytes > upload.MaxUploadSizeBytes {
			return sentinal_errors.ErrTooLarge
		}
	}

	return nil
}

func normalizeAttachmentMetadata(in CreateAttachmentInput, session upload.UploadSession, storage UploadStorage) (string, string, int64, error) {
	if err := validateCreateAttachmentInput(in, storage); err != nil {
		return "", "", 0, err
	}

	filename := strings.TrimSpace(in.Filename)
	if filename == "" {
		filename = session.Filename
	} else if filename != session.Filename {
		return "", "", 0, sentinal_errors.ErrConflict
	}

	mimeType := strings.TrimSpace(in.MimeType)
	if mimeType == "" {
		mimeType = session.MimeType
	} else if mimeType != session.MimeType {
		return "", "", 0, sentinal_errors.ErrConflict
	}

	sizeBytes := session.SizeBytes
	if in.SizeBytes != nil {
		if *in.SizeBytes != session.SizeBytes {
			return "", "", 0, sentinal_errors.ErrConflict
		}
		sizeBytes = *in.SizeBytes
	}

	if storage != nil {
		if err := storage.ValidateContentType(mimeType); err != nil {
			return "", "", 0, sentinal_errors.ErrInvalidInput
		}
	} else if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		return "", "", 0, sentinal_errors.ErrInvalidInput
	}

	return filename, mimeType, sizeBytes, nil
}

func normalizeListInput(in ListUploadSessionsInput) (string, int, int, error) {
	if in.UploaderID == uuid.Nil {
		return "", 0, 0, sentinal_errors.ErrUnauthorized
	}

	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if !isValidUploadStatus(status) {
		return "", 0, 0, sentinal_errors.ErrInvalidInput
	}

	page := in.Page
	if page <= 0 {
		page = 1
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	return status, page, limit, nil
}

func filterUploadsByStatus(sessions []upload.UploadSession, status string) []upload.UploadSession {
	if status == "" {
		return sessions
	}

	filtered := make([]upload.UploadSession, 0, len(sessions))
	for _, row := range sessions {
		if row.Status == status {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func buildObjectKey(uploaderID uuid.UUID, filename string) string {
	now := time.Now().UTC()
	cleanName := sanitizeFilename(filename)
	return fmt.Sprintf(
		"uploads/%s/%04d/%02d/%02d/%s-%s",
		uploaderID.String(),
		now.Year(),
		now.Month(),
		now.Day(),
		uuid.NewString(),
		cleanName,
	)
}

func sanitizeFilename(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "file.bin"
	}

	var b strings.Builder
	b.Grow(len(base))
	for _, r := range base {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	clean := strings.Trim(b.String(), "._-")
	if clean == "" {
		return "file.bin"
	}
	if len(clean) > 160 {
		clean = clean[:160]
	}
	return clean
}

func nullableString(v string) sql.NullString {
	if strings.TrimSpace(v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullableInt32(v *int32) sql.NullInt32 {
	if v == nil {
		return sql.NullInt32{}
	}
	if *v < 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: *v, Valid: true}
}

func paginate[T any](items []T, page, limit int) ([]T, int64) {
	total := int64(len(items))
	if total == 0 {
		return []T{}, 0
	}

	start := (page - 1) * limit
	if start >= len(items) {
		return []T{}, total
	}

	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	out := make([]T, end-start)
	copy(out, items[start:end])
	return out, total
}

var validUploadStatuses = []string{
	upload.StatusInProgress,
	upload.StatusCompleted,
	upload.StatusFailed,
}

func isValidUploadStatus(status string) bool {
	return status == "" || slices.Contains(validUploadStatuses, status)
}
