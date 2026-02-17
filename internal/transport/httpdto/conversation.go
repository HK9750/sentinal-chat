package httpdto

import (
	"sentinal-chat/internal/domain/conversation"
	"time"
)

// CreateConversationRequest is used for POST /conversations
type CreateConversationRequest struct {
	Type         string   `json:"type" binding:"required"` // "DM" or "GROUP"
	Subject      string   `json:"subject,omitempty"`
	Description  string   `json:"description,omitempty"`
	Participants []string `json:"participants" binding:"required"`
}

// CreateConversationResponse is returned after creating a conversation
type CreateConversationResponse struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Subject          string `json:"subject,omitempty"`
	Description      string `json:"description,omitempty"`
	CreatorID        string `json:"creator_id"`
	ParticipantCount int    `json:"participant_count"`
	CreatedAt        string `json:"created_at"`
}

// UpdateConversationRequest is used for PUT /conversations/:id
type UpdateConversationRequest struct {
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// ListConversationsRequest holds query parameters for listing conversations
type ListConversationsRequest struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

// ListConversationsResponse is returned when listing conversations
type ListConversationsResponse struct {
	Conversations []ConversationDTO `json:"conversations"`
	Total         int64             `json:"total"`
}

// ConversationDTO represents a conversation in API responses
type ConversationDTO struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Subject          string `json:"subject,omitempty"`
	Description      string `json:"description,omitempty"`
	AvatarURL        string `json:"avatar_url,omitempty"`
	CreatorID        string `json:"creator_id"`
	InviteLink       string `json:"invite_link,omitempty"`
	ParticipantCount int    `json:"participant_count"`
	LastMessageAt    string `json:"last_message_at,omitempty"`
	CreatedAt        string `json:"created_at"`
}

// SearchConversationsRequest holds query parameters for searching
type SearchConversationsRequest struct {
	Query string `form:"query" binding:"required"`
}

// GetDirectConversationRequest holds query parameters for getting direct conversation
type GetDirectConversationRequest struct {
	UserID1 string `form:"user_id_1" binding:"required"`
	UserID2 string `form:"user_id_2" binding:"required"`
}

// GetByTypeRequest holds query parameters for getting by type
type GetByTypeRequest struct {
	Type string `form:"type" binding:"required"`
}

// GetByInviteLinkRequest holds query parameters for getting by invite link
type GetByInviteLinkRequest struct {
	Link string `form:"link" binding:"required"`
}

// AddParticipantRequest is used for POST /conversations/:id/participants
type AddParticipantRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role,omitempty"` // "member", "admin", "owner"
}

// UpdateParticipantRoleRequest is used for PUT /conversations/:id/participants/:user_id/role
type UpdateParticipantRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ParticipantsResponse is returned when listing participants
type ParticipantsResponse struct {
	Participants []ParticipantDTO `json:"participants"`
}

// ParticipantDTO represents a conversation participant in API responses
type ParticipantDTO struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

// MuteConversationRequest is used for POST /conversations/:id/mute
type MuteConversationRequest struct {
	Until string `json:"until" binding:"required"` // RFC3339 format
}

// UpdateLastReadSequenceRequest is used for PUT /conversations/:id/read-sequence
type UpdateLastReadSequenceRequest struct {
	SeqID int64 `json:"seq_id" binding:"required"`
}

// RegenerateInviteLinkResponse is returned when regenerating invite link
type RegenerateInviteLinkResponse struct {
	InviteLink string `json:"invite_link"`
}

// SequenceResponse is returned when getting/incrementing sequence
type SequenceResponse struct {
	Sequence int64 `json:"sequence"`
}

// ConversationSequenceDTO represents conversation sequence in API responses
type ConversationSequenceDTO struct {
	ConversationID string `json:"conversation_id"`
	LastSequence   int64  `json:"last_sequence"`
	UpdatedAt      string `json:"updated_at"`
}

// FromConversation converts a domain conversation to ConversationDTO
func FromConversation(c conversation.Conversation) ConversationDTO {
	dto := ConversationDTO{
		ID:        c.ID.String(),
		Type:      c.Type,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
	if c.Subject.Valid {
		dto.Subject = c.Subject.String
	}
	if c.Description.Valid {
		dto.Description = c.Description.String
	}
	if c.AvatarURL.Valid {
		dto.AvatarURL = c.AvatarURL.String
	}
	if c.CreatedBy.Valid {
		dto.CreatorID = c.CreatedBy.UUID.String()
	}
	if c.InviteLink.Valid {
		dto.InviteLink = c.InviteLink.String
	}
	dto.ParticipantCount = len(c.Participants)
	return dto
}

// FromConversationSlice converts a slice of domain conversations to ConversationDTO slice
func FromConversationSlice(conversations []conversation.Conversation) []ConversationDTO {
	dtos := make([]ConversationDTO, len(conversations))
	for i, c := range conversations {
		dtos[i] = FromConversation(c)
	}
	return dtos
}

// FromParticipant converts a domain participant to ParticipantDTO
func FromParticipant(p conversation.Participant) ParticipantDTO {
	dto := ParticipantDTO{
		UserID:   p.UserID.String(),
		Role:     p.Role,
		JoinedAt: p.JoinedAt.Format(time.RFC3339),
	}
	return dto
}

// FromParticipantSlice converts a slice of domain participants to ParticipantDTO slice
func FromParticipantSlice(participants []conversation.Participant) []ParticipantDTO {
	dtos := make([]ParticipantDTO, len(participants))
	for i, p := range participants {
		dtos[i] = FromParticipant(p)
	}
	return dtos
}

// FromConversationSequence converts a domain conversation sequence to ConversationSequenceDTO
func FromConversationSequence(s conversation.ConversationSequence) ConversationSequenceDTO {
	return ConversationSequenceDTO{
		ConversationID: s.ConversationID.String(),
		LastSequence:   s.LastSequence,
		UpdatedAt:      s.UpdatedAt.Format(time.RFC3339),
	}
}
