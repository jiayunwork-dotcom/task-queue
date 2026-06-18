package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"task-queue/internal/config"
)

type Cache struct {
	Client *redis.Client
}

const (
	KeyPrefixReadyQueue     = "tq:ready:"
	KeyPrefixDelayedQueue   = "tq:delayed"
	KeyPrefixDeadLetter     = "tq:dead_letter"
	KeyPrefixWorkerSlots    = "tq:worker:slots:"
	KeyPrefixWorkerRunning  = "tq:worker:running:"
	KeyPrefixTaskLease      = "tq:task:lease:"
	KeyPrefixHandlers       = "tq:handlers:"
	KeyPrefixDAGState       = "tq:dag:state:"
	KeyConsecutiveHighCount = "tq:consecutive_high"
	KeyThroughputCounter    = "tq:throughput:counter"
	KeyThroughputWindow     = "tq:throughput:window"
	KeyLatencySamples     = "tq:latency:samples"

	KeyPrefixRateLimitWindow   = "tq:ratelimit:window:"
	KeyPrefixRateLimitConfig = "tq:ratelimit:config:"
	KeyPrefixRateLimitThrottle = "tq:ratelimit:throttle:"

	ChannelRateLimitConfig = "tq:ratelimit:config:channel"
)

func New(cfg *config.RedisConfig) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		MinIdleConns: cfg.PoolSize / 10,
		MaxRetries:   3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Cache{Client: client}, nil
}

func (c *Cache) Close() error {
	return c.Client.Close()
}

func ReadyQueueKey(priority int) string {
	return fmt.Sprintf("%s%d", KeyPrefixReadyQueue, priority)
}
