package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"task-queue/internal/cache"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/repository"
)

type Collector struct {
	cache        *cache.Cache
	workerRepo   *repository.WorkerRepository
	taskRepo     *repository.TaskRepository
	deadRepo     *repository.DeadLetterRepository
	scheduler    *queue.PriorityScheduler

	completedCount   int64
	failedCount      int64
	windowStart      time.Time
	latencySamples   []float64
	latencyMu        sync.Mutex

	prioritySuccess  map[models.Priority]int64
	priorityFail     map[models.Priority]int64
	priorityMu       sync.Mutex

	wg       sync.WaitGroup
	stopCh   chan struct{}
}

func NewCollector(
	cache *cache.Cache,
	workerRepo *repository.WorkerRepository,
	taskRepo *repository.TaskRepository,
	deadRepo *repository.DeadLetterRepository,
	scheduler *queue.PriorityScheduler,
) *Collector {
	return &Collector{
		cache:         cache,
		workerRepo:    workerRepo,
		taskRepo:      taskRepo,
		deadRepo:      deadRepo,
		scheduler:     scheduler,
		windowStart:   time.Now(),
		latencySamples: make([]float64, 0, 10000),
		prioritySuccess: make(map[models.Priority]int64),
		priorityFail:    make(map[models.Priority]int64),
		stopCh:        make(chan struct{}),
	}
}

func (c *Collector) RecordTaskComplete(priority models.Priority, durationMS int64, success bool) {
	if success {
		atomic.AddInt64(&c.completedCount, 1)
		c.priorityMu.Lock()
		c.prioritySuccess[priority]++
		c.priorityMu.Unlock()
	} else {
		atomic.AddInt64(&c.failedCount, 1)
		c.priorityMu.Lock()
		c.priorityFail[priority]++
		c.priorityMu.Unlock()
	}
	c.latencyMu.Lock()
	c.latencySamples = append(c.latencySamples, float64(durationMS))
	if len(c.latencySamples) > 100000 {
		c.latencySamples = c.latencySamples[50000:]
	}
	c.latencyMu.Unlock()
}

func (c *Collector) RecordLatency(durationMS int64) {
	c.latencyMu.Lock()
	c.latencySamples = append(c.latencySamples, float64(durationMS))
	if len(c.latencySamples) > 100000 {
		c.latencySamples = c.latencySamples[50000:]
	}
	c.latencyMu.Unlock()
}

func (c *Collector) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.collectLoop(ctx)
}

func (c *Collector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *Collector) collectLoop(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			snap := c.Snapshot(ctx)
			c.saveHistory(ctx, snap)
			c.resetCounters()
		}
	}
}

func (c *Collector) Snapshot(ctx context.Context) *models.MetricsSnapshot {
	depths, _ := c.scheduler.GetQueueDepths(ctx)
	if depths == nil {
		depths = map[models.Priority]int64{}
	}

	workers, _ := c.workerRepo.List(ctx)
	workersOnline := 0
	workersOffline := 0
	totalSlots := 0
	usedSlots := 0
	for _, w := range workers {
		if w.Status == models.WorkerStatusOnline || w.Status == models.WorkerStatusDraining {
			workersOnline++
		} else {
			workersOffline++
		}
		totalSlots += w.TotalSlots
		usedSlots += w.UsedSlots
	}

	utilization := 0.0
	if totalSlots > 0 {
		utilization = float64(usedSlots) / float64(totalSlots) * 100
	}

	elapsed := time.Since(c.windowStart).Seconds()
	throughput := 0.0
	if elapsed > 0 {
		throughput = float64(atomic.LoadInt64(&c.completedCount)) / elapsed
	}

	c.priorityMu.Lock()
	successRates := map[models.Priority]float64{}
	failureRates := map[models.Priority]float64{}
	priorities := []models.Priority{
		models.PriorityCritical, models.PriorityHigh, models.PriorityNormal,
		models.PriorityLow, models.PriorityBulk,
	}
	for _, p := range priorities {
		succ := c.prioritySuccess[p]
		fail := c.priorityFail[p]
		total := succ + fail
		if total > 0 {
			successRates[p] = float64(succ) / float64(total) * 100
			failureRates[p] = float64(fail) / float64(total) * 100
		} else {
			successRates[p] = 100.0
			failureRates[p] = 0.0
		}
	}
	c.priorityMu.Unlock()

	c.latencyMu.Lock()
	avgLatency := 0.0
	if len(c.latencySamples) > 0 {
		sum := 0.0
		for _, s := range c.latencySamples {
			sum += s
		}
		avgLatency = sum / float64(len(c.latencySamples))
	}
	c.latencyMu.Unlock()

	deadCount, _ := c.deadRepo.Count(ctx)

	return &models.MetricsSnapshot{
		QueueDepths:       depths,
		Throughput:        throughput,
		SuccessRates:      successRates,
		FailureRates:      failureRates,
		AvgLatency:        avgLatency,
		WorkerUtilization: utilization,
		DeadLetterCount:   deadCount,
		WorkersOnline:     workersOnline,
		WorkersOffline:    workersOffline,
		WorkersTotal:      workersOnline + workersOffline,
		Timestamp:         time.Now(),
	}
}

