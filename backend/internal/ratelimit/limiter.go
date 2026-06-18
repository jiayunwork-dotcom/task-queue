package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"task-queue/internal/cache"
)

const (
	windowPrecision = 100 * time.Millisecond
)

type RateLimiter struct {
	cache      *cache.Cache
	configMgr  *ConfigManager
	waiter     *WaitQueue
	stats      *StatsCollector

	mu         sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewRateLimiter(cache *cache.Cache, configMgr *ConfigManager, waiter *WaitQueue, stats *StatsCollector) *RateLimiter {
	return &RateLimiter{
		cache:     cache,
		configMgr: configMgr,
		waiter:    waiter,
		stats:     stats,
		stopCh:    make(chan struct{}),
	}
}

func (rl *RateLimiter) Start(ctx context.Context) {
	rl.wg.Add(2)
	go rl.cleanupLoop(ctx)
	go rl.releaseLoop(ctx)
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.wg.Wait()
}

func (rl *RateLimiter) TryAcquire(ctx context.Context, taskType string, taskID uuid.UUID) (bool, error) {
	cfg := rl.configMgr.GetConfig(taskType)
	if cfg == nil || cfg.MaxPerSecond <= 0 {
		return true, nil
	}

	now := time.Now()
	windowStart := now.Add(-time.Duration(cfg.WindowSizeMs) * time.Millisecond)

	allowed, err := rl.checkAndRecord(ctx, taskType, taskID, now, windowStart, cfg)
	if err != nil {
		return false, err
	}

	if allowed {
		rl.stats.RecordExecution(taskType, now)
		return true, nil
	}

	rl.stats.RecordThrottled(taskType, now)
	rl.waiter.Enqueue(taskType, taskID, now)
	return false, nil
}

func (rl *RateLimiter) checkAndRecord(ctx context.Context, taskType string, taskID uuid.UUID,
	now, windowStart time.Time, cfg *RateLimitConfig) (bool, error) {

	key := executionWindowKey(taskType)
	score := float64(now.UnixNano()) / 1e6
	member := fmt.Sprintf("%d:%s", now.UnixNano(), taskID.String())

	pipe := rl.cache.Client.TxPipeline()

	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", float64(windowStart.UnixNano())/1e6))
	pipe.ZCard(ctx, key)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("redis pipeline exec: %w", err)
	}

	if len(results) < 2 {
		return false, fmt.Errorf("unexpected pipeline result length")
	}

	zcardCmd, ok := results[1].(*redis.IntCmd)
	if !ok {
		return false, fmt.Errorf("unexpected zcard result type")
	}

	count, err := zcardCmd.Result()
	if err != nil {
		return false, fmt.Errorf("zcard result: %w", err)
	}

	maxAllowed := int64(float64(cfg.MaxPerSecond) * float64(cfg.WindowSizeMs) / 1000.0)
	if count >= maxAllowed {
		return false, nil
	}

	err = rl.cache.Client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
	if err != nil {
		return false, fmt.Errorf("zadd execution record: %w", err)
	}

	ttl := time.Duration(cfg.WindowSizeMs+1000) * time.Millisecond
	rl.cache.Client.Expire(ctx, key, ttl)

	return true, nil
}

func (rl *RateLimiter) cleanupLoop(ctx context.Context) {
	defer rl.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanupExpiredWindows(ctx)
		}
	}
}

func (rl *RateLimiter) cleanupExpiredWindows(ctx context.Context) {
	configs := rl.configMgr.GetAllConfigs()
	for taskType, cfg := range configs {
		if cfg.MaxPerSecond <= 0 {
			continue
		}
		key := executionWindowKey(taskType)
		cutoff := float64(time.Now().Add(-time.Duration(cfg.WindowSizeMs+1000)*time.Millisecond).UnixNano()) / 1e6
		_ = rl.cache.Client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", cutoff)).Err()
	}
}

func (rl *RateLimiter) releaseLoop(ctx context.Context) {
	defer rl.wg.Done()
	ticker := time.NewTicker(windowPrecision)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.tryReleaseWaiting(ctx)
		}
	}
}

func (rl *RateLimiter) tryReleaseWaiting(ctx context.Context) {
	now := time.Now()
	taskTypes := rl.waiter.GetTaskTypes()

	for _, taskType := range taskTypes {
		cfg := rl.configMgr.GetConfig(taskType)
		if cfg == nil || cfg.MaxPerSecond <= 0 {
			rl.waiter.ReleaseAll(taskType)
			continue
		}

		windowStart := now.Add(-time.Duration(cfg.WindowSizeMs) * time.Millisecond)
		key := executionWindowKey(taskType)

		count, err := rl.cache.Client.ZCount(ctx, key,
			fmt.Sprintf("%f", float64(windowStart.UnixNano())/1e6),
			"+inf").Result()
		if err != nil {
			continue
		}

		maxAllowed := int64(float64(cfg.MaxPerSecond) * float64(cfg.WindowSizeMs) / 1000.0)
		available := maxAllowed - count
		if available <= 0 {
			continue
		}

		toRelease := rl.waiter.Dequeue(taskType, int(available))
		for _, taskID := range toRelease {
			score := float64(now.UnixNano()) / 1e6
			member := fmt.Sprintf("%d:%s", now.UnixNano(), taskID.String())
			_ = rl.cache.Client.ZAdd(ctx, key, redis.Z{
				Score:  score,
				Member: member,
			}).Err()
			rl.stats.RecordExecution(taskType, now)
		}
	}
}

func (rl *RateLimiter) GetReleaseChan() <-chan uuid.UUID {
	return rl.waiter.ReleaseChan()
}

func (rl *RateLimiter) GetCurrentRate(ctx context.Context, taskType string) (float64, error) {
	cfg := rl.configMgr.GetConfig(taskType)
	if cfg == nil || cfg.MaxPerSecond <= 0 {
		return 0, nil
	}

	now := time.Now()
	windowStart := now.Add(-time.Duration(cfg.WindowSizeMs) * time.Millisecond)
	key := executionWindowKey(taskType)

	count, err := rl.cache.Client.ZCount(ctx, key,
		fmt.Sprintf("%f", float64(windowStart.UnixNano())/1e6),
		"+inf").Result()
	if err != nil {
		return 0, err
	}

	rate := float64(count) / (float64(cfg.WindowSizeMs) / 1000.0)
	return rate, nil
}

func executionWindowKey(taskType string) string {
	return fmt.Sprintf("%s%s", cache.KeyPrefixRateLimitWindow, taskType)
}
