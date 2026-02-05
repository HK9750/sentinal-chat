package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// SignalingMessage represents a WebRTC signaling message
type SignalingMessage struct {
	Type      string        `json:"type"` // offer, answer, ice_candidate
	CallID    string        `json:"call_id"`
	FromID    string        `json:"from_id"`
	ToID      string        `json:"to_id"`
	SDP       string        `json:"sdp,omitempty"`
	Candidate *ICECandidate `json:"candidate,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ICECandidate represents a WebRTC ICE candidate
type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdp_mid"`
	SDPMLineIndex int    `json:"sdp_mline_index"`
}

// CallState represents the current state of a call in Redis
type CallState struct {
	CallID         string            `json:"call_id"`
	ConversationID string            `json:"conversation_id"`
	InitiatorID    string            `json:"initiator_id"`
	CallType       string            `json:"call_type"`    // AUDIO, VIDEO
	Status         string            `json:"status"`       // RINGING, CONNECTED, ENDED
	Participants   map[string]string `json:"participants"` // userID -> status (INVITED, JOINED, LEFT)
	StartedAt      time.Time         `json:"started_at"`
	ConnectedAt    *time.Time        `json:"connected_at,omitempty"`
}

// SignalingStore handles WebRTC signaling state in Redis
type SignalingStore struct {
	client    *goredis.Client
	publisher *Publisher
}

// Redis key prefixes for signaling
const (
	signalingCallStateKey  = "call:state:"      // Hash storing call state
	signalingOffersKey     = "call:offers:"     // List of pending offers for a user
	signalingCandidatesKey = "call:candidates:" // List of ICE candidates for a call
	signalingTTL           = 5 * time.Minute    // TTL for signaling data
)

// NewSignalingStore creates a new signaling store
func NewSignalingStore(client *goredis.Client, publisher *Publisher) *SignalingStore {
	return &SignalingStore{
		client:    client,
		publisher: publisher,
	}
}

// CreateCallState initializes call state in Redis
func (s *SignalingStore) CreateCallState(ctx context.Context, state *CallState) error {
	key := signalingCallStateKey + state.CallID
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, data, signalingTTL).Err()
}

