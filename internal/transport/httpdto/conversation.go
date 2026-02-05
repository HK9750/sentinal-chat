package httpdto

type CreateConversationRequest struct {
	Type         string   `json:"type"`
	Subject      string   `json:"subject"`
	Description  string   `json:"description"`
	Participants []string `json:"participants"`
}
