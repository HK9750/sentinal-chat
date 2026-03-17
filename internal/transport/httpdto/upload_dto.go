package httpdto

import "time"

type CreateAttachmentRequest struct {
	MessageID       *string `json:"message_id,omitempty" binding:"omitempty,uuid"`
	FileURL         string  `json:"file_url" binding:"required,max=4096"`
	Filename        string  `json:"filename" binding:"required,max=255"`
	MimeType        string  `json:"mime_type" binding:"required,max=255"`
	SizeBytes       int64   `json:"size_bytes" binding:"required,gt=0"`
	ViewOnce        bool    `json:"view_once"`
	ThumbnailURL    string  `json:"thumbnail_url,omitempty"`
	Width           *int32  `json:"width,omitempty"`
	Height          *int32  `json:"height,omitempty"`
	DurationSeconds *int32  `json:"duration_seconds,omitempty"`
}

type UploadFilePayload struct {
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	ObjectKey string `json:"object_key"`
	FileURL   string `json:"file_url,omitempty"`
}

type UploadFilesPayload struct {
	Items []UploadFilePayload `json:"items"`
}

type AttachmentPayload struct {
	ID              string     `json:"id"`
	UploaderID      *string    `json:"uploader_id,omitempty"`
	FileURL         string     `json:"file_url"`
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
