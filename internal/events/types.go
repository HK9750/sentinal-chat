package events

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	EventMessageNew       EventType = "message:new"
	EventMessageRead      EventType = "message:read"
	EventMessageDelivered EventType = "message:delivered"
	EventTypingStarted    EventType = "typing:started"
	EventTypingStopped    EventType = "typing:stopped"
	EventPresenceOnline   EventType = "presence:online"
	EventPresenceOffline  EventType = "presence:offline"
	EventCallOffer        EventType = "call:offer"
	EventCallAnswer       EventType = "call:answer"
	EventCallICE          EventType = "call:ice"
	EventCallEnded        EventType = "call:ended"
)

// Event is the base interface for all events
type Event interface {
	Type() EventType
	Timestamp() time.Time
	Payload() interface{}
}

// EventHandler processes events
type EventHandler interface {
	Handle(ctx context.Context, event Event) error
}

// EventBus is the main interface for publishing and subscribing
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType EventType, handler EventHandler) error
	Start() error
	Stop() error
}

// BaseEvent provides common fields
type BaseEvent struct {
	EventTypeVal EventType `json:"type"`
	TimestampVal time.Time `json:"timestamp"`
	UserIDVal    uuid.UUID `json:"user_id"`
	ConvIDVal    uuid.UUID `json:"conversation_id,omitempty"`
}

func (e *BaseEvent) Type() EventType      { return e.EventTypeVal }
func (e *BaseEvent) Timestamp() time.Time { return e.TimestampVal }

// MessageNewEvent triggered when a new message is sent
type MessageNewEvent struct {
	BaseEvent
	MessageID      uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	SenderID       uuid.UUID `json:"sender_id"`
	Content        string    `json:"content"`
}

func (e *MessageNewEvent) Payload() interface{} { return e }

// TypingEvent triggered when user starts/stops typing
type TypingEvent struct {
	BaseEvent
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID         uuid.UUID `json:"user_id"`
	DisplayName    string    `json:"display_name"`
	IsTyping       bool      `json:"is_typing"`
}

func (e *TypingEvent) Payload() interface{} { return e }

// MessageReadEvent triggered when a message is marked as read
type MessageReadEvent struct {
	BaseEvent
	MessageID      uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	ReaderID       uuid.UUID `json:"reader_id"`
}

func (e *MessageReadEvent) Payload() interface{} { return e }

// MessageDeliveredEvent triggered when a message is delivered
type MessageDeliveredEvent struct {
	BaseEvent
	MessageID      uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	RecipientID    uuid.UUID `json:"recipient_id"`
}

func (e *MessageDeliveredEvent) Payload() interface{} { return e }

// PresenceEvent triggered when user's presence changes
type PresenceEvent struct {
	BaseEvent
	UserID   uuid.UUID `json:"user_id"`
	IsOnline bool      `json:"is_online"`
	Status   string    `json:"status"` // online, away, busy, offline
}

func (e *PresenceEvent) Payload() interface{} { return e }

// CallSignalingEvent triggered for WebRTC signaling (offer, answer, ice)
type CallSignalingEvent struct {
	BaseEvent
	CallID     uuid.UUID `json:"call_id"`
	FromID     uuid.UUID `json:"from_id"`
	ToID       uuid.UUID `json:"to_id"`
	SignalType string    `json:"signal_type"` // offer, answer, ice
	Data       string    `json:"data"`        // SDP or ICE candidate
}

func (e *CallSignalingEvent) Payload() interface{} { return e }

// CallEndedEvent triggered when a call ends
type CallEndedEvent struct {
	BaseEvent
	CallID         uuid.UUID `json:"call_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	EndedBy        uuid.UUID `json:"ended_by"`
	Reason         string    `json:"reason"` // completed, declined, missed, error
	Duration       int       `json:"duration_seconds"`
}

func (e *CallEndedEvent) Payload() interface{} { return e }