// GetCallState retrieves call state from Redis
func (s *SignalingStore) GetCallState(ctx context.Context, callID string) (*CallState, error) {
	key := signalingCallStateKey + callID
	data, err := s.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var state CallState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// UpdateCallState updates call state in Redis
func (s *SignalingStore) UpdateCallState(ctx context.Context, state *CallState) error {
	return s.CreateCallState(ctx, state)
}

// UpdateCallStatus updates just the status field
func (s *SignalingStore) UpdateCallStatus(ctx context.Context, callID, status string) error {
	state, err := s.GetCallState(ctx, callID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("call not found: %s", callID)
	}
	state.Status = status
	if status == "CONNECTED" {
		now := time.Now()
		state.ConnectedAt = &now
	}
	return s.UpdateCallState(ctx, state)
}

// AddParticipant adds a participant to the call state
func (s *SignalingStore) AddParticipant(ctx context.Context, callID, userID, status string) error {
	state, err := s.GetCallState(ctx, callID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("call not found: %s", callID)
	}
	if state.Participants == nil {
		state.Participants = make(map[string]string)
	}
	state.Participants[userID] = status
	return s.UpdateCallState(ctx, state)
}

// RemoveCallState removes call state from Redis
func (s *SignalingStore) RemoveCallState(ctx context.Context, callID string) error {
	key := signalingCallStateKey + callID
	return s.client.Del(ctx, key).Err()
}

// SendOffer sends an SDP offer to a user via Redis pub/sub
func (s *SignalingStore) SendOffer(ctx context.Context, callID, fromID, toID, sdp string) error {
	msg := SignalingMessage{
		Type:      "offer",
		CallID:    callID,
		FromID:    fromID,
		ToID:      toID,
		SDP:       sdp,
		Timestamp: time.Now(),
	}
	return s.publishSignalingMessage(ctx, toID, msg)
}

// SendAnswer sends an SDP answer to a user via Redis pub/sub
func (s *SignalingStore) SendAnswer(ctx context.Context, callID, fromID, toID, sdp string) error {
	msg := SignalingMessage{
		Type:      "answer",
		CallID:    callID,
		FromID:    fromID,
		ToID:      toID,
		SDP:       sdp,
		Timestamp: time.Now(),
	}
	return s.publishSignalingMessage(ctx, toID, msg)
}

// SendICECandidate sends an ICE candidate to a user via Redis pub/sub
func (s *SignalingStore) SendICECandidate(ctx context.Context, callID, fromID, toID string, candidate *ICECandidate) error {
	msg := SignalingMessage{
		Type:      "ice_candidate",
		CallID:    callID,
		FromID:    fromID,
		ToID:      toID,
		Candidate: candidate,
		Timestamp: time.Now(),
	}

	// Also store candidate for late joiners (trickle ICE)
	if err := s.storeICECandidate(ctx, callID, fromID, toID, candidate); err != nil {
		// Log error but don't fail - storing is optional
	}

	return s.publishSignalingMessage(ctx, toID, msg)
}

// storeICECandidate stores ICE candidate for potential late retrieval
func (s *SignalingStore) storeICECandidate(ctx context.Context, callID, fromID, toID string, candidate *ICECandidate) error {
	key := fmt.Sprintf("%s%s:%s:%s", signalingCandidatesKey, callID, fromID, toID)
	data, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, signalingTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// GetStoredICECandidates retrieves stored ICE candidates for a peer connection
func (s *SignalingStore) GetStoredICECandidates(ctx context.Context, callID, fromID, toID string) ([]*ICECandidate, error) {
	key := fmt.Sprintf("%s%s:%s:%s", signalingCandidatesKey, callID, fromID, toID)
	data, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var candidates []*ICECandidate
	for _, d := range data {
		var candidate ICECandidate
		if err := json.Unmarshal([]byte(d), &candidate); err != nil {
			continue
		}
		candidates = append(candidates, &candidate)
	}
	return candidates, nil
}

// ClearICECandidates removes stored ICE candidates
func (s *SignalingStore) ClearICECandidates(ctx context.Context, callID, fromID, toID string) error {
	key := fmt.Sprintf("%s%s:%s:%s", signalingCandidatesKey, callID, fromID, toID)
	return s.client.Del(ctx, key).Err()
}

// publishSignalingMessage publishes a signaling message to the target user's call channel
func (s *SignalingStore) publishSignalingMessage(ctx context.Context, targetUserID string, msg SignalingMessage) error {
	if s.publisher == nil {
		return fmt.Errorf("publisher not configured")
	}

	event := map[string]interface{}{
		"event_type":     "call." + msg.Type,
		"aggregate_type": "call",
		"aggregate_id":   msg.CallID,
		"occurred_at":    msg.Timestamp.UTC().Format(time.RFC3339),
		"payload":        msg,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Publish to the call channel (all participants subscribed)
	callChannel := fmt.Sprintf("channel:call:%s", msg.CallID)
	if err := s.publisher.Publish(ctx, callChannel, data); err != nil {
		return err
	}

	// Also publish to user's personal channel for redundancy
	userChannel := fmt.Sprintf("channel:user:%s", targetUserID)
	return s.publisher.Publish(ctx, userChannel, data)
}

// SendCallRinging sends a ringing notification to participants
func (s *SignalingStore) SendCallRinging(ctx context.Context, callID, initiatorID string, participantIDs []string) error {
	for _, participantID := range participantIDs {
		event := map[string]interface{}{
			"event_type":     "call.ringing",
			"aggregate_type": "call",
			"aggregate_id":   callID,
			"occurred_at":    time.Now().UTC().Format(time.RFC3339),
			"payload": map[string]interface{}{
				"call_id":      callID,
				"initiator_id": initiatorID,
				"target_id":    participantID,
			},
		}

		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		userChannel := fmt.Sprintf("channel:user:%s", participantID)
		s.publisher.Publish(ctx, userChannel, data)
	}
	return nil
}

// SendCallEnded sends call ended notification to all participants
func (s *SignalingStore) SendCallEnded(ctx context.Context, callID, reason string) error {
	event := map[string]interface{}{
		"event_type":     "call.ended",
		"aggregate_type": "call",
		"aggregate_id":   callID,
		"occurred_at":    time.Now().UTC().Format(time.RFC3339),
		"payload": map[string]interface{}{
			"call_id": callID,
			"reason":  reason,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	callChannel := fmt.Sprintf("channel:call:%s", callID)
	return s.publisher.Publish(ctx, callChannel, data)
}

// GenerateTURNCredentials generates temporary TURN server credentials
func (s *SignalingStore) GenerateTURNCredentials(ctx context.Context, userID string, turnSecret string, ttl time.Duration) (string, string, time.Time) {
	// Simple time-limited TURN credential generation
	// In production, use proper TURN REST API or coturn's turnutils
	timestamp := time.Now().Add(ttl).Unix()
	username := fmt.Sprintf("%d:%s", timestamp, userID)

	// HMAC-SHA1 would be used here with turnSecret to generate password
	// For now, return placeholder - implement proper TURN auth in production
	password := fmt.Sprintf("turn_%s_%d", uuid.New().String()[:8], timestamp)

	return username, password, time.Now().Add(ttl)
}

// GetActiveCallsForUser returns active calls the user is part of
func (s *SignalingStore) GetActiveCallsForUser(ctx context.Context, userID string) ([]string, error) {
	// This would require maintaining an index of user -> calls
	// For simplicity, return empty - use the database for this query
	return nil, nil
}

// ExtendCallTTL extends the TTL for call state
func (s *SignalingStore) ExtendCallTTL(ctx context.Context, callID string, ttl time.Duration) error {
	key := signalingCallStateKey + callID
	return s.client.Expire(ctx, key, ttl).Err()
}
