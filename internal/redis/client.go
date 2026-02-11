package redis

import (
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// Singleton instance variables
var (
	client     *redis.Client
	clientOnce sync.Once
	clientCfg  Config
)

// Initialize initializes the global Redis client singleton with the specified configuration.
// This function is safe to call multiple times - only the first call will create the client.
// Must be called once at application startup before using GetClient().
func Initialize(cfg Config) {
	clientOnce.Do(func() {
		clientCfg = cfg
		client = NewClient(cfg)
	})
}

// GetClient returns the singleton Redis client instance.
// Panics if Initialize() has not been called.
// This is the recommended way to access the Redis client in application code.
func GetClient() *redis.Client {
	if client == nil {
		panic("redis client not initialized. Call Initialize() first")
	}
	return client
}

// IsInitialized returns true if the Redis client has been initialized
func IsInitialized() bool {
	return client != nil
}

// GetConfig returns the configuration used to initialize the Redis client
func GetConfig() Config {
	return clientCfg
}

// NewClient creates a new Redis client instance (not singleton - use for testing/multiple instances).
// For the global singleton client, use Initialize() and GetClient() instead.
func NewClient(cfg Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
