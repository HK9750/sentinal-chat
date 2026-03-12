package proxy

import (
	"context"

	"github.com/google/uuid"

	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"
)

type MembershipProxy struct {
	conversations repository.ConversationRepository
}

func NewMembershipProxy(conversations repository.ConversationRepository) *MembershipProxy {
	return &MembershipProxy{conversations: conversations}
}

func (p *MembershipProxy) RequireParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	if p == nil || p.conversations == nil {
		return sentinal_errors.ErrServiceUnavailable
	}
	allowed, err := p.conversations.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

func (p *MembershipProxy) RequireAdminOrOwner(ctx context.Context, conversationID, userID uuid.UUID) error {
	if err := p.RequireParticipant(ctx, conversationID, userID); err != nil {
		return err
	}
	participant, err := p.conversations.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant.Role != "OWNER" && participant.Role != "ADMIN" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

func (p *MembershipProxy) RequireOwner(ctx context.Context, conversationID, userID uuid.UUID) error {
	if err := p.RequireParticipant(ctx, conversationID, userID); err != nil {
		return err
	}
	participant, err := p.conversations.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant.Role != "OWNER" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}
