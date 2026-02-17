package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sentinal-chat/internal/domain/conversation"
	"sentinal-chat/internal/domain/user"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// Cache key patterns (from database.md Appendix H):
// - session:{session_id} - 15m TTL, refresh on activity
// - user:{user_id} - 5m TTL, profile cache
// - conversation:{conv_id} - 5m TTL, metadata cache
// - conversation:{conv_id}:participants - 5m TTL, participants cache

// CacheConfig contains configuration for caching
type CacheConfig struct {
	SessionTTL      time.Duration // TTL for session cache (default 15m)
	UserTTL         time.Duration // TTL for user cache (default 5m)
	ConversationTTL time.Duration // TTL for conversation cache (default 5m)
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		SessionTTL:      15 * time.Minute,
		UserTTL:         5 * time.Minute,
		ConversationTTL: 5 * time.Minute,
	}
}

// CacheStore handles caching in Redis
type CacheStore struct {
	client *goredis.Client
	config CacheConfig
}

// NewCacheStore creates a new cache store
func NewCacheStore(client *goredis.Client, config CacheConfig) *CacheStore {
	return &CacheStore{
		client: client,
		config: config,
	}
}

// --- Session Cache ---

// SessionCache represents cached session data
type SessionCache struct {
	SessionID  uuid.UUID `json:"session_id"`
	UserID     uuid.UUID `json:"user_id"`
	DeviceID   string    `json:"device_id,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastActive time.Time `json:"last_active"`
}

// GetSession retrieves a session from cache
func (c *CacheStore) GetSession(ctx context.Context, sessionID uuid.UUID) (*SessionCache, error) {
	key := fmt.Sprintf("session:%s", sessionID.String())
	data, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var session SessionCache
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SetSession stores a session in cache
func (c *CacheStore) SetSession(ctx context.Context, session *SessionCache) error {
	key := fmt.Sprintf("session:%s", session.SessionID.String())
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.config.SessionTTL).Err()
}

// SetSessionFromEntity stores a session from the domain entity
func (c *CacheStore) SetSessionFromEntity(ctx context.Context, session *user.UserSession) error {
	deviceID := ""
	if session.DeviceID != nil {
		deviceID = session.DeviceID.String()
	}
	cached := &SessionCache{
		SessionID:  session.ID,
		UserID:     session.UserID,
		DeviceID:   deviceID,
		ExpiresAt:  session.ExpiresAt,
		LastActive: time.Now(),
	}
	return c.SetSession(ctx, cached)
}

// RefreshSession extends the session TTL (call on activity)
func (c *CacheStore) RefreshSession(ctx context.Context, sessionID uuid.UUID) error {
	key := fmt.Sprintf("session:%s", sessionID.String())
	return c.client.Expire(ctx, key, c.config.SessionTTL).Err()
}

// InvalidateSession removes a session from cache
func (c *CacheStore) InvalidateSession(ctx context.Context, sessionID uuid.UUID) error {
	key := fmt.Sprintf("session:%s", sessionID.String())
	return c.client.Del(ctx, key).Err()
}

// InvalidateUserSessions removes all sessions for a user from cache
// Note: This requires scanning, so use sparingly
func (c *CacheStore) InvalidateUserSessions(ctx context.Context, userID uuid.UUID) error {
	// This is an expensive operation; in production, consider maintaining a set of session IDs per user
	pattern := "session:*"
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keysToDelete []string
	for iter.Next(ctx) {
		key := iter.Val()
		data, err := c.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var session SessionCache
		if err := json.Unmarshal([]byte(data), &session); err != nil {
			continue
		}

		if session.UserID == userID {
			keysToDelete = append(keysToDelete, key)
		}
	}

	if len(keysToDelete) > 0 {
		return c.client.Del(ctx, keysToDelete...).Err()
	}
	return nil
}

// --- User Cache ---

// UserCache represents cached user data (subset for performance)
type UserCache struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username,omitempty"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	IsOnline    bool      `json:"is_online"`
	LastSeenAt  time.Time `json:"last_seen_at,omitempty"`
	Role        string    `json:"role"`
}

// GetUser retrieves a user from cache
func (c *CacheStore) GetUser(ctx context.Context, userID uuid.UUID) (*UserCache, error) {
	key := fmt.Sprintf("user:%s", userID.String())
	data, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var u UserCache
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SetUser stores a user in cache
func (c *CacheStore) SetUser(ctx context.Context, u *UserCache) error {
	key := fmt.Sprintf("user:%s", u.ID.String())
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.config.UserTTL).Err()
}

// SetUserFromEntity stores a user from the domain entity
func (c *CacheStore) SetUserFromEntity(ctx context.Context, u *user.User) error {
	username := ""
	if u.Username.Valid {
		username = u.Username.String
	}
	var lastSeen time.Time
	if u.LastSeenAt.Valid {
		lastSeen = u.LastSeenAt.Time
	}
	cached := &UserCache{
		ID:          u.ID,
		Username:    username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		IsOnline:    u.IsOnline,
		LastSeenAt:  lastSeen,
		Role:        u.Role,
	}
	return c.SetUser(ctx, cached)
}

// InvalidateUser removes a user from cache
func (c *CacheStore) InvalidateUser(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("user:%s", userID.String())
	return c.client.Del(ctx, key).Err()
}

// GetMultipleUsers retrieves multiple users from cache
func (c *CacheStore) GetMultipleUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*UserCache, []uuid.UUID, error) {
	result := make(map[uuid.UUID]*UserCache)
	var misses []uuid.UUID

	if len(userIDs) == 0 {
		return result, misses, nil
	}

	pipe := c.client.Pipeline()
	cmds := make(map[uuid.UUID]*goredis.StringCmd)

	for _, userID := range userIDs {
		key := fmt.Sprintf("user:%s", userID.String())
		cmds[userID] = pipe.Get(ctx, key)
	}

	_, _ = pipe.Exec(ctx)

	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err == goredis.Nil {
			misses = append(misses, userID)
			continue
		}
		if err != nil {
			misses = append(misses, userID)
			continue
		}

		var u UserCache
		if err := json.Unmarshal([]byte(data), &u); err != nil {
			misses = append(misses, userID)
			continue
		}
		result[userID] = &u
	}

	return result, misses, nil
}

// --- Conversation Cache ---

// ConversationCache represents cached conversation data
type ConversationCache struct {
	ID               uuid.UUID `json:"id"`
	Type             string    `json:"type"`
	Subject          string    `json:"subject,omitempty"`
	Description      string    `json:"description,omitempty"`
	AvatarURL        string    `json:"avatar_url,omitempty"`
	DisappearingMode string    `json:"disappearing_mode"`
	CreatedBy        uuid.UUID `json:"created_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// GetConversation retrieves a conversation from cache
func (c *CacheStore) GetConversation(ctx context.Context, conversationID uuid.UUID) (*ConversationCache, error) {
	key := fmt.Sprintf("conversation:%s", conversationID.String())
	data, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var conv ConversationCache
	if err := json.Unmarshal([]byte(data), &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

// SetConversation stores a conversation in cache
func (c *CacheStore) SetConversation(ctx context.Context, conv *ConversationCache) error {
	key := fmt.Sprintf("conversation:%s", conv.ID.String())
	data, err := json.Marshal(conv)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.config.ConversationTTL).Err()
}

// SetConversationFromEntity stores a conversation from the domain entity
func (c *CacheStore) SetConversationFromEntity(ctx context.Context, conv *conversation.Conversation) error {
	subject := ""
	if conv.Subject.Valid {
		subject = conv.Subject.String
	}
	description := ""
	if conv.Description.Valid {
		description = conv.Description.String
	}
	avatarURL := ""
	if conv.AvatarURL.Valid {
		avatarURL = conv.AvatarURL.String
	}
	var createdBy uuid.UUID
	if conv.CreatedBy.Valid {
		createdBy = conv.CreatedBy.UUID
	}

	cached := &ConversationCache{
		ID:               conv.ID,
		Type:             conv.Type,
		Subject:          subject,
		Description:      description,
		AvatarURL:        avatarURL,
		DisappearingMode: conv.DisappearingMode,
		CreatedBy:        createdBy,
		CreatedAt:        conv.CreatedAt,
	}
	return c.SetConversation(ctx, cached)
}

// InvalidateConversation removes a conversation from cache
func (c *CacheStore) InvalidateConversation(ctx context.Context, conversationID uuid.UUID) error {
	keys := []string{
		fmt.Sprintf("conversation:%s", conversationID.String()),
		fmt.Sprintf("conversation:%s:participants", conversationID.String()),
	}
	return c.client.Del(ctx, keys...).Err()
}

// --- Conversation Participants Cache ---

// GetConversationParticipants retrieves participant IDs from cache
func (c *CacheStore) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	key := fmt.Sprintf("conversation:%s:participants", conversationID.String())
	data, err := c.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var participants []uuid.UUID
	if err := json.Unmarshal([]byte(data), &participants); err != nil {
		return nil, err
	}
	return participants, nil
}

// SetConversationParticipants stores participant IDs in cache
func (c *CacheStore) SetConversationParticipants(ctx context.Context, conversationID uuid.UUID, participantIDs []uuid.UUID) error {
	key := fmt.Sprintf("conversation:%s:participants", conversationID.String())
	data, err := json.Marshal(participantIDs)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.config.ConversationTTL).Err()
}

