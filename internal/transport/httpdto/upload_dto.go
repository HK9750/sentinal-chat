package httpdto

import "time"

type CreateUploadSessionRequest struct {
	Filename  string `json:"filename" binding:"required,max=255"`
	MimeType  string `json:"mime_type" binding:"required,max=255"`
	SizeBytes int64  `json:"size_bytes" binding:"required,gt=0"`
	ChunkSize int    `json:"chunk_size" binding:"required,gt=0"`
}

type UpdateUploadProgressRequest struct {
	UploadedBytes int64 `json:"uploaded_bytes" binding:"required,min=0"`
}

type CreateAttachmentRequest struct {
	UploadSessionID string  `json:"upload_session_id" binding:"required,uuid"`
	MessageID       *string `json:"message_id,omitempty" binding:"omitempty,uuid"`
	EncryptedURL    string  `json:"encrypted_url" binding:"required,max=4096"`
	Filename        string  `json:"filename,omitempty"`
	MimeType        string  `json:"mime_type,omitempty" binding:"omitempty,max=255"`
	SizeBytes       *int64  `json:"size_bytes,omitempty" binding:"omitempty,gt=0"`
	ViewOnce        bool    `json:"view_once"`
	ThumbnailURL    string  `json:"thumbnail_url,omitempty"`
	Width           *int32  `json:"width,omitempty"`
	Height          *int32  `json:"height,omitempty"`
	DurationSeconds *int32  `json:"duration_seconds,omitempty"`
}

type UploadSessionPayload struct {
	ID            string     `json:"id"`
	UploaderID    string     `json:"uploader_id"`
	Filename      string     `json:"filename"`
	MimeType      string     `json:"mime_type"`
	SizeBytes     int64      `json:"size_bytes"`
	ChunkSize     int        `json:"chunk_size"`
	UploadedBytes int64      `json:"uploaded_bytes"`
	Status        string     `json:"status"`
	ObjectKey     string     `json:"object_key"`
	FileURL       *string    `json:"file_url,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type UploadTargetPayload struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type CreateUploadSessionPayload struct {
	Upload       UploadSessionPayload `json:"upload"`
	UploadTarget UploadTargetPayload  `json:"upload_target"`
}

type ListUploadSessionsPayload struct {
	Items  []UploadSessionPayload `json:"items"`
	Page   int                    `json:"page"`
	Limit  int                    `json:"limit"`
	Total  int64                  `json:"total"`
	Status string                 `json:"status,omitempty"`
}

type AttachmentPayload struct {
	ID              string     `json:"id"`
	UploaderID      *string    `json:"uploader_id,omitempty"`
	EncryptedURL    string     `json:"encrypted_url"`
	Filename        *string    `json:"filename,omitempty"`
	MimeType        string     `json:"mime_type"`
	SizeBytes       int64      `json:"size_bytes"`
	ViewOnce        bool       `json:"view_once"`
	ViewedAt        *time.Time `json:"viewed_at,omitempty"`
	ThumbnailURL    *string    `json:"thumbnail_url,omitempty"`
	Width           *int32     `json:"width,omitempty"`
	Height          *int32     `json:"height,omitempty"`
	DurationSeconds *int32     `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type MessageAttachmentsPayload struct {
	MessageID   string              `json:"message_id"`
	Attachments []AttachmentPayload `json:"attachments"`
}

type AttachmentViewedPayload struct {
	AttachmentID string    `json:"attachment_id"`
	Viewed       bool      `json:"viewed"`
	ViewedAt     time.Time `json:"viewed_at"`
}
