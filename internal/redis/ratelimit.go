package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Rate limiting key patterns (from database.md Appendix H):
// - ratelimit:{user_id}:messages - 60s TTL, per-minute message limit
// - ratelimit:{user_id}:calls - 60s TTL, per-minute call limit
// - ratelimit:{ip}:auth - 60s TTL, per-minute auth attempts

// RateLimitConfig contains configuration for rate limiting
type RateLimitConfig struct {
	MessageLimit  int           // Max messages per window
	MessageWindow time.Duration // Message rate limit window
	CallLimit     int           // Max calls per window
	CallWindow    time.Duration // Call rate limit window
	AuthLimit     int           // Max auth attempts per window
	AuthWindow    time.Duration // Auth rate limit window
}

// DefaultRateLimitConfig returns sensible defaults
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MessageLimit:  60, // 60 messages per minute
		MessageWindow: 60 * time.Second,
		CallLimit:     10, // 10 calls per minute
		CallWindow:    60 * time.Second,
		AuthLimit:     5, // 5 auth attempts per minute
		AuthWindow:    60 * time.Second,
	}
}

// RateLimiter handles rate limiting using Redis
type RateLimiter struct {
	client *goredis.Client
	config RateLimitConfig
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool          // Whether the action is allowed
	Remaining int           // Remaining actions in the window
	ResetIn   time.Duration // Time until the window resets
	Limit     int           // The limit for this action
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *goredis.Client, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		client: client,
		config: config,
	}
}

// AllowMessage checks if a user can send a message
func (r *RateLimiter) AllowMessage(ctx context.Context, userID string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:messages", userID)
	return r.checkLimit(ctx, key, r.config.MessageLimit, r.config.MessageWindow)
}

// AllowCall checks if a user can initiate a call
func (r *RateLimiter) AllowCall(ctx context.Context, userID string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:calls", userID)
	return r.checkLimit(ctx, key, r.config.CallLimit, r.config.CallWindow)
}

// AllowAuth checks if an IP can make an auth attempt
func (r *RateLimiter) AllowAuth(ctx context.Context, ip string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:auth", ip)
	return r.checkLimit(ctx, key, r.config.AuthLimit, r.config.AuthWindow)
}

// checkLimit performs the actual rate limit check using a sliding window counter
func (r *RateLimiter) checkLimit(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	// Use Lua script for atomic increment and check
	script := goredis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		
		local current = redis.call('GET', key)
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		
		local ttl = redis.call('TTL', key)
		if ttl < 0 then
			ttl = window
		end
		
		if current < limit then
			redis.call('INCR', key)
			if ttl == window then
				redis.call('EXPIRE', key, window)
			end
			return {1, limit - current - 1, ttl}
		else
			return {0, 0, ttl}
		end
	`)

	result, err := script.Run(ctx, r.client, []string{key}, limit, int(window.Seconds())).Result()
	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	// Parse the result
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) < 3 {
		return nil, fmt.Errorf("unexpected rate limit result format")
	}

	allowed := resultSlice[0].(int64) == 1
	remaining := int(resultSlice[1].(int64))
	resetIn := time.Duration(resultSlice[2].(int64)) * time.Second

	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetIn:   resetIn,
		Limit:     limit,
	}, nil
}

// ConsumeMessage consumes a message quota (call after AllowMessage if sending succeeds)
// This is useful if you want to check first, then consume only on success
func (r *RateLimiter) ConsumeMessage(ctx context.Context, userID string) error {
	key := fmt.Sprintf("ratelimit:%s:messages", userID)
	return r.consume(ctx, key, r.config.MessageWindow)
}

// ConsumeCall consumes a call quota
func (r *RateLimiter) ConsumeCall(ctx context.Context, userID string) error {
	key := fmt.Sprintf("ratelimit:%s:calls", userID)
	return r.consume(ctx, key, r.config.CallWindow)
}

// ConsumeAuth consumes an auth attempt quota
func (r *RateLimiter) ConsumeAuth(ctx context.Context, ip string) error {
	key := fmt.Sprintf("ratelimit:%s:auth", ip)
	return r.consume(ctx, key, r.config.AuthWindow)
}

// consume increments the counter for a key
func (r *RateLimiter) consume(ctx context.Context, key string, window time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	return err
}

// Reset resets the rate limit for a specific key (admin operation)
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// ResetUser resets all rate limits for a user
func (r *RateLimiter) ResetUser(ctx context.Context, userID string) error {
	keys := []string{
		fmt.Sprintf("ratelimit:%s:messages", userID),
		fmt.Sprintf("ratelimit:%s:calls", userID),
	}
	return r.client.Del(ctx, keys...).Err()
}

// ResetAuth resets auth rate limit for an IP
func (r *RateLimiter) ResetAuth(ctx context.Context, ip string) error {
	key := fmt.Sprintf("ratelimit:%s:auth", ip)
	return r.client.Del(ctx, key).Err()
}

// GetStatus returns the current rate limit status without consuming
func (r *RateLimiter) GetStatus(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	pipe := r.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	_, _ = pipe.Exec(ctx)

	current := 0
	if val, err := getCmd.Int(); err == nil {
		current = val
	}

	ttl := window
	if ttlVal := ttlCmd.Val(); ttlVal > 0 {
		ttl = ttlVal
	}

	return &RateLimitResult{
		Allowed:   current < limit,
		Remaining: limit - current,
		ResetIn:   ttl,
		Limit:     limit,
	}, nil
}

// GetMessageStatus returns current message rate limit status
func (r *RateLimiter) GetMessageStatus(ctx context.Context, userID string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:messages", userID)
	return r.GetStatus(ctx, key, r.config.MessageLimit, r.config.MessageWindow)
}

// GetCallStatus returns current call rate limit status
func (r *RateLimiter) GetCallStatus(ctx context.Context, userID string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:calls", userID)
	return r.GetStatus(ctx, key, r.config.CallLimit, r.config.CallWindow)
}

// GetAuthStatus returns current auth rate limit status
func (r *RateLimiter) GetAuthStatus(ctx context.Context, ip string) (*RateLimitResult, error) {
	key := fmt.Sprintf("ratelimit:%s:auth", ip)
	return r.GetStatus(ctx, key, r.config.AuthLimit, r.config.AuthWindow)
}
