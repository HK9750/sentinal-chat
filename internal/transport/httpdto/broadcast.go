package httpdto

import (
	"sentinal-chat/internal/domain/broadcast"
	"time"
)

// CreateBroadcastRequest is used for POST /broadcasts
type CreateBroadcastRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description,omitempty"`
	Recipients  []string `json:"recipients,omitempty"`
}

// CreateBroadcastResponse is returned after creating a broadcast
type CreateBroadcastResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
}

// UpdateBroadcastRequest is used for PUT /broadcasts/:id
type UpdateBroadcastRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// AddRecipientRequest is used for POST /broadcasts/:id/recipients
type AddRecipientRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// BulkRecipientsRequest is used for bulk add/remove recipients
type BulkRecipientsRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}

// BulkRecipientsResponse is returned after bulk operations
type BulkRecipientsResponse struct {
	Count int `json:"count"`
}

// ListBroadcastsRequest holds query parameters for listing broadcasts
type ListBroadcastsRequest struct {
	OwnerID string `form:"owner_id" binding:"required"`
}

// ListBroadcastsResponse is returned when listing broadcasts
type ListBroadcastsResponse struct {
	Broadcasts []BroadcastDTO `json:"broadcasts"`
}

// BroadcastDTO represents a broadcast list in API responses
type BroadcastDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	OwnerID        string `json:"owner_id"`
	RecipientCount int    `json:"recipient_count"`
	CreatedAt      string `json:"created_at"`
}

// SearchBroadcastsRequest holds query parameters for searching broadcasts
type SearchBroadcastsRequest struct {
	OwnerID string `form:"owner_id" binding:"required"`
	Query   string `form:"query" binding:"required"`
}

// RecipientsResponse is returned when listing recipients
type RecipientsResponse struct {
	Recipients []RecipientDTO `json:"recipients"`
}

// RecipientDTO represents a broadcast recipient in API responses
type RecipientDTO struct {
	UserID    string `json:"user_id"`
	AddedAt   string `json:"added_at"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// RecipientCountResponse is returned when getting recipient count
type RecipientCountResponse struct {
	Count int64 `json:"count"`
}

// IsRecipientResponse is returned when checking if user is a recipient
type IsRecipientResponse struct {
	IsRecipient bool `json:"is_recipient"`
}

// FromBroadcastList converts a domain broadcast list to BroadcastDTO
func FromBroadcastList(b broadcast.BroadcastList) BroadcastDTO {
	dto := BroadcastDTO{
		ID:        b.ID.String(),
		Name:      b.Name,
		OwnerID:   b.OwnerID.String(),
		CreatedAt: b.CreatedAt.Format(time.RFC3339),
	}
	if b.Description.Valid {
		dto.Description = b.Description.String
	}
	return dto
}

// FromBroadcastListSlice converts a slice of domain broadcast lists to BroadcastDTO slice
func FromBroadcastListSlice(lists []broadcast.BroadcastList) []BroadcastDTO {
	dtos := make([]BroadcastDTO, len(lists))
	for i, b := range lists {
		dtos[i] = FromBroadcastList(b)
	}
	return dtos
}

// FromBroadcastRecipient converts a domain broadcast recipient to RecipientDTO
func FromBroadcastRecipient(r broadcast.BroadcastRecipient) RecipientDTO {
	return RecipientDTO{
		UserID:  r.UserID.String(),
		AddedAt: r.AddedAt.Format(time.RFC3339),
	}
}

// FromBroadcastRecipientSlice converts a slice of domain broadcast recipients to RecipientDTO slice
func FromBroadcastRecipientSlice(recipients []broadcast.BroadcastRecipient) []RecipientDTO {
	dtos := make([]RecipientDTO, len(recipients))
	for i, r := range recipients {
		dtos[i] = FromBroadcastRecipient(r)
	}
	return dtos
}
