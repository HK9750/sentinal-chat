package services

import (
	"context"
	"database/sql"
	"time"

	"sentinal-chat/internal/commands"
	"sentinal-chat/internal/domain/call"
	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/repository"
	sentinal_errors "sentinal-chat/pkg/errors"

	"github.com/google/uuid"
)

type CallService struct {
	repo           repository.CallRepository
	bus            *commands.Bus
	eventRepo      repository.EventRepository
	signalingStore *redis.SignalingStore
}

func NewCallService(repo repository.CallRepository, eventRepo repository.EventRepository, bus *commands.Bus, signalingStore *redis.SignalingStore) *CallService {
	if bus == nil {
		bus = commands.NewBus()
	}
	svc := &CallService{repo: repo, eventRepo: eventRepo, bus: bus, signalingStore: signalingStore}
	svc.RegisterHandlers(bus)
	return svc
}

func (s *CallService) RegisterHandlers(bus *commands.Bus) {
	if bus == nil {
		return
	}

	// call.initiate - Initiate a new call
	bus.Register("call.initiate", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.InitiateCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		topology := c.Topology
		if topology == "" {
			topology = "P2P"
		}
		newCall := &call.Call{
			ID:             uuid.New(),
			ConversationID: c.ConversationID,
			InitiatedBy:    c.InitiatorID,
			Type:           c.CallType,
			Topology:       topology,
			IsGroupCall:    c.IsGroupCall,
			StartedAt:      time.Now(),
			CreatedAt:      time.Now(),
		}
		if err := s.Create(ctx, newCall); err != nil {
			return commands.Result{}, err
		}

		// Create call state in Redis for real-time signaling
		if s.signalingStore != nil {
			callState := &redis.CallState{
				CallID:         newCall.ID.String(),
				ConversationID: newCall.ConversationID.String(),
				InitiatorID:    newCall.InitiatedBy.String(),
				CallType:       newCall.Type,
				Status:         "RINGING",
				Participants:   map[string]string{newCall.InitiatedBy.String(): "JOINED"},
				StartedAt:      newCall.StartedAt,
			}
			_ = s.signalingStore.CreateCallState(ctx, callState)
		}

		return commands.Result{AggregateID: newCall.ID.String(), Payload: newCall}, nil
	}))

	// call.accept - Accept an incoming call
	bus.Register("call.accept", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.AcceptCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		// Mark call as connected
		if err := s.MarkConnected(ctx, c.CallID); err != nil {
			return commands.Result{}, err
		}
		existingCall, err := s.GetByID(ctx, c.CallID)
		if err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "call", "call.accepted", c.CallID, map[string]any{"call_id": c.CallID, "user_id": c.UserID})
		return commands.Result{AggregateID: c.CallID.String(), Payload: existingCall}, nil
	}))

	// call.reject - Reject an incoming call
	bus.Register("call.reject", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RejectCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		// End call with DECLINED reason
		if err := s.EndCall(ctx, c.CallID, "DECLINED"); err != nil {
			return commands.Result{}, err
		}
		existingCall, err := s.GetByID(ctx, c.CallID)
		if err != nil {
			return commands.Result{}, err
		}
		_ = createOutboxEvent(ctx, s.eventRepo, "call", "call.rejected", c.CallID, map[string]any{"call_id": c.CallID, "user_id": c.UserID})
		return commands.Result{AggregateID: c.CallID.String(), Payload: existingCall}, nil
	}))

	// call.end - End an active call
	bus.Register("call.end", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.EndCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		reason := c.Reason
		if reason == "" {
			reason = "COMPLETED"
		}
		if err := s.EndCall(ctx, c.CallID, reason); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.join - Join an ongoing call
	bus.Register("call.join", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.JoinCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		participant := &call.CallParticipant{
			CallID:   c.CallID,
			UserID:   c.UserID,
			JoinedAt: sql.NullTime{Time: time.Now(), Valid: true},
			Status:   "CONNECTED",
		}
		if err := s.AddParticipant(ctx, participant); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.CallID.String(), Payload: participant}, nil
	}))

	// call.leave - Leave an ongoing call
	bus.Register("call.leave", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.LeaveCallCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		if err := s.RemoveParticipant(ctx, c.CallID, c.UserID); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.toggle_mute - Toggle audio/video mute
	bus.Register("call.toggle_mute", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.ToggleMuteCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		audioMuted := false
		videoMuted := false
		if c.MuteAudio != nil {
			audioMuted = *c.MuteAudio
		}
		if c.MuteVideo != nil {
			videoMuted = *c.MuteVideo
		}
		if err := s.UpdateParticipantMuteStatus(ctx, c.CallID, c.UserID, audioMuted, videoMuted); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.offer - Send WebRTC SDP offer (real-time signaling via Redis pub/sub)
	bus.Register("call.offer", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SendOfferCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		// Real-time signaling - publish directly via SignalingStore
		if s.signalingStore != nil {
			if err := s.signalingStore.SendOffer(ctx, c.CallID.String(), c.FromID.String(), c.ToID.String(), c.SDP); err != nil {
				return commands.Result{}, err
			}
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.answer - Send WebRTC SDP answer (real-time signaling via Redis pub/sub)
	bus.Register("call.answer", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SendAnswerCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		// Real-time signaling - publish directly via SignalingStore
		if s.signalingStore != nil {
			if err := s.signalingStore.SendAnswer(ctx, c.CallID.String(), c.FromID.String(), c.ToID.String(), c.SDP); err != nil {
				return commands.Result{}, err
			}
			// Update call status to CONNECTED when answer is received
			_ = s.signalingStore.UpdateCallStatus(ctx, c.CallID.String(), "CONNECTED")
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.ice - Send WebRTC ICE candidate (real-time signaling via Redis pub/sub)
	bus.Register("call.ice", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.SendICECandidateCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		// Real-time signaling - publish directly via SignalingStore
		if s.signalingStore != nil {
			candidate := &redis.ICECandidate{
				Candidate:     c.Candidate,
				SDPMid:        c.SDPMid,
				SDPMLineIndex: c.SDPMLineIndex,
			}
			if err := s.signalingStore.SendICECandidate(ctx, c.CallID.String(), c.FromID.String(), c.ToID.String(), candidate); err != nil {
				return commands.Result{}, err
			}
		}
		return commands.Result{AggregateID: c.CallID.String()}, nil
	}))

	// call.record_quality - Record call quality metrics
	bus.Register("call.record_quality", commands.HandlerFunc(func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
		c, ok := cmd.(commands.RecordQualityMetricCommand)
		if !ok {
			return commands.Result{}, sentinal_errors.ErrInvalidInput
		}
		metric := &call.CallQualityMetric{
			ID:               uuid.New(),
			CallID:           c.CallID,
			UserID:           c.UserID,
			PacketsSent:      c.PacketsSent,
			PacketsReceived:  c.PacketsReceived,
			PacketsLost:      c.PacketsLost,
			JitterMs:         c.JitterMs,
			RoundTripTimeMs:  c.RoundTripTimeMs,
			BitrateKbps:      c.BitrateKbps,
			FrameRate:        c.FrameRate,
			ResolutionWidth:  c.ResolutionWidth,
			ResolutionHeight: c.ResolutionHeight,
			AudioLevel:       c.AudioLevel,
			ConnectionType:   c.ConnectionType,
			IceCandidateType: c.ICECandidateType,
			RecordedAt:       c.RecordedAt,
		}
		if metric.RecordedAt.IsZero() {
			metric.RecordedAt = time.Now()
		}
		if err := s.RecordQualityMetric(ctx, metric); err != nil {
			return commands.Result{}, err
		}
		return commands.Result{AggregateID: c.CallID.String(), Payload: metric}, nil
	}))
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
