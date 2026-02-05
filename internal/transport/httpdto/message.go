package httpdto

type SendMessageRequest struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	ClientMsgID    string `json:"client_message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}