// InvalidateConversationParticipants removes participants from cache
func (c *CacheStore) InvalidateConversationParticipants(ctx context.Context, conversationID uuid.UUID) error {
	key := fmt.Sprintf("conversation:%s:participants", conversationID.String())
	return c.client.Del(ctx, key).Err()
}

// --- Utility Methods ---

// Ping checks if Redis is available
func (c *CacheStore) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// FlushAll clears all cache (use with caution!)
func (c *CacheStore) FlushAll(ctx context.Context) error {
	return c.client.FlushAll(ctx).Err()
}

// GetStats returns cache statistics
type CacheStats struct {
	SessionCount      int64 `json:"session_count"`
	UserCount         int64 `json:"user_count"`
	ConversationCount int64 `json:"conversation_count"`
}

func (c *CacheStore) GetStats(ctx context.Context) (*CacheStats, error) {
	stats := &CacheStats{}

	// Count sessions
	sessionIter := c.client.Scan(ctx, 0, "session:*", 0).Iterator()
	for sessionIter.Next(ctx) {
		stats.SessionCount++
	}

	// Count users
	userIter := c.client.Scan(ctx, 0, "user:*", 0).Iterator()
	for userIter.Next(ctx) {
		stats.UserCount++
	}

	// Count conversations
	convIter := c.client.Scan(ctx, 0, "conversation:*", 0).Iterator()
	for convIter.Next(ctx) {
		stats.ConversationCount++
	}

	return stats, nil
}
