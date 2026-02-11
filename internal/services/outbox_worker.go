package services

import (
	"context"
	"encoding/json"
	"sentinal-chat/internal/domain/outbox"
	"sentinal-chat/internal/events"
	"sentinal-chat/internal/repository"
	"sync"
	"time"
)

// OutboxWorker polls the outbox table and publishes events to Redis
type OutboxWorker struct {
	outboxRepo repository.OutboxRepository
	eventBus   events.EventBus
	interval   time.Duration
	batchSize  int
	stopChan   chan struct{}
	wg         sync.WaitGroup
	running    bool
}

func NewOutboxWorker(outboxRepo repository.OutboxRepository, eventBus events.EventBus) *OutboxWorker {
	return &OutboxWorker{
		outboxRepo: outboxRepo,
		eventBus:   eventBus,
		interval:   100 * time.Millisecond,
		batchSize:  100,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the worker loop
func (w *OutboxWorker) Start() {
	w.running = true
	w.wg.Add(1)
	go w.run()
}

// Stop gracefully shuts down
func (w *OutboxWorker) Stop() {
	w.running = false
	close(w.stopChan)
	w.wg.Wait()
}

func (w *OutboxWorker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

func (w *OutboxWorker) processBatch() {
	ctx := context.Background()
	events, err := w.outboxRepo.GetPending(ctx, w.batchSize)
	if err != nil {
		return
	}

	for _, event := range events {
		w.processEvent(ctx, &event)
	}
}

func (w *OutboxWorker) processEvent(ctx context.Context, event *outbox.OutboxEvent) {
	// Prevent duplicate processing
	if err := w.outboxRepo.MarkProcessing(ctx, event.ID.String()); err != nil {
		return
	}

	// Deserialize and publish
	domainEvent := w.unmarshalEvent(event.EventType, event.Payload)
	if domainEvent == nil {
		w.outboxRepo.MarkFailed(ctx, event.ID.String(), "failed to unmarshal")
		return
	}

	if err := w.eventBus.Publish(ctx, domainEvent); err != nil {
		w.outboxRepo.IncrementRetry(ctx, event.ID.String())
		if event.RetryCount >= 9 {
			w.outboxRepo.MarkFailed(ctx, event.ID.String(), err.Error())
		}
		return
	}

	// Mark as completed
	w.outboxRepo.MarkCompleted(ctx, event.ID.String())
}

func (w *OutboxWorker) unmarshalEvent(eventType string, payload []byte) events.Event {
	switch events.EventType(eventType) {
	case events.EventMessageNew:
		var e events.MessageNewEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventMessageRead:
		var e events.MessageReadEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventMessageDelivered:
		var e events.MessageDeliveredEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventTypingStarted, events.EventTypingStopped:
		var e events.TypingEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventPresenceOnline, events.EventPresenceOffline:
		var e events.PresenceEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventCallOffer, events.EventCallAnswer, events.EventCallICE:
		var e events.CallSignalingEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	case events.EventCallEnded:
		var e events.CallEndedEvent
		if err := json.Unmarshal(payload, &e); err == nil {
			return &e
		}
	}
	return nil
}
