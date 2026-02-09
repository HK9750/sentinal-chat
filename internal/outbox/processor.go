package outbox

import (
	"context"
	"encoding/json"
	"time"

	"sentinal-chat/internal/domain/event"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"

	"github.com/google/uuid"
)

type Processor struct {
	repo       repository.EventRepository
	publisher  events.Publisher
	clock      func() time.Time
	batchSize  int
	interval   time.Duration
	maxRetries int
}

func NewProcessor(repo repository.EventRepository, publisher events.Publisher, batchSize int, interval time.Duration, maxRetries int) *Processor {
	return &Processor{
		repo:       repo,
		publisher:  publisher,
		clock:      time.Now,
		batchSize:  batchSize,
		interval:   interval,
		maxRetries: maxRetries,
	}
}

func (p *Processor) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

func (p *Processor) processBatch(ctx context.Context) {
	eventsBatch, err := p.repo.GetPendingOutboxEvents(ctx, p.batchSize)
	if err != nil || len(eventsBatch) == 0 {
		return
	}

	for _, e := range eventsBatch {
		if e.RetryCount >= p.maxRetries {
			_ = p.repo.MarkOutboxEventFailed(ctx, e.ID, p.clock().Add(time.Hour), "max retries exceeded")
			continue
		}

		env := events.Envelope{
			EventType:     e.EventType,
			AggregateType: e.AggregateType,
			AggregateID:   e.AggregateID.String(),
			OccurredAt:    e.CreatedAt.UTC(),
			Payload:       json.RawMessage(e.Payload),
		}
		payload, err := json.Marshal(env)
		if err != nil {
			_ = p.repo.MarkOutboxEventFailed(ctx, e.ID, p.clock().Add(time.Minute), err.Error())
			continue
		}

		channel := routeChannel(env)
		if err := p.publisher.Publish(ctx, channel, payload); err != nil {
			_ = p.repo.MarkOutboxEventFailed(ctx, e.ID, p.clock().Add(time.Minute), err.Error())
			_ = p.repo.CreateOutboxEventDelivery(ctx, &event.OutboxEventDelivery{
				ID:            uuid.New(),
				EventID:       e.ID,
				AttemptNumber: e.RetryCount + 1,
				Status:        "FAILED",
			})
			continue
		}

		_ = p.repo.MarkOutboxEventProcessed(ctx, e.ID)
		_ = p.repo.CreateOutboxEventDelivery(ctx, &event.OutboxEventDelivery{
			ID:            uuid.New(),
			EventID:       e.ID,
			AttemptNumber: e.RetryCount + 1,
			Status:        "DELIVERED",
		})
	}
}

// routeChannel determines the Redis pub/sub channel based on event type.
// See Appendix B of database.md for channel taxonomy.
func routeChannel(env events.Envelope) string {
	switch env.AggregateType {
	// Message-related events route to conversation channel
	// The aggregate_id for messages is the message_id, but we need conversation_id
	// Try to extract conversation_id from payload, fallback to aggregate_id
	case events.AggregateTypeMessage:
		if env.EventType == events.EventTypeMessageCreated {
			if recipientID := extractRecipientUserID(env.Payload); recipientID != "" {
				return events.ChannelPrefixUser + recipientID
			}
		}
		if convID := extractConversationID(env.Payload); convID != "" {
			return events.ChannelPrefixConversation + convID
		}
		return events.ChannelPrefixConversation + env.AggregateID

	case events.AggregateTypeMessageReceipt:
		if convID := extractConversationID(env.Payload); convID != "" {
			return events.ChannelPrefixConversation + convID
		}
		return events.ChannelPrefixConversation + env.AggregateID

	case events.AggregateTypeReaction:
		if convID := extractConversationID(env.Payload); convID != "" {
			return events.ChannelPrefixConversation + convID
		}
		return events.ChannelPrefixConversation + env.AggregateID

	case events.AggregateTypeTyping:
		// For typing events, aggregate_id IS the conversation_id
		return events.ChannelPrefixConversation + env.AggregateID

	case events.AggregateTypePoll:
		if convID := extractConversationID(env.Payload); convID != "" {
			return events.ChannelPrefixConversation + convID
		}
		return events.ChannelPrefixConversation + env.AggregateID

	// Conversation and participant events route to conversation channel
	case events.AggregateTypeConversation, events.AggregateTypeParticipant:
		return events.ChannelPrefixConversation + env.AggregateID

	// Call events route to call channel
	case events.AggregateTypeCall:
		return events.ChannelPrefixCall + env.AggregateID

	// Presence events route to presence channel (user-specific)
	case events.AggregateTypePresence:
		return events.ChannelPrefixPresence + env.AggregateID

	// User and encryption events route to user channel
	case events.AggregateTypeUser:
		return events.ChannelPrefixUser + env.AggregateID

	case events.AggregateTypeEncryption:
		// For encryption, try to get user_id from payload
		if userID := extractUserID(env.Payload); userID != "" {
			return events.ChannelPrefixUser + userID
		}
		return events.ChannelPrefixUser + env.AggregateID

	// Broadcast events route to broadcast channel
	case events.AggregateTypeBroadcast:
		return events.ChannelPrefixBroadcast + env.AggregateID

	// Upload events route to user channel (uploader)
	case events.AggregateTypeUpload:
		if userID := extractUserID(env.Payload); userID != "" {
			return events.ChannelPrefixUser + userID
		}
		return events.ChannelPrefixUpload + env.AggregateID

	default:
		return events.ChannelSystemOutbox
	}
}

// extractConversationID attempts to extract conversation_id from the payload JSON
func extractConversationID(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	var data struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	return data.ConversationID
}

// extractUserID attempts to extract user_id from the payload JSON
func extractUserID(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	var data struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	return data.UserID
}

func extractRecipientUserID(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	var data struct {
		RecipientUserID string `json:"recipient_user_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return ""
	}
	return data.RecipientUserID
}
