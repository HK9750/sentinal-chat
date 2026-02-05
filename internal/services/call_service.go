package services

import (
	"context"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type CallService struct {
	repo      repository.CallRepository
	bus       *commands.Bus
	eventRepo repository.EventRepository
}

func NewCallService(repo repository.CallRepository, eventRepo repository.EventRepository, bus *commands.Bus) *CallService {
	return &CallService{repo: repo, eventRepo: eventRepo, bus: bus}
}

func (s *CallService) Create(ctx context.Context, c *call.Call) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "call", "call.initiated", c.ID, c)
}

func (s *CallService) GetByID(ctx context.Context, id uuid.UUID) (call.Call, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CallService) Update(ctx context.Context, c call.Call) error {
	return s.repo.Update(ctx, c)
}

func (s *CallService) GetConversationCalls(ctx context.Context, conversationID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	return s.repo.GetConversationCalls(ctx, conversationID, page, limit)
}

func (s *CallService) GetUserCalls(ctx context.Context, userID uuid.UUID, page, limit int) ([]call.Call, int64, error) {
	return s.repo.GetUserCalls(ctx, userID, page, limit)
}

func (s *CallService) GetActiveCalls(ctx context.Context, userID uuid.UUID) ([]call.Call, error) {
	return s.repo.GetActiveCalls(ctx, userID)
}

func (s *CallService) GetMissedCalls(ctx context.Context, userID uuid.UUID, since time.Time) ([]call.Call, error) {
	return s.repo.GetMissedCalls(ctx, userID, since)
}

func (s *CallService) MarkConnected(ctx context.Context, callID uuid.UUID) error {
	if err := s.repo.MarkConnected(ctx, callID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "call", "call.connected", callID, map[string]any{"call_id": callID})
}

func (s *CallService) EndCall(ctx context.Context, callID uuid.UUID, reason string) error {
	if reason == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if err := s.repo.EndCall(ctx, callID, reason); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "call", "call.ended", callID, map[string]any{"call_id": callID, "reason": reason})
}

func (s *CallService) GetCallDuration(ctx context.Context, callID uuid.UUID) (int32, error) {
	return s.repo.GetCallDuration(ctx, callID)
}

func (s *CallService) AddParticipant(ctx context.Context, p *call.CallParticipant) error {
	if err := s.repo.AddParticipant(ctx, p); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "call", "call.participant_added", p.CallID, p)
}

func (s *CallService) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	if err := s.repo.RemoveParticipant(ctx, callID, userID); err != nil {
		return err
	}
	return createOutboxEvent(ctx, s.eventRepo, "call", "call.participant_removed", callID, map[string]any{"call_id": callID, "user_id": userID})
}

func (s *CallService) GetCallParticipants(ctx context.Context, callID uuid.UUID) ([]call.CallParticipant, error) {
	return s.repo.GetCallParticipants(ctx, callID)
}

func (s *CallService) UpdateParticipantStatus(ctx context.Context, callID, userID uuid.UUID, status string) error {
	return s.repo.UpdateParticipantStatus(ctx, callID, userID, status)
}

func (s *CallService) UpdateParticipantMuteStatus(ctx context.Context, callID, userID uuid.UUID, audioMuted, videoMuted bool) error {
	return s.repo.UpdateParticipantMuteStatus(ctx, callID, userID, audioMuted, videoMuted)
}

func (s *CallService) GetActiveParticipantCount(ctx context.Context, callID uuid.UUID) (int64, error) {
	return s.repo.GetActiveParticipantCount(ctx, callID)
}

func (s *CallService) RecordQualityMetric(ctx context.Context, m *call.CallQualityMetric) error {
	return s.repo.RecordQualityMetric(ctx, m)
}

func (s *CallService) GetCallQualityMetrics(ctx context.Context, callID uuid.UUID) ([]call.CallQualityMetric, error) {
	return s.repo.GetCallQualityMetrics(ctx, callID)
}

func (s *CallService) GetUserCallQualityMetrics(ctx context.Context, callID, userID uuid.UUID) ([]call.CallQualityMetric, error) {
	return s.repo.GetUserCallQualityMetrics(ctx, callID, userID)
}

func (s *CallService) GetAverageCallQuality(ctx context.Context, callID uuid.UUID) (float64, error) {
	return s.repo.GetAverageCallQuality(ctx, callID)
}

func (s *CallService) CreateTurnCredential(ctx context.Context, tc *call.TurnCredential) error {
	return s.repo.CreateTurnCredential(ctx, tc)
}

func (s *CallService) GetActiveTurnCredentials(ctx context.Context, userID uuid.UUID) ([]call.TurnCredential, error) {
	return s.repo.GetActiveTurnCredentials(ctx, userID)
}

func (s *CallService) DeleteExpiredTurnCredentials(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredTurnCredentials(ctx)
}

func (s *CallService) CreateSFUServer(ctx context.Context, server *call.SFUServer) error {
	return s.repo.CreateSFUServer(ctx, server)
}

func (s *CallService) GetSFUServerByID(ctx context.Context, id uuid.UUID) (call.SFUServer, error) {
	return s.repo.GetSFUServerByID(ctx, id)
}

func (s *CallService) GetHealthySFUServers(ctx context.Context, region string) ([]call.SFUServer, error) {
	return s.repo.GetHealthySFUServers(ctx, region)
}

func (s *CallService) GetLeastLoadedServer(ctx context.Context, region string) (call.SFUServer, error) {
	return s.repo.GetLeastLoadedServer(ctx, region)
}

func (s *CallService) UpdateServerLoad(ctx context.Context, serverID uuid.UUID, load int) error {
	return s.repo.UpdateServerLoad(ctx, serverID, load)
}

func (s *CallService) UpdateServerHealth(ctx context.Context, serverID uuid.UUID, isHealthy bool) error {
	return s.repo.UpdateServerHealth(ctx, serverID, isHealthy)
}

func (s *CallService) UpdateServerHeartbeat(ctx context.Context, serverID uuid.UUID) error {
	return s.repo.UpdateServerHeartbeat(ctx, serverID)
}

func (s *CallService) AssignCallToServer(ctx context.Context, a *call.CallServerAssignment) error {
	return s.repo.AssignCallToServer(ctx, a)
}

func (s *CallService) GetCallServerAssignments(ctx context.Context, callID uuid.UUID) ([]call.CallServerAssignment, error) {
	return s.repo.GetCallServerAssignments(ctx, callID)
}

func (s *CallService) RemoveCallServerAssignment(ctx context.Context, callID, serverID uuid.UUID) error {
	return s.repo.RemoveCallServerAssignment(ctx, callID, serverID)
}
