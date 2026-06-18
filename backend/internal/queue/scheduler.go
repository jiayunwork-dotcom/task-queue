package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/semaphore"
	"task-queue/internal/cache"
	"task-queue/internal/config"
	"task-queue/internal/models"
	"task-queue/internal/repository"
)

type PriorityScheduler struct {
	cfg          *config.QueueConfig
	cache        *cache.Cache
	taskRepo     *repository.TaskRepository
	auditLog     func(ctx context.Context, entityType string, entityID uuid.UUID, action string)
	consecutiveHigh int64
	mu           sync.Mutex
	dispatchCh   chan DispatchRequest
	wg           sync.WaitGroup
	stopCh       chan struct{}
	preemptCh    chan uuid.UUID
}

type DispatchRequest struct {
	TaskID   uuid.UUID
	Priority models.Priority
}

func NewPriorityScheduler(
	cfg *config.QueueConfig,
	cache *cache.Cache,
	taskRepo *repository.TaskRepository,
) *PriorityScheduler {
	return &PriorityScheduler{
		cfg:      cfg,
		cache:    cache,
		taskRepo: taskRepo,
		dispatchCh: make(chan DispatchRequest, 10000),
		stopCh:   make(chan struct{}),
		preemptCh: make(chan uuid.UUID, 1000),
	}
}

func (s *PriorityScheduler) SetAuditLogger(fn func(ctx context.Context, entityType string, entityID uuid.UUID, action string)) {
	s.auditLog = fn
}

func (s *PriorityScheduler) DispatchChan() <-chan DispatchRequest {
	return s.dispatchCh
}

func (s *PriorityScheduler) PreemptChan() <-chan uuid.UUID {
	return s.preemptCh
}

func (s *PriorityScheduler) EnqueueReady(ctx context.Context, taskID uuid.UUID, priority models.Priority) error {
	key := cache.ReadyQueueKey(int(priority))
	score := float64(time.Now().UnixNano())
	if err := s.cache.Client.ZAdd(ctx, key, redis.Z{Score: score, Member: taskID.String()}).Err(); err != nil {
		return fmt.Errorf("zadd ready queue: %w", err)
	}
	if priority == models.PriorityCritical || priority == models.PriorityHigh {
		select {
		case s.preemptCh <- taskID:
		default:
		}
	}
	return nil
}

func (s *PriorityScheduler) Start(ctx context.Context, dispatchInterval time.Duration, batchSize int) {
	s.wg.Add(1)
	go s.dispatchLoop(ctx, dispatchInterval, batchSize)
}

func (s *PriorityScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *PriorityScheduler) dispatchLoop(ctx context.Context, interval time.Duration, batchSize int) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sem := semaphore.NewWeighted(100)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := sem.Acquire(ctx, 1); err != nil {
				continue
			}
			go func() {
				defer sem.Release(1)
				s.dispatchBatch(ctx, batchSize)
			}()
		}
	}
}

func (s *PriorityScheduler) dispatchBatch(ctx context.Context, batchSize int) {
	remaining := batchSize
	for remaining > 0 {
		taskID, priority, found := s.pickNextTask(ctx)
		if !found {
			return
		}

		key := cache.ReadyQueueKey(int(priority))
		removed, err := s.cache.Client.ZRem(ctx, key, taskID.String()).Result()
		if err != nil || removed == 0 {
			continue
		}

		task, err := s.taskRepo.GetByID(ctx, taskID)
		if err != nil || task == nil || task.Status != models.TaskStatusReady {
			continue
		}

		s.dispatchCh <- DispatchRequest{TaskID: taskID, Priority: priority}
		remaining--
	}
}

func (s *PriorityScheduler) pickNextTask(ctx context.Context) (uuid.UUID, models.Priority, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	useFairness := s.cfg.FairnessN > 0 && s.consecutiveHigh >= int64(s.cfg.FairnessN)

	priorities := []models.Priority{
		models.PriorityCritical,
		models.PriorityHigh,
		models.PriorityNormal,
		models.PriorityLow,
		models.PriorityBulk,
	}

	if useFairness {
		s.consecutiveHigh = 0
		lowPriorities := []models.Priority{
			models.PriorityBulk,
			models.PriorityLow,
			models.PriorityNormal,
		}
		for _, p := range lowPriorities {
			if id, ok := s.popFromQueue(ctx, p); ok {
				return id, p, true
			}
		}
	}

	for _, p := range priorities {
		if id, ok := s.popFromQueue(ctx, p); ok {
			if p >= models.PriorityHigh {
				s.consecutiveHigh++
			} else {
				s.consecutiveHigh = 0
			}
			return id, p, true
		}
	}

	return uuid.Nil, models.PriorityNormal, false
}

