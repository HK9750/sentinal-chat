package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"sentinal-chat/config"
)

type Client struct {
	client *goredis.Client
}

func NewRedis(cfg *config.Config) (*Client, error) {
	if cfg == nil || strings.TrimSpace(cfg.RedisHost) == "" {
		return nil, fmt.Errorf("redis config missing")
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password:     cfg.RedisPassword,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

func (r *Client) Publish(ctx context.Context, channel string, payload []byte) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Publish(ctx, channel, payload).Err()
}

func (r *Client) Subscribe(ctx context.Context, channels ...string) *goredis.PubSub {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Subscribe(ctx, channels...)
}

func (r *Client) PSubscribe(ctx context.Context, patterns ...string) *goredis.PubSub {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.PSubscribe(ctx, patterns...)
}

func (r *Client) SetPresence(ctx context.Context, userID string, ttl time.Duration) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Set(ctx, "presence:"+userID, time.Now().UTC().Format(time.RFC3339Nano), ttl).Err()
}

func (r *Client) ClearPresence(ctx context.Context, userID string) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Del(ctx, "presence:"+userID).Err()
}

func (r *Client) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}