func (c *Collector) saveHistory(ctx context.Context, snap *models.MetricsSnapshot) {
	depthsJSON, _ := json.Marshal(snap.QueueDepths)
	successJSON, _ := json.Marshal(mapStrFloat(snap.SuccessRates))
	failJSON, _ := json.Marshal(mapStrFloat(snap.FailureRates))

	_ = c.cache.Client.ZAdd(ctx, cache.KeyThroughputWindow, redis.Z{
		Score:  float64(snap.Timestamp.Unix()),
		Member: fmt.Sprintf("%d:%f", snap.Timestamp.Unix(), snap.Throughput),
	}).Err()
	cutoff := float64(time.Now().Add(-24 * time.Hour).Unix())
	c.cache.Client.ZRemRangeByScore(ctx, cache.KeyThroughputWindow, "0", fmt.Sprintf("%f", cutoff))

	insertSQL := `INSERT INTO metrics_history 
		(timestamp, queue_depths, throughput, success_rates, failure_rates, 
		 avg_latency_ms, worker_utilization, dead_letter_count, 
		 workers_online, workers_offline, workers_total)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	if c.taskRepo != nil {
		_, _ = c.taskRepo.ExecHistory(ctx, insertSQL, snap.Timestamp, depthsJSON, snap.Throughput,
			successJSON, failJSON, snap.AvgLatency, snap.WorkerUtilization, snap.DeadLetterCount,
			snap.WorkersOnline, snap.WorkersOffline, snap.WorkersTotal)
	}
}

func (c *Collector) resetCounters() {
	atomic.StoreInt64(&c.completedCount, 0)
	atomic.StoreInt64(&c.failedCount, 0)
	c.windowStart = time.Now()
	c.priorityMu.Lock()
	c.prioritySuccess = make(map[models.Priority]int64)
	c.priorityFail = make(map[models.Priority]int64)
	c.priorityMu.Unlock()
	c.latencyMu.Lock()
	if len(c.latencySamples) > 50000 {
		c.latencySamples = c.latencySamples[len(c.latencySamples)-50000:]
	}
	c.latencyMu.Unlock()
}

func (c *Collector) GetThroughputHistory(ctx context.Context, hours int) (map[int64]float64, error) {
	cutoff := float64(time.Now().Add(-time.Duration(hours)*time.Hour).Unix())
	result, err := c.cache.Client.ZRangeByScore(ctx, cache.KeyThroughputWindow, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", cutoff),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	history := map[int64]float64{}
	for _, s := range result {
		var ts int64
		var tp float64
		_, err := fmt.Sscanf(s, "%d:%f", &ts, &tp)
		if err == nil {
			history[ts] = tp
		}
	}
	return history, nil
}

func mapStrFloat(m map[models.Priority]float64) map[string]float64 {
	result := map[string]float64{}
	for k, v := range m {
		result[k.String()] = v
	}
	return result
}
