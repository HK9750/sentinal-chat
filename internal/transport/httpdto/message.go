package httpdto

// SendMessageRequest is used for POST /messages
type SendMessageRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
	Content        string `json:"content" binding:"required"`
	ClientMsgID    string `json:"client_message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// SendMessageResponse is returned after sending a message
type SendMessageResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Content        string `json:"content"`
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
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Content        string `json:"content"`
	ClientMsgID    string `json:"client_message_id,omitempty"`
	SequenceNumber int64  `json:"sequence_number"`
	IsDeleted      bool   `json:"is_deleted"`
	IsEdited       bool   `json:"is_edited"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

// UpdateMessageRequest is used for PUT /messages/:id
type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}
