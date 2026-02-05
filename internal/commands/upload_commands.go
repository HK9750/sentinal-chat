package commands

import (
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

// CreateUploadSessionCommand creates a new upload session
type CreateUploadSessionCommand struct {
	UploaderID          uuid.UUID
	Filename            string
	MimeType            string
	SizeBytes           int64
	ChunkSize           int
	IdempotencyKeyValue string
}

func (CreateUploadSessionCommand) CommandType() string { return "upload.create" }

func (c CreateUploadSessionCommand) Validate() error {
	if c.UploaderID == uuid.Nil || c.Filename == "" || c.MimeType == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if c.SizeBytes <= 0 || c.ChunkSize <= 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CreateUploadSessionCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CreateUploadSessionCommand) ActorID() uuid.UUID { return c.UploaderID }

// UpdateUploadProgressCommand updates upload progress
type UpdateUploadProgressCommand struct {
	SessionID           uuid.UUID
	UploaderID          uuid.UUID
	UploadedBytes       int64
	IdempotencyKeyValue string
}

func (UpdateUploadProgressCommand) CommandType() string { return "upload.progress" }

func (c UpdateUploadProgressCommand) Validate() error {
	if c.SessionID == uuid.Nil || c.UploaderID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	if c.UploadedBytes < 0 {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c UpdateUploadProgressCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c UpdateUploadProgressCommand) ActorID() uuid.UUID { return c.UploaderID }

// CompleteUploadCommand marks upload as completed
type CompleteUploadCommand struct {
	SessionID           uuid.UUID
	UploaderID          uuid.UUID
	IdempotencyKeyValue string
}

func (CompleteUploadCommand) CommandType() string { return "upload.complete" }

func (c CompleteUploadCommand) Validate() error {
	if c.SessionID == uuid.Nil || c.UploaderID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c CompleteUploadCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c CompleteUploadCommand) ActorID() uuid.UUID { return c.UploaderID }

// FailUploadCommand marks upload as failed
type FailUploadCommand struct {
	SessionID           uuid.UUID
	UploaderID          uuid.UUID
	ErrorMessage        string
	IdempotencyKeyValue string
}

func (FailUploadCommand) CommandType() string { return "upload.fail" }

func (c FailUploadCommand) Validate() error {
	if c.SessionID == uuid.Nil || c.UploaderID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c FailUploadCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c FailUploadCommand) ActorID() uuid.UUID { return c.UploaderID }

// DeleteUploadCommand deletes an upload session
type DeleteUploadCommand struct {
	SessionID           uuid.UUID
	UploaderID          uuid.UUID
	IdempotencyKeyValue string
}

func (DeleteUploadCommand) CommandType() string { return "upload.delete" }

func (c DeleteUploadCommand) Validate() error {
	if c.SessionID == uuid.Nil || c.UploaderID == uuid.Nil {
		return sentinal_errors.ErrInvalidInput
	}
	return nil
}

func (c DeleteUploadCommand) IdempotencyKey() string { return c.IdempotencyKeyValue }

func (c DeleteUploadCommand) ActorID() uuid.UUID { return c.UploaderID }
