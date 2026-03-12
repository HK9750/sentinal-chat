package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"sentinal-chat/internal/domain/message"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"
)

const maxUploadSizeBytes int64 = 15 * 1024 * 1024

type UploadStorage interface {
	PutObject(ctx context.Context, key, contentType string, body io.Reader, sizeBytes int64) error
	FileURL(key string) string
	ValidateContentType(contentType string) error
}

type UploadService struct {
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	s3               UploadStorage
}

type CreateAttachmentInput struct {
	UploaderID      uuid.UUID
	MessageID       *uuid.UUID
	EncryptedURL    string
	Filename        string
	MimeType        string
	SizeBytes       int64
	ViewOnce        bool
	ThumbnailURL    string
	Width           *int32
	Height          *int32
	DurationSeconds *int32
}

type UploadFileInput struct {
	UploaderID uuid.UUID
	Filename   string
	MimeType   string
	SizeBytes  int64
	Body       io.Reader
}

type UploadedFile struct {
	Filename  string
	MimeType  string
	SizeBytes int64
	ObjectKey string
	FileURL   string
}

func NewUploadService(
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	s3 UploadStorage,
) *UploadService {
	return &UploadService{
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		s3:               s3,
	}
}

func (s *UploadService) UploadFile(ctx context.Context, in UploadFileInput) (UploadedFile, error) {
	if s == nil || s.s3 == nil {
		return UploadedFile{}, sentinal_errors.ErrServiceUnavailable
	}
	if err := validateUploadFileInput(in, s.s3); err != nil {
		return UploadedFile{}, err
	}

	objectKey := buildObjectKey(in.UploaderID, in.Filename)
	if err := s.s3.PutObject(ctx, objectKey, strings.TrimSpace(in.MimeType), in.Body, in.SizeBytes); err != nil {
		return UploadedFile{}, err
	}

	return UploadedFile{
		Filename:  strings.TrimSpace(in.Filename),
		MimeType:  strings.TrimSpace(in.MimeType),
		SizeBytes: in.SizeBytes,
		ObjectKey: objectKey,
		FileURL:   s.s3.FileURL(objectKey),
	}, nil
}

func (s *UploadService) UploadFiles(ctx context.Context, uploaderID uuid.UUID, files []UploadFileInput) ([]UploadedFile, error) {
	if s == nil || s.s3 == nil {
		return nil, sentinal_errors.ErrServiceUnavailable
	}
	if uploaderID == uuid.Nil {
		return nil, sentinal_errors.ErrUnauthorized
	}
	if len(files) == 0 || len(files) > 20 {
		return nil, sentinal_errors.ErrInvalidInput
	}

	results := make([]UploadedFile, len(files))
	errCh := make(chan error, len(files))
	var wg sync.WaitGroup

	for i := range files {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			if ctx.Err() != nil {
				errCh <- ctx.Err()
				return
			}

			input := files[idx]
			input.UploaderID = uploaderID

			result, err := s.UploadFile(ctx, input)
			if err != nil {
				errCh <- err
				return
			}

			results[idx] = result
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (s *UploadService) CreateAttachment(ctx context.Context, in CreateAttachmentInput) (message.Attachment, error) {
	if s == nil || s.messageRepo == nil {
		return message.Attachment{}, sentinal_errors.ErrServiceUnavailable
	}

	filename, mimeType, sizeBytes, err := normalizeAttachmentMetadata(in, s.s3)
	if err != nil {
		return message.Attachment{}, err
	}

	now := time.Now().UTC()
	attachment := &message.Attachment{
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
		msg, err := s.messageRepo.GetByID(ctx, *in.MessageID)
		if err != nil {
			return message.Attachment{}, err
		}
		if msg.SenderID != in.UploaderID {
			return message.Attachment{}, sentinal_errors.ErrForbidden
		}

		ma := &message.MessageAttachment{
			MessageID:    *in.MessageID,
			AttachmentID: attachment.ID,
		}
		if err := s.messageRepo.CreateAttachmentWithLink(ctx, attachment, ma); err != nil {
			return message.Attachment{}, err
		}
	} else {
		if err := s.messageRepo.CreateAttachment(ctx, attachment); err != nil {
			return message.Attachment{}, err
		}
	}

	return *attachment, nil
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

	canAccess, err := s.messageRepo.CanUserAccessAttachment(ctx, attachmentID, userID)
	if err != nil {
		return message.Attachment{}, err
	}
	if !canAccess {
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
		return attachment, nil
	}
	if attachment.ViewedAt.Valid {
		return attachment, nil
	}

	if err := s.messageRepo.MarkViewOnceViewed(ctx, attachmentID); err != nil {
		return message.Attachment{}, err
	}

	return s.messageRepo.GetAttachmentByID(ctx, attachmentID)
}

func (s *UploadService) GetMessageAttachments(ctx context.Context, userID, messageID uuid.UUID) ([]message.Attachment, error) {
	if s == nil || s.messageRepo == nil {
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

	if s.conversationRepo != nil {
		ok, err := s.conversationRepo.IsParticipant(ctx, msg.ConversationID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, sentinal_errors.ErrForbidden
		}
	}

	return s.messageRepo.GetMessageAttachments(ctx, messageID)
}

// --- private helpers ---

func validateUploadFileInput(in UploadFileInput, storage UploadStorage) error {
	if in.Body == nil {
		return sentinal_errors.ErrInvalidInput
	}
	return validateUploadedFileMetadata(in.UploaderID, in.Filename, in.MimeType, in.SizeBytes, storage)
}

func validateCreateAttachmentInput(in CreateAttachmentInput, storage UploadStorage) error {
	if in.UploaderID == uuid.Nil {
		return sentinal_errors.ErrUnauthorized
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

	if strings.TrimSpace(in.Filename) == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if in.SizeBytes <= 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if in.SizeBytes > maxUploadSizeBytes {
		return sentinal_errors.ErrTooLarge
	}

	return nil
}

func normalizeAttachmentMetadata(in CreateAttachmentInput, storage UploadStorage) (string, string, int64, error) {
	if err := validateCreateAttachmentInput(in, storage); err != nil {
		return "", "", 0, err
	}

	filename := strings.TrimSpace(in.Filename)
	mimeType := strings.TrimSpace(in.MimeType)
	sizeBytes := in.SizeBytes

	if storage != nil {
		if err := storage.ValidateContentType(mimeType); err != nil {
			return "", "", 0, sentinal_errors.ErrInvalidInput
		}
	} else if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		return "", "", 0, sentinal_errors.ErrInvalidInput
	}

	return filename, mimeType, sizeBytes, nil
}

func validateUploadedFileMetadata(uploaderID uuid.UUID, filename, mimeType string, sizeBytes int64, storage UploadStorage) error {
	if uploaderID == uuid.Nil {
		return sentinal_errors.ErrUnauthorized
	}
	filename = strings.TrimSpace(filename)
	mimeType = strings.TrimSpace(mimeType)
	if filename == "" || mimeType == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if sizeBytes <= 0 {
		return sentinal_errors.ErrInvalidInput
	}
	if sizeBytes > maxUploadSizeBytes {
		return sentinal_errors.ErrTooLarge
	}
	if storage != nil {
		if err := storage.ValidateContentType(mimeType); err != nil {
			return sentinal_errors.ErrInvalidInput
		}
	} else if _, _, err := mime.ParseMediaType(mimeType); err != nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
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