func (s *PriorityScheduler) popFromQueue(ctx context.Context, p models.Priority) (uuid.UUID, bool) {
	key := cache.ReadyQueueKey(int(p))
	result, err := s.cache.Client.ZRange(ctx, key, 0, 0).Result()
	if err != nil || len(result) == 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(result[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func (s *PriorityScheduler) GetQueueDepths(ctx context.Context) (map[models.Priority]int64, error) {
	depths := map[models.Priority]int64{
		models.PriorityCritical: 0,
		models.PriorityHigh:     0,
		models.PriorityNormal:   0,
		models.PriorityLow:      0,
		models.PriorityBulk:     0,
	}

	priorities := []models.Priority{
		models.PriorityCritical,
		models.PriorityHigh,
		models.PriorityNormal,
		models.PriorityLow,
		models.PriorityBulk,
	}

	for _, p := range priorities {
		key := cache.ReadyQueueKey(int(p))
		count, err := s.cache.Client.ZCard(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		depths[p] = count
	}
	return depths, nil
}

func (s *PriorityScheduler) RequeueFront(ctx context.Context, taskID uuid.UUID, priority models.Priority) error {
	key := cache.ReadyQueueKey(int(priority))
	score := float64(time.Now().Add(-1 * time.Hour).UnixNano())
	return s.cache.Client.ZAdd(ctx, key, redis.Z{Score: score, Member: taskID.String()}).Err()
}

type DelayScheduler struct {
	cfg      *config.QueueConfig
	cache    *cache.Cache
	taskRepo *repository.TaskRepository
	scheduler *PriorityScheduler
	wg       sync.WaitGroup
	stopCh   chan struct{}
	auditLog func(ctx context.Context, entityType string, entityID uuid.UUID, action string)
}

func NewDelayScheduler(
	cfg *config.QueueConfig,
	cache *cache.Cache,
	taskRepo *repository.TaskRepository,
	scheduler *PriorityScheduler,
) *DelayScheduler {
	return &DelayScheduler{
		cfg:       cfg,
		cache:     cache,
		taskRepo:  taskRepo,
		scheduler: scheduler,
		stopCh:    make(chan struct{}),
	}
}

func (s *DelayScheduler) SetAuditLogger(fn func(ctx context.Context, entityType string, entityID uuid.UUID, action string)) {
	s.auditLog = fn
}

func (s *DelayScheduler) EnqueueDelay(ctx context.Context, taskID uuid.UUID, scheduledAt time.Time) error {
	score := float64(scheduledAt.Unix())
	return s.cache.Client.ZAdd(ctx, cache.KeyPrefixDelayedQueue, redis.Z{
		Score:  score,
		Member: taskID.String(),
	}).Err()
}

func (s *DelayScheduler) Start(ctx context.Context) {
	interval := time.Duration(s.cfg.DelayScanInterval) * time.Second
	s.wg.Add(1)
	go s.scanLoop(ctx, interval)
}

func (s *DelayScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *DelayScheduler) scanLoop(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.scanAndDispatch(ctx)
		}
	}
}

func (s *DelayScheduler) scanAndDispatch(ctx context.Context) {
	now := float64(time.Now().Unix())
	max := fmt.Sprintf("%f", now)

	result, err := s.cache.Client.ZRangeByScore(ctx, cache.KeyPrefixDelayedQueue, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    max,
		Offset: 0,
		Count:  500,
	}).Result()
	if err != nil || len(result) == 0 {
		return
	}

	for _, idStr := range result {
		taskID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}

		removed, _ := s.cache.Client.ZRem(ctx, cache.KeyPrefixDelayedQueue, idStr).Result()
		if removed == 0 {
			continue
		}

		task, err := s.taskRepo.GetByID(ctx, taskID)
		if err != nil || task == nil {
			continue
		}

		if task.Status != models.TaskStatusDelayed {
			continue
		}

		if err := s.taskRepo.UpdateStatus(ctx, taskID, models.TaskStatusReady); err != nil {
			continue
		}
		if s.auditLog != nil {
			s.auditLog(ctx, "task", taskID, "delayed_to_ready")
		}
		if err := s.scheduler.EnqueueReady(ctx, taskID, task.Priority); err != nil {
			continue
		}
	}
}

func (s *DelayScheduler) GetDelayedCount(ctx context.Context) (int64, error) {
	return s.cache.Client.ZCard(ctx, cache.KeyPrefixDelayedQueue).Result()
}
