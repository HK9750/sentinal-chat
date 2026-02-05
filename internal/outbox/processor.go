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

func routeChannel(env events.Envelope) string {
	switch env.AggregateType {
	case "message":
		return "channel:conversation:" + env.AggregateID
	case "call":
		return "channel:call:" + env.AggregateID
	case "presence":
		return "channel:presence:" + env.AggregateID
	default:
		return "channel:system:outbox"
	}
}
