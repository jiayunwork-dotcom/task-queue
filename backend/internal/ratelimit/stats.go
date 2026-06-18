package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"task-queue/internal/cache"
)

type throttleRecord struct {
	timestamp time.Time
	count     int64
}

type StatsCollector struct {
	cache *cache.Cache

	throttleHistory map[string][]throttleRecord
	throttleMu      sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewStatsCollector(cache *cache.Cache) *StatsCollector {
	return &StatsCollector{
		cache:           cache,
		throttleHistory: make(map[string][]throttleRecord),
		stopCh:          make(chan struct{}),
	}
}

func (sc *StatsCollector) Start(ctx context.Context) {
	sc.wg.Add(1)
	go sc.persistLoop(ctx)
}

func (sc *StatsCollector) Stop() {
	close(sc.stopCh)
	sc.wg.Wait()
}

func (sc *StatsCollector) RecordExecution(taskType string, now time.Time) {
	_ = taskType
	_ = now
}

func (sc *StatsCollector) RecordThrottled(taskType string, now time.Time) {
	sc.throttleMu.Lock()
	defer sc.throttleMu.Unlock()

	rounded := now.Truncate(time.Minute)
	records := sc.throttleHistory[taskType]

	if len(records) > 0 && records[len(records)-1].timestamp.Equal(rounded) {
		records[len(records)-1].count++
	} else {
		records = append(records, throttleRecord{
			timestamp: rounded,
			count:     1,
		})
	}

	cutoff := now.Add(-2 * time.Hour)
	filtered := records[:0]
	for _, r := range records {
		if !r.timestamp.Before(cutoff) {
			filtered = append(filtered, r)
		}
	}
	sc.throttleHistory[taskType] = filtered
}

func (sc *StatsCollector) persistLoop(ctx context.Context) {
	defer sc.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sc.stopCh:
			return
		case <-ticker.C:
			sc.persistThrottleStats(ctx)
		}
	}
}

func (sc *StatsCollector) persistThrottleStats(ctx context.Context) {
	sc.throttleMu.Lock()
	records := make(map[string][]throttleRecord, len(sc.throttleHistory))
	for k, v := range sc.throttleHistory {
		records[k] = make([]throttleRecord, len(v))
		copy(records[k], v)
	}
	sc.throttleMu.Unlock()

	for taskType, recs := range records {
		for _, r := range recs {
			key := throttleStatsKey(taskType)
			score := float64(r.timestamp.Unix())
			member := fmt.Sprintf("%d:%d", r.timestamp.Unix(), r.count)

			_ = sc.cache.Client.ZAdd(ctx, key, redis.Z{
				Score:  score,
				Member: member,
			}).Err()

			_ = sc.cache.Client.Expire(ctx, key, 24*time.Hour).Err()
		}
	}

	cutoff := float64(time.Now().Add(-24 * time.Hour).Unix())
	pattern := throttleStatsKey("*")
	iter := sc.cache.Client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		_ = sc.cache.Client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", cutoff)).Err()
	}
}

func (sc *StatsCollector) GetThrottleCount(ctx context.Context, taskType string, window time.Duration) (int64, error) {
	key := throttleStatsKey(taskType)
	cutoff := float64(time.Now().Add(-window).Unix())

	result, err := sc.cache.Client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", cutoff),
		Max: "+inf",
	}).Result()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, s := range result {
		var ts int64
		var count int64
		if _, err := fmt.Sscanf(s, "%d:%d", &ts, &count); err == nil {
			total += count
		}
	}

	sc.throttleMu.RLock()
	if recs, ok := sc.throttleHistory[taskType]; ok {
		for _, r := range recs {
			if !r.timestamp.Before(time.Now().Add(-window)) {
				total += r.count
			}
		}
	}
	sc.throttleMu.RUnlock()

	return total, nil
}

func (sc *StatsCollector) GetThrottleCounts(ctx context.Context, window time.Duration) (map[string]int64, error) {
	pattern := throttleStatsKey("*")
	iter := sc.cache.Client.Scan(ctx, 0, pattern, 100).Iterator()

	result := make(map[string]int64)
	taskTypes := make(map[string]bool)

	for iter.Next(ctx) {
		key := iter.Val()
		taskType := key[len(cache.KeyPrefixRateLimitThrottle):]
		taskTypes[taskType] = true

		count, err := sc.GetThrottleCount(ctx, taskType, window)
		if err == nil {
			result[taskType] = count
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	sc.throttleMu.RLock()
	for tt := range sc.throttleHistory {
		taskTypes[tt] = true
	}
	sc.throttleMu.RUnlock()

	for tt := range taskTypes {
		if _, exists := result[tt]; !exists {
			count, _ := sc.GetThrottleCount(ctx, tt, window)
			result[tt] = count
		}
	}

	return result, nil
}

func throttleStatsKey(taskType string) string {
	return fmt.Sprintf("%s%s", cache.KeyPrefixRateLimitThrottle, taskType)
}
