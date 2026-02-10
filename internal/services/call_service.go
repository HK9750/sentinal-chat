package services

import (
	"context"
	"time"

	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type CallService struct {
	repo           repository.CallRepository
	signalingStore *redis.SignalingStore
}

func NewCallService(repo repository.CallRepository, signalingStore *redis.SignalingStore) *CallService {
	return &CallService{repo: repo, signalingStore: signalingStore}
}

func (s *CallService) Create(ctx context.Context, c *call.Call) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return err
	}
	if s.signalingStore != nil {
		state := &redis.CallState{
			CallID:         c.ID.String(),
			ConversationID: c.ConversationID.String(),
			InitiatorID:    c.InitiatedBy.String(),
			CallType:       c.Type,
			Status:         "RINGING",
			Participants:   map[string]string{c.InitiatedBy.String(): "JOINED"},
			StartedAt:      c.StartedAt,
		}
		_ = s.signalingStore.CreateCallState(ctx, state)
	}
	return nil
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
	if s.signalingStore != nil {
		return s.signalingStore.UpdateCallStatus(ctx, callID.String(), "CONNECTED")
	}
	return nil
}

func (s *CallService) EndCall(ctx context.Context, callID uuid.UUID, reason string) error {
	if reason == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if err := s.repo.EndCall(ctx, callID, reason); err != nil {
		return err
	}
	if s.signalingStore != nil {
		_ = s.signalingStore.SendCallEnded(ctx, callID.String(), reason)
		_ = s.signalingStore.RemoveCallState(ctx, callID.String())
	}
	return nil
}

func (s *CallService) GetCallDuration(ctx context.Context, callID uuid.UUID) (int32, error) {
	return s.repo.GetCallDuration(ctx, callID)
}

func (s *CallService) AddParticipant(ctx context.Context, p *call.CallParticipant) error {
	return s.repo.AddParticipant(ctx, p)
}

func (s *CallService) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	return s.repo.RemoveParticipant(ctx, callID, userID)
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

func (s *CallService) SendOffer(ctx context.Context, callID, fromID, toID uuid.UUID, sdp string) error {
	if s.signalingStore == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if callID == uuid.Nil || fromID == uuid.Nil || toID == uuid.Nil || sdp == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return s.signalingStore.SendOffer(ctx, callID.String(), fromID.String(), toID.String(), sdp)
}

func (s *CallService) SendAnswer(ctx context.Context, callID, fromID, toID uuid.UUID, sdp string) error {
	if s.signalingStore == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if callID == uuid.Nil || fromID == uuid.Nil || toID == uuid.Nil || sdp == "" {
		return sentinal_errors.ErrInvalidInput
	}
	if err := s.signalingStore.SendAnswer(ctx, callID.String(), fromID.String(), toID.String(), sdp); err != nil {
		return err
	}
	return s.signalingStore.UpdateCallStatus(ctx, callID.String(), "CONNECTED")
}

func (s *CallService) SendICECandidate(ctx context.Context, callID, fromID, toID uuid.UUID, candidate, sdpMid string, sdpMLineIndex int) error {
	if s.signalingStore == nil {
		return sentinal_errors.ErrInvalidInput
	}
	if callID == uuid.Nil || fromID == uuid.Nil || toID == uuid.Nil || candidate == "" {
		return sentinal_errors.ErrInvalidInput
	}
	return s.signalingStore.SendICECandidate(ctx, callID.String(), fromID.String(), toID.String(), &redis.ICECandidate{
		Candidate:     candidate,
		SDPMid:        sdpMid,
		SDPMLineIndex: sdpMLineIndex,
	})
}
