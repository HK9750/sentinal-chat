package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"
)

// EventPublisher writes domain events to the outbox table for reliable delivery
type EventPublisher struct {
	outboxRepo repository.OutboxRepository
}

func NewEventPublisher(outboxRepo repository.OutboxRepository) *EventPublisher {
	return &EventPublisher{outboxRepo: outboxRepo}
}

// PublishMessageNew creates an outbox event for a new message
func (p *EventPublisher) PublishMessageNew(ctx context.Context, tx *gorm.DB, msgID, convID, senderID uuid.UUID) error {
	event := &events.MessageNewEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventMessageNew,
			TimestampVal: time.Now(),
			UserIDVal:    senderID,
			ConvIDVal:    convID,
		},
		MessageID:      msgID,
		ConversationID: convID,
		SenderID:       senderID,
	}

	return p.saveToOutbox(ctx, tx, events.EventMessageNew, "message", convID.String(), event)
}

// PublishTypingStarted creates a typing indicator event
func (p *EventPublisher) PublishTypingStarted(ctx context.Context, tx *gorm.DB, convID, userID uuid.UUID, displayName string) error {
	event := &events.TypingEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventTypingStarted,
			TimestampVal: time.Now(),
			UserIDVal:    userID,
			ConvIDVal:    convID,
		},
		ConversationID: convID,
		UserID:         userID,
		DisplayName:    displayName,
		IsTyping:       true,
	}

	return p.saveToOutbox(ctx, tx, events.EventTypingStarted, "conversation", convID.String(), event)
}

// PublishTypingStopped creates a typing stopped event
func (p *EventPublisher) PublishTypingStopped(ctx context.Context, tx *gorm.DB, convID, userID uuid.UUID, displayName string) error {
	event := &events.TypingEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventTypingStopped,
			TimestampVal: time.Now(),
			UserIDVal:    userID,
			ConvIDVal:    convID,
		},
		ConversationID: convID,
		UserID:         userID,
		DisplayName:    displayName,
		IsTyping:       false,
	}

	return p.saveToOutbox(ctx, tx, events.EventTypingStopped, "conversation", convID.String(), event)
}

// PublishMessageRead creates an event when message is marked as read
func (p *EventPublisher) PublishMessageRead(ctx context.Context, tx *gorm.DB, msgID, convID, readerID uuid.UUID) error {
	event := &events.MessageReadEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventMessageRead,
			TimestampVal: time.Now(),
			UserIDVal:    readerID,
			ConvIDVal:    convID,
		},
		MessageID:      msgID,
		ConversationID: convID,
		ReaderID:       readerID,
	}

	return p.saveToOutbox(ctx, tx, events.EventMessageRead, "message", msgID.String(), event)
}

// PublishMessageDelivered creates an event when message is delivered
func (p *EventPublisher) PublishMessageDelivered(ctx context.Context, tx *gorm.DB, msgID, convID, recipientID uuid.UUID) error {
	event := &events.MessageDeliveredEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventMessageDelivered,
			TimestampVal: time.Now(),
			UserIDVal:    recipientID,
			ConvIDVal:    convID,
		},
		MessageID:      msgID,
		ConversationID: convID,
		RecipientID:    recipientID,
	}

	return p.saveToOutbox(ctx, tx, events.EventMessageDelivered, "message", msgID.String(), event)
}

// PublishPresenceOnline creates an event when user comes online
func (p *EventPublisher) PublishPresenceOnline(ctx context.Context, tx *gorm.DB, userID uuid.UUID) error {
	event := &events.PresenceEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventPresenceOnline,
			TimestampVal: time.Now(),
			UserIDVal:    userID,
		},
		UserID:   userID,
		IsOnline: true,
		Status:   "online",
	}

	return p.saveToOutbox(ctx, tx, events.EventPresenceOnline, "user", userID.String(), event)
}

// PublishPresenceOffline creates an event when user goes offline
func (p *EventPublisher) PublishPresenceOffline(ctx context.Context, tx *gorm.DB, userID uuid.UUID) error {
	event := &events.PresenceEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventPresenceOffline,
			TimestampVal: time.Now(),
			UserIDVal:    userID,
		},
		UserID:   userID,
		IsOnline: false,
		Status:   "offline",
	}

	return p.saveToOutbox(ctx, tx, events.EventPresenceOffline, "user", userID.String(), event)
}

// PublishCallOffer creates an event for WebRTC offer
func (p *EventPublisher) PublishCallOffer(ctx context.Context, tx *gorm.DB, callID, fromID, toID uuid.UUID, sdp string) error {
	event := &events.CallSignalingEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventCallOffer,
			TimestampVal: time.Now(),
			UserIDVal:    fromID,
		},
		CallID:     callID,
		FromID:     fromID,
		ToID:       toID,
		SignalType: "offer",
		Data:       sdp,
	}

	return p.saveToOutbox(ctx, tx, events.EventCallOffer, "call", callID.String(), event)
}

// PublishCallAnswer creates an event for WebRTC answer
func (p *EventPublisher) PublishCallAnswer(ctx context.Context, tx *gorm.DB, callID, fromID, toID uuid.UUID, sdp string) error {
	event := &events.CallSignalingEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventCallAnswer,
			TimestampVal: time.Now(),
			UserIDVal:    fromID,
		},
		CallID:     callID,
		FromID:     fromID,
		ToID:       toID,
		SignalType: "answer",
		Data:       sdp,
	}

	return p.saveToOutbox(ctx, tx, events.EventCallAnswer, "call", callID.String(), event)
}

// PublishCallICE creates an event for ICE candidate
func (p *EventPublisher) PublishCallICE(ctx context.Context, tx *gorm.DB, callID, fromID, toID uuid.UUID, candidate string) error {
	event := &events.CallSignalingEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventCallICE,
			TimestampVal: time.Now(),
			UserIDVal:    fromID,
		},
		CallID:     callID,
		FromID:     fromID,
		ToID:       toID,
		SignalType: "ice",
		Data:       candidate,
	}

	return p.saveToOutbox(ctx, tx, events.EventCallICE, "call", callID.String(), event)
}

// PublishCallEnded creates an event when call ends
func (p *EventPublisher) PublishCallEnded(ctx context.Context, tx *gorm.DB, callID, convID, endedBy uuid.UUID, reason string, duration int) error {
	event := &events.CallEndedEvent{
		BaseEvent: events.BaseEvent{
			EventTypeVal: events.EventCallEnded,
			TimestampVal: time.Now(),
			UserIDVal:    endedBy,
			ConvIDVal:    convID,
		},
		CallID:         callID,
		ConversationID: convID,
		EndedBy:        endedBy,
		Reason:         reason,
		Duration:       duration,
	}

	return p.saveToOutbox(ctx, tx, events.EventCallEnded, "call", callID.String(), event)
}

// saveToOutbox serializes the event and creates an outbox record within the transaction
func (p *EventPublisher) saveToOutbox(ctx context.Context, tx *gorm.DB, eventType events.EventType, aggregateType, aggregateID string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	outboxEvent := &outbox.OutboxEvent{
		EventType:     string(eventType),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       payload,
		Status:        outbox.StatusPending,
	}

	return p.outboxRepo.Create(ctx, tx, outboxEvent)
}
