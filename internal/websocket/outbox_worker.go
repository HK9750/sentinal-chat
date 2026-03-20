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

	"go.uber.org/zap"
)

type OutboxWorker struct {
	repo   repository.OutboxRepository
	redis  *redisclient.Client
	logger *logger.Logger
	quit   chan struct{}
}

func NewOutboxWorker(repo repository.OutboxRepository, redis *redisclient.Client, log *logger.Logger) *OutboxWorker {
	return &OutboxWorker{repo: repo, redis: redis, logger: log.WithComponent("outbox_worker"), quit: make(chan struct{})}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.redis == nil {
		return
	}
	w.logger.InfoCtx(ctx, "outbox_worker.starting")
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				w.logger.InfoCtx(ctx, "outbox_worker.stopped", zap.String("reason", "context_cancelled"))
				return
			case <-w.quit:
				w.logger.InfoCtx(ctx, "outbox_worker.stopped", zap.String("reason", "quit_signal"))
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
	w.logger.Info("outbox_worker.stopping")
	close(w.quit)
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	start := time.Now()
	eventsBatch, err := w.repo.GetPending(ctx, 50)
	if err != nil {
		w.logger.LogError(ctx, "outbox.get_pending", err)
		return
	}

	if len(eventsBatch) == 0 {
		return
	}

	w.logger.InfoCtx(ctx, "outbox.batch.started",
		zap.Int("event_count", len(eventsBatch)),
	)

	processedCount := 0
	failedCount := 0
	retriedCount := 0

	for _, event := range eventsBatch {
		claimed, err := w.repo.MarkProcessing(ctx, event.ID.String())
		if err != nil || !claimed {
			if err != nil {
				w.logger.LogError(ctx, "outbox.mark_processing", err, zap.String("event_id", event.ID.String()))
			}
			continue
		}

		var envelope EventEnvelope
		if err := json.Unmarshal(event.Payload, &envelope); err != nil {
			w.markFailed(ctx, event.ID.String(), err)
			failedCount++
			continue
		}

		payload, err := json.Marshal(envelope)
		if err != nil {
			w.markFailed(ctx, event.ID.String(), err)
			failedCount++
			continue
		}

		channel := channelForEnvelope(envelope)
		if channel == "" {
			if err := w.repo.MarkCompleted(ctx, event.ID.String()); err != nil {
				w.logger.LogError(ctx, "outbox.mark_completed", err, zap.String("event_id", event.ID.String()))
			}
			processedCount++
			continue
		}

		publishStart := time.Now()
		if err := w.redis.Publish(ctx, channel, payload); err != nil {
			w.logger.LogRedisOperation(ctx, "PUBLISH", channel, time.Since(publishStart), err)
			if retryErr := w.repo.ScheduleRetry(ctx, event.ID.String(), time.Now().Add(2*time.Second), err.Error()); retryErr != nil {
				w.logger.LogError(ctx, "outbox.schedule_retry", retryErr, zap.String("event_id", event.ID.String()))
			}
			retriedCount++
			continue
		}
		w.logger.LogRedisOperation(ctx, "PUBLISH", channel, time.Since(publishStart), nil)

		if err := w.repo.MarkCompleted(ctx, event.ID.String()); err != nil {
			w.logger.LogError(ctx, "outbox.mark_completed", err, zap.String("event_id", event.ID.String()))
		}
		processedCount++
	}

	w.logger.InfoCtx(ctx, "outbox.batch.completed",
		zap.Int("processed", processedCount),
		zap.Int("failed", failedCount),
		zap.Int("retried", retriedCount),
		zap.Duration("duration", time.Since(start)),
	)
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
	w.logger.LogError(ctx, "outbox.event.failed", err, zap.String("event_id", id))
	if markErr := w.repo.MarkFailed(ctx, id, err.Error()); markErr != nil {
		w.logger.LogError(ctx, "outbox.mark_failed", fmt.Errorf("%v: %w", err, markErr), zap.String("event_id", id))
	}
}
