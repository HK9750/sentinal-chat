package httpdto

import (
	"encoding/base64"
	"sentinal-chat/internal/domain/message"
	"time"

	"github.com/google/uuid"
)

// SendMessageRequest is used for POST /messages
type SendMessageRequest struct {
	ConversationID string                   `json:"conversation_id" binding:"required"`
	Ciphertexts    []MessageCiphertextInput `json:"ciphertexts" binding:"required"`
	MessageType    string                   `json:"message_type"`
	ClientMsgID    string                   `json:"client_message_id"`
	IdempotencyKey string                   `json:"idempotency_key"`
}

// MessageCiphertextInput represents per-device ciphertext for a message
type MessageCiphertextInput struct {
	RecipientDeviceID string                 `json:"recipient_device_id" binding:"required"`
	Ciphertext        string                 `json:"ciphertext" binding:"required"`
	Header            map[string]interface{} `json:"header"`
}

// SendMessageResponse is returned after sending a message
type SendMessageResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	ClientMsgID    string `json:"client_message_id,omitempty"`
	SequenceNumber int64  `json:"sequence_number"`
	CreatedAt      string `json:"created_at"`
}

// ListMessagesRequest holds query parameters for listing messages
type ListMessagesRequest struct {
	ConversationID string `form:"conversation_id" binding:"required"`
	BeforeSeq      int64  `form:"before_seq"`
	Limit          int    `form:"limit"`
}

// ListMessagesResponse is returned when listing messages
type ListMessagesResponse struct {
	Messages []MessageDTO `json:"messages"`
}

// MessageDTO represents a message in API responses
type MessageDTO struct {
	ID                string `json:"id"`
	ConversationID    string `json:"conversation_id"`
	SenderID          string `json:"sender_id"`
	ClientMsgID       string `json:"client_message_id,omitempty"`
	SequenceNumber    int64  `json:"sequence_number"`
	IsDeleted         bool   `json:"is_deleted"`
	IsEdited          bool   `json:"is_edited"`
	Ciphertext        string `json:"ciphertext,omitempty"`
	Header            string `json:"header,omitempty"`
	RecipientDeviceID string `json:"recipient_device_id,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

// UpdateMessageRequest is used for PUT /messages/:id
type UpdateMessageRequest struct {
	Ciphertext string `json:"ciphertext" binding:"required"`
}

// FromMessage converts a domain message to MessageDTO
func FromMessage(m message.Message) MessageDTO {
	dto := MessageDTO{
		ID:             m.ID.String(),
		ConversationID: m.ConversationID.String(),
		SenderID:       m.SenderID.String(),
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		IsDeleted:      m.DeletedAt.Valid,
		IsEdited:       m.EditedAt.Valid,
	}
	if m.ClientMessageID.Valid {
		dto.ClientMsgID = m.ClientMessageID.String
	}
	if m.SeqID.Valid {
		dto.SequenceNumber = m.SeqID.Int64
	}
	if len(m.Ciphertext) > 0 {
		dto.Ciphertext = base64.StdEncoding.EncodeToString(m.Ciphertext)
	}
	if m.Header != "" {
		dto.Header = m.Header
	}
	if m.RecipientDeviceID.Valid {
		dto.RecipientDeviceID = m.RecipientDeviceID.UUID.String()
	}
	if m.EditedAt.Valid {
		dto.UpdatedAt = m.EditedAt.Time.Format(time.RFC3339)
	}
	return dto
}

// FromSendMessage converts a domain message to SendMessageResponse
func FromSendMessage(m message.Message) SendMessageResponse {
	res := SendMessageResponse{
		ID:             m.ID.String(),
		ConversationID: m.ConversationID.String(),
		SenderID:       m.SenderID.String(),
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
	}
	if m.ClientMessageID.Valid {
		res.ClientMsgID = m.ClientMessageID.String
	}
	if m.SeqID.Valid {
		res.SequenceNumber = m.SeqID.Int64
	}
	return res
}

// FromMessageSlice converts a slice of domain messages to MessageDTO slice
func FromMessageSlice(messages []message.Message) []MessageDTO {
	dtos := make([]MessageDTO, len(messages))
	for i, m := range messages {
		dtos[i] = FromMessage(m)
	}
	return dtos
}

// NullUUIDString converts a uuid.NullUUID to string
func NullUUIDString(value uuid.NullUUID) string {
	if value.Valid {
		return value.UUID.String()
	}
	return ""
}
