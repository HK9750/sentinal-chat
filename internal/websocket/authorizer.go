package websocket

import (
	"context"
	"strings"

	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

// ChannelAuthorizer handles authorization for WebSocket channel subscriptions
type ChannelAuthorizer struct {
	conversationRepo repository.ConversationRepository
	callRepo         repository.CallRepository
	broadcastRepo    repository.BroadcastRepository
}

// NewChannelAuthorizer creates a new channel authorizer
func NewChannelAuthorizer(
	conversationRepo repository.ConversationRepository,
	callRepo repository.CallRepository,
	broadcastRepo repository.BroadcastRepository,
) *ChannelAuthorizer {
	return &ChannelAuthorizer{
		conversationRepo: conversationRepo,
		callRepo:         callRepo,
		broadcastRepo:    broadcastRepo,
	}
}

// CanSubscribe checks if a user is authorized to subscribe to a channel
func (a *ChannelAuthorizer) CanSubscribe(ctx context.Context, userID string, channel string) (bool, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, nil
	}

	// User's own channels - always allowed
	if strings.HasPrefix(channel, events.ChannelPrefixUser+userID) ||
		strings.HasPrefix(channel, events.ChannelPrefixPresence+userID) {
		return true, nil
	}

	// Conversation channels - check if user is a participant
	if strings.HasPrefix(channel, events.ChannelPrefixConversation) {
		convIDStr := strings.TrimPrefix(channel, events.ChannelPrefixConversation)
		convID, err := uuid.Parse(convIDStr)
		if err != nil {
			return false, nil
		}
		return a.conversationRepo.IsParticipant(ctx, convID, userUUID)
	}

	// Call channels - check if user is a call participant
	if strings.HasPrefix(channel, events.ChannelPrefixCall) {
		callIDStr := strings.TrimPrefix(channel, events.ChannelPrefixCall)
		callID, err := uuid.Parse(callIDStr)
		if err != nil {
			return false, nil
		}
		return a.callRepo.IsCallParticipant(ctx, callID, userUUID)
	}

	// Broadcast channels - check if user is owner or recipient
	if strings.HasPrefix(channel, events.ChannelPrefixBroadcast) {
		broadcastIDStr := strings.TrimPrefix(channel, events.ChannelPrefixBroadcast)
		broadcastID, err := uuid.Parse(broadcastIDStr)
		if err != nil {
			return false, nil
		}
		// Check if user is recipient
		isRecipient, err := a.broadcastRepo.IsRecipient(ctx, broadcastID, userUUID)
		if err != nil {
			return false, err
		}
		if isRecipient {
			return true, nil
		}
		// Check if user is owner
		broadcast, err := a.broadcastRepo.GetByID(ctx, broadcastID)
		if err != nil {
			return false, nil // Broadcast not found
		}
		return broadcast.OwnerID == userUUID, nil
	}

	// Presence channels for other users - check if they are contacts or in shared conversation
	if strings.HasPrefix(channel, events.ChannelPrefixPresence) {
		// For now, allow presence subscriptions if user is in any shared conversation
		// This could be made more strict by checking contacts list
		targetUserIDStr := strings.TrimPrefix(channel, events.ChannelPrefixPresence)
		targetUserID, err := uuid.Parse(targetUserIDStr)
		if err != nil {
			return false, nil
		}
		return a.hasSharedConversation(ctx, userUUID, targetUserID)
	}

	// Upload channels - only owner can subscribe
	if strings.HasPrefix(channel, events.ChannelPrefixUpload) {
		// Upload channels are typically subscribed via user channel
		// Direct upload channel subscription is restricted
		return false, nil
	}

	// System channels - not allowed for regular users
	if strings.HasPrefix(channel, "channel:system:") {
		return false, nil
	}

	// Default deny
	return false, nil
}

// hasSharedConversation checks if two users share any conversation
func (a *ChannelAuthorizer) hasSharedConversation(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	// Try to find a direct conversation between them
	_, err := a.conversationRepo.GetDirectConversation(ctx, userID1, userID2)
	if err == nil {
		return true, nil
	}

	// Could extend this to check group conversations, but for now
	// direct conversation check is sufficient for presence authorization
	return false, nil
}
