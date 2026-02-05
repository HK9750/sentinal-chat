package httpdto

// CreateUploadRequest is used for POST /uploads
type CreateUploadRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	UploaderID  string `json:"uploader_id" binding:"required"`
}

// CreateUploadResponse is returned after creating an upload session
type CreateUploadResponse struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	UploaderID  string `json:"uploader_id"`
	Status      string `json:"status"`
	UploadURL   string `json:"upload_url,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// UpdateUploadRequest is used for PUT /uploads/:id
type UpdateUploadRequest struct {
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

// UpdateProgressRequest is used for PUT /uploads/:id/progress
type UpdateProgressRequest struct {
	UploadedBytes int64 `json:"uploaded_bytes" binding:"required"`
}

// ListUploadsRequest holds query parameters for listing uploads
type ListUploadsRequest struct {
	UploaderID string `form:"uploader_id" binding:"required"`
	Page       int    `form:"page"`
	Limit      int    `form:"limit"`
}

// ListUploadsResponse is returned when listing uploads
type ListUploadsResponse struct {
	Uploads []UploadDTO `json:"uploads"`
	Total   int64       `json:"total,omitempty"`
}

// UploadDTO represents an upload session in API responses
type UploadDTO struct {
	ID            string `json:"id"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	ContentType   string `json:"content_type"`
	UploaderID    string `json:"uploader_id"`
	Status        string `json:"status"`
	UploadedBytes int64  `json:"uploaded_bytes"`
	FileURL       string `json:"file_url,omitempty"`
	CreatedAt     string `json:"created_at"`
	CompletedAt   string `json:"completed_at,omitempty"`
}

// ListStaleUploadsRequest holds query parameters for listing stale uploads
type ListStaleUploadsRequest struct {
	OlderThanSec int `form:"older_than_sec" binding:"required"`
}

// DeleteStaleUploadsRequest holds query parameters for deleting stale uploads
type DeleteStaleUploadsRequest struct {
	OlderThanSec int `form:"older_than_sec" binding:"required"`
}

// DeleteStaleUploadsResponse is returned after deleting stale uploads
type DeleteStaleUploadsResponse struct {
	Deleted int64 `json:"deleted"`
}
