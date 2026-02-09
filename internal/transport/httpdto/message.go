package httpdto

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
