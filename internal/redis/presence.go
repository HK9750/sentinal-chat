package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// PresenceStatus represents a user's online status
type PresenceStatus struct {
	UserID   string    `json:"user_id"`
	IsOnline bool      `json:"is_online"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"` // online, away, busy, offline
	DeviceID string    `json:"device_id,omitempty"`
	ClientID string    `json:"client_id,omitempty"`
}

// PresenceStore handles presence tracking in Redis
type PresenceStore struct {
	client    *goredis.Client
	publisher *Publisher
	ttl       time.Duration
}

// Redis key prefixes for presence
const (
	presenceKeyPrefix    = "presence:"           // Hash storing user presence data
	presenceOnlineSet    = "presence:online"     // Set of online user IDs
	presenceHeartbeatKey = "presence:heartbeat:" // Sorted set for heartbeat timestamps
)

// NewPresenceStore creates a new presence store
func NewPresenceStore(client *goredis.Client, publisher *Publisher, ttl time.Duration) *PresenceStore {
	if ttl == 0 {
		ttl = 5 * time.Minute // Default TTL for presence data
	}
	return &PresenceStore{
		client:    client,
		publisher: publisher,
		ttl:       ttl,
	}
}

// SetOnline marks a user as online
func (p *PresenceStore) SetOnline(ctx context.Context, userID, deviceID, clientID string) error {
	now := time.Now()
	status := PresenceStatus{
		UserID:   userID,
		IsOnline: true,
		LastSeen: now,
		Status:   "online",
		DeviceID: deviceID,
		ClientID: clientID,
	}

	pipe := p.client.Pipeline()

	// Store presence data in hash
	key := presenceKeyPrefix + userID
	data, _ := json.Marshal(status)
	pipe.Set(ctx, key, data, p.ttl)

	// Add to online users set
	pipe.SAdd(ctx, presenceOnlineSet, userID)

	// Update heartbeat timestamp (for cleanup)
	pipe.ZAdd(ctx, presenceHeartbeatKey+"all", goredis.Z{
		Score:  float64(now.Unix()),
		Member: userID,
	})

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	// Publish presence event
	return p.publishPresenceEvent(ctx, userID, true, "online", now)
}

