package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sentinal-chat/internal/events"
	redisclient "sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	"sentinal-chat/pkg/logger"
)

type OutboxWorker struct {
	repo   repository.OutboxRepository
	redis  *redisclient.Client
	logger *logger.Logger
	quit   chan struct{}
}

func NewOutboxWorker(repo repository.OutboxRepository, redis *redisclient.Client, log *logger.Logger) *OutboxWorker {
	return &OutboxWorker{repo: repo, redis: redis, logger: log, quit: make(chan struct{})}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.redis == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.quit:
				return
			case <-ticker.C:
				w.processBatch(ctx)
			}
		}
	}()
}

func (w *OutboxWorker) Stop() {
	if w == nil {
		return
	}
	close(w.quit)
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	eventsBatch, err := w.repo.GetPending(ctx, 50)
	if err != nil {
		w.logf("outbox.get_pending", err)
		return
	}
	if len(eventsBatch) > 0 {
		w.infof("outbox.batch processing_events=%d", len(eventsBatch))
	}
	for _, event := range eventsBatch {
		claimed, err := w.repo.MarkProcessing(ctx, event.ID.String())
		if err != nil || !claimed {
			if err != nil {
				w.logf("outbox.mark_processing", err)
			}
			continue
		}
		var envelope EventEnvelope
		if err := json.Unmarshal(event.Payload, &envelope); err != nil {
			w.markFailed(ctx, event.ID.String(), err)
			continue
		}
		payload, err := json.Marshal(envelope)
		if err != nil {
			w.markFailed(ctx, event.ID.String(), err)
			continue
		}
		channel := channelForEnvelope(envelope)
		if channel == "" {
			if err := w.repo.MarkCompleted(ctx, event.ID.String()); err != nil {
				w.logf("outbox.mark_completed", err)
			}
			continue
		}
		if err := w.redis.Publish(ctx, channel, payload); err != nil {
			if retryErr := w.repo.ScheduleRetry(ctx, event.ID.String(), time.Now().Add(2*time.Second), err.Error()); retryErr != nil {
				w.logf("outbox.schedule_retry", retryErr)
			}
			continue
		}
		if err := w.repo.MarkCompleted(ctx, event.ID.String()); err != nil {
			w.logf("outbox.mark_completed", err)
		}
	}
}

func channelForEnvelope(envelope EventEnvelope) string {
	if strings.TrimSpace(envelope.CallID) != "" {
		return events.CallChannel(envelope.CallID)
	}
	if strings.TrimSpace(envelope.ConversationID) != "" {
		return events.ConversationChannel(envelope.ConversationID)
	}
	if strings.TrimSpace(envelope.UserID) != "" {
		return events.UserChannel(envelope.UserID)
	}
	if strings.TrimSpace(envelope.DeviceID) != "" {
		return events.DeviceChannel(envelope.DeviceID)
	}
	return ""
}

func (w *OutboxWorker) markFailed(ctx context.Context, id string, err error) {
	if err == nil {
		return
	}
	if markErr := w.repo.MarkFailed(ctx, id, err.Error()); markErr != nil {
		w.logf("outbox.mark_failed", fmt.Errorf("%v: %w", err, markErr))
	}
}

func (w *OutboxWorker) logf(operation string, err error) {
	if w == nil || w.logger == nil || err == nil {
		return
	}
	w.logger.Errorf("%s: %v", operation, err)
}

func (w *OutboxWorker) infof(template string, args ...interface{}) {
	if w == nil || w.logger == nil {
		return
	}
	w.logger.Infof(template, args...)
}
