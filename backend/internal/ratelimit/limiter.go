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

var tryAcquireScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '0', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
if count >= tonumber(ARGV[2]) then
    return 0
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
`)

var releaseAndRecordScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '0', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
local maxAllowed = tonumber(ARGV[2])
local available = maxAllowed - count
if available <= 0 then
    return 0
end
local numPairs = math.floor((#ARGV - 3) / 2)
local n = math.min(available, numPairs)
for i = 1, n do
    redis.call('ZADD', KEYS[1], ARGV[2*i+1], ARGV[2*i+2])
end
redis.call('PEXPIRE', KEYS[1], ARGV[#ARGV])
return n
`)

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
	windowStartScore := float64(windowStart.UnixNano()) / 1e6
	maxAllowed := int64(float64(cfg.MaxPerSecond) * float64(cfg.WindowSizeMs) / 1000.0)
	ttlMs := int64(cfg.WindowSizeMs + 1000)

	result, err := tryAcquireScript.Run(ctx, rl.cache.Client,
		[]string{key},
		fmt.Sprintf("%f", windowStartScore),
		maxAllowed,
		fmt.Sprintf("%f", score),
		member,
		ttlMs,
	).Int64()

	if err != nil {
		return false, fmt.Errorf("lua tryAcquire: %w", err)
	}

	return result == 1, nil
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
		windowStartScore := float64(windowStart.UnixNano()) / 1e6
		maxAllowed := int64(float64(cfg.MaxPerSecond) * float64(cfg.WindowSizeMs) / 1000.0)

		pending := rl.waiter.PeekCount(taskType)
		if pending == 0 {
			continue
		}

		batchSize := pending
		if batchSize > int(maxAllowed) {
			batchSize = int(maxAllowed)
		}

		candidates := rl.waiter.Peek(taskType, batchSize)

		var args []interface{}
		args = append(args,
			fmt.Sprintf("%f", windowStartScore),
			maxAllowed,
		)

		for _, taskID := range candidates {
			score := float64(now.UnixNano())/1e6 + float64(len(args))*0.0001
			member := fmt.Sprintf("%d:%s", now.UnixNano(), taskID.String())
			args = append(args, fmt.Sprintf("%f", score), member)
		}

		ttlMs := int64(cfg.WindowSizeMs + 1000)
		args = append(args, ttlMs)

		released, err := releaseAndRecordScript.Run(ctx, rl.cache.Client,
			[]string{key}, args...,
		).Int64()

		if err != nil || released <= 0 {
			continue
		}

		releasedIDs := rl.waiter.Dequeue(taskType, int(released))
		for range releasedIDs {
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