// SetOffline marks a user as offline
func (p *PresenceStore) SetOffline(ctx context.Context, userID string) error {
	now := time.Now()

	pipe := p.client.Pipeline()

	// Update presence data
	key := presenceKeyPrefix + userID
	status := PresenceStatus{
		UserID:   userID,
		IsOnline: false,
		LastSeen: now,
		Status:   "offline",
	}
	data, _ := json.Marshal(status)
	pipe.Set(ctx, key, data, 24*time.Hour) // Keep offline status longer for last_seen queries

	// Remove from online users set
	pipe.SRem(ctx, presenceOnlineSet, userID)

	// Remove from heartbeat tracking
	pipe.ZRem(ctx, presenceHeartbeatKey+"all", userID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	// Publish presence event
	return p.publishPresenceEvent(ctx, userID, false, "offline", now)
}

// UpdateStatus updates a user's status (online, away, busy)
func (p *PresenceStore) UpdateStatus(ctx context.Context, userID, status string) error {
	now := time.Now()

	key := presenceKeyPrefix + userID
	existing, err := p.client.Get(ctx, key).Result()
	if err != nil && err != goredis.Nil {
		return err
	}

	var presenceStatus PresenceStatus
	if existing != "" {
		json.Unmarshal([]byte(existing), &presenceStatus)
	}

	presenceStatus.UserID = userID
	presenceStatus.Status = status
	presenceStatus.LastSeen = now
	presenceStatus.IsOnline = status != "offline"

	data, _ := json.Marshal(presenceStatus)
	if err := p.client.Set(ctx, key, data, p.ttl).Err(); err != nil {
		return err
	}

	// Update online set based on status
	if status == "offline" {
		p.client.SRem(ctx, presenceOnlineSet, userID)
	} else {
		p.client.SAdd(ctx, presenceOnlineSet, userID)
	}

	// Publish presence event
	return p.publishPresenceEvent(ctx, userID, status != "offline", status, now)
}

// Heartbeat updates the user's heartbeat to prevent timeout
func (p *PresenceStore) Heartbeat(ctx context.Context, userID string) error {
	now := time.Now()

	pipe := p.client.Pipeline()

	// Refresh presence TTL
	key := presenceKeyPrefix + userID
	pipe.Expire(ctx, key, p.ttl)

	// Update heartbeat timestamp
	pipe.ZAdd(ctx, presenceHeartbeatKey+"all", goredis.Z{
		Score:  float64(now.Unix()),
		Member: userID,
	})

	_, err := pipe.Exec(ctx)
	return err
}

// GetPresence gets the presence status of a user
func (p *PresenceStore) GetPresence(ctx context.Context, userID string) (*PresenceStatus, error) {
	key := presenceKeyPrefix + userID
	data, err := p.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return &PresenceStatus{
			UserID:   userID,
			IsOnline: false,
			Status:   "offline",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var status PresenceStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetMultiplePresence gets presence status for multiple users
func (p *PresenceStore) GetMultiplePresence(ctx context.Context, userIDs []string) (map[string]*PresenceStatus, error) {
	result := make(map[string]*PresenceStatus)

	if len(userIDs) == 0 {
		return result, nil
	}

	pipe := p.client.Pipeline()
	cmds := make(map[string]*goredis.StringCmd)

	for _, userID := range userIDs {
		key := presenceKeyPrefix + userID
		cmds[userID] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != goredis.Nil {
		// Some keys might not exist, which is fine
	}

	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err == goredis.Nil {
			result[userID] = &PresenceStatus{
				UserID:   userID,
				IsOnline: false,
				Status:   "offline",
			}
			continue
		}
		if err != nil {
			result[userID] = &PresenceStatus{
				UserID:   userID,
				IsOnline: false,
				Status:   "offline",
			}
			continue
		}

		var status PresenceStatus
		if err := json.Unmarshal([]byte(data), &status); err != nil {
			result[userID] = &PresenceStatus{
				UserID:   userID,
				IsOnline: false,
				Status:   "offline",
			}
			continue
		}
		result[userID] = &status
	}

	return result, nil
}

// IsOnline checks if a user is online
func (p *PresenceStore) IsOnline(ctx context.Context, userID string) (bool, error) {
	return p.client.SIsMember(ctx, presenceOnlineSet, userID).Result()
}

// GetOnlineUsers returns all online user IDs
func (p *PresenceStore) GetOnlineUsers(ctx context.Context) ([]string, error) {
	return p.client.SMembers(ctx, presenceOnlineSet).Result()
}

// GetOnlineCount returns the count of online users
func (p *PresenceStore) GetOnlineCount(ctx context.Context) (int64, error) {
	return p.client.SCard(ctx, presenceOnlineSet).Result()
}

// CleanupStalePresence removes presence data for users who haven't sent a heartbeat
func (p *PresenceStore) CleanupStalePresence(ctx context.Context, maxAge time.Duration) (int64, error) {
	threshold := time.Now().Add(-maxAge).Unix()

	// Get stale users (heartbeat older than threshold)
	staleUsers, err := p.client.ZRangeByScore(ctx, presenceHeartbeatKey+"all", &goredis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(threshold, 10),
	}).Result()
	if err != nil {
		return 0, err
	}

	if len(staleUsers) == 0 {
		return 0, nil
	}

	// Mark each stale user as offline
	for _, userID := range staleUsers {
		p.SetOffline(ctx, userID)
	}

	return int64(len(staleUsers)), nil
}

// publishPresenceEvent publishes a presence change event to Redis pub/sub
func (p *PresenceStore) publishPresenceEvent(ctx context.Context, userID string, isOnline bool, status string, timestamp time.Time) error {
	if p.publisher == nil {
		return nil
	}

	eventType := "presence.offline"
	if isOnline {
		eventType = "presence.online"
	}

	event := map[string]interface{}{
		"event_type":     eventType,
		"aggregate_type": "presence",
		"aggregate_id":   userID,
		"occurred_at":    timestamp.UTC().Format(time.RFC3339),
		"payload": map[string]interface{}{
			"user_id":   userID,
			"is_online": isOnline,
			"status":    status,
			"timestamp": timestamp.UTC().Format(time.RFC3339),
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	channel := fmt.Sprintf("channel:presence:%s", userID)
	return p.publisher.Publish(ctx, channel, data)
}

// TrackTyping sets a typing indicator for a user in a conversation
func (p *PresenceStore) TrackTyping(ctx context.Context, conversationID, userID string, isTyping bool) error {
	key := fmt.Sprintf("typing:%s", conversationID)

	if isTyping {
		// Add user to typing set with expiry
		pipe := p.client.Pipeline()
		pipe.SAdd(ctx, key, userID)
		pipe.Expire(ctx, key, 10*time.Second) // Typing indicator expires after 10 seconds
		_, err := pipe.Exec(ctx)
		return err
	}

	// Remove user from typing set
	return p.client.SRem(ctx, key, userID).Err()
}

// GetTypingUsers returns users currently typing in a conversation
func (p *PresenceStore) GetTypingUsers(ctx context.Context, conversationID string) ([]string, error) {
	key := fmt.Sprintf("typing:%s", conversationID)
	return p.client.SMembers(ctx, key).Result()
}

// TrackUserConnection tracks a user's WebSocket connection
func (p *PresenceStore) TrackUserConnection(ctx context.Context, userID, clientID, deviceID string) error {
	key := fmt.Sprintf("connections:%s", userID)
	connectionData := map[string]interface{}{
		"client_id":    clientID,
		"device_id":    deviceID,
		"connected_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(connectionData)

	pipe := p.client.Pipeline()
	pipe.HSet(ctx, key, clientID, data)
	pipe.Expire(ctx, key, p.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// RemoveUserConnection removes a user's WebSocket connection tracking
func (p *PresenceStore) RemoveUserConnection(ctx context.Context, userID, clientID string) error {
	key := fmt.Sprintf("connections:%s", userID)

	// Remove this connection
	if err := p.client.HDel(ctx, key, clientID).Err(); err != nil {
		return err
	}

	// Check if user has any remaining connections
	count, err := p.client.HLen(ctx, key).Result()
	if err != nil {
		return err
	}

	// If no connections remain, mark user offline
	if count == 0 {
		return p.SetOffline(ctx, userID)
	}

	return nil
}

// GetUserConnections returns all active connections for a user
func (p *PresenceStore) GetUserConnections(ctx context.Context, userID string) ([]map[string]string, error) {
	key := fmt.Sprintf("connections:%s", userID)
	data, err := p.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var connections []map[string]string
	for clientID, connData := range data {
		var conn map[string]string
		if err := json.Unmarshal([]byte(connData), &conn); err != nil {
			continue
		}
		conn["client_id"] = clientID
		connections = append(connections, conn)
	}

	return connections, nil
}

// GetUserConnectionCount returns the number of active connections for a user
func (p *PresenceStore) GetUserConnectionCount(ctx context.Context, userID string) (int64, error) {
	key := fmt.Sprintf("connections:%s", userID)
	return p.client.HLen(ctx, key).Result()
}

// LastSeenKey generates the key for storing last seen time
func LastSeenKey(userID uuid.UUID) string {
	return fmt.Sprintf("last_seen:%s", userID.String())
}
