package proxy

import (
	"context"

	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type AccessControl struct {
	eventRepo        repository.EventRepository
	conversationRepo repository.ConversationRepository
}

func NewAccessControl(eventRepo repository.EventRepository, conversationRepo repository.ConversationRepository) *AccessControl {
	return &AccessControl{eventRepo: eventRepo, conversationRepo: conversationRepo}
}

func (a *AccessControl) CanSendMessage(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "message.send")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

func (a *AccessControl) CanViewConversation(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "conversation.view")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

func (a *AccessControl) CanManageGroup(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	participant, err := a.conversationRepo.GetParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if participant.Role != "OWNER" && participant.Role != "ADMIN" {
		return sentinal_errors.ErrForbidden
	}
	return nil
}

func (a *AccessControl) CanInitiateCall(ctx context.Context, userID, conversationID uuid.UUID) error {
	if a.eventRepo != nil {
		ok, err := a.eventRepo.HasAccessPolicy(ctx, "conversation", conversationID, "user", userID, "call.start")
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return a.ensureParticipant(ctx, conversationID, userID)
}

func (a *AccessControl) ensureParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	if a.conversationRepo == nil {
		return sentinal_errors.ErrForbidden
	}
	ok, err := a.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return sentinal_errors.ErrForbidden
	}
	return nil
}
