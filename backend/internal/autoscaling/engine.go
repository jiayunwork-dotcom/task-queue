package autoscaling

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"task-queue/internal/models"
	"task-queue/internal/repository"
)

type ScalingEventBroadcaster interface {
	BroadcastScalingEvent(event *models.ScalingEvent)
}

type Engine struct {
	scalingRepo   *repository.ScalingRepository
	workerRepo    *repository.WorkerRepository
	handlerRepo   *repository.HandlerRepository
	taskRepo      *repository.TaskRepository
	eventNotifier ScalingEventBroadcaster

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func NewEngine(
	scalingRepo *repository.ScalingRepository,
	workerRepo *repository.WorkerRepository,
	handlerRepo *repository.HandlerRepository,
	taskRepo *repository.TaskRepository,
) *Engine {
	return &Engine{
		scalingRepo: scalingRepo,
		workerRepo:  workerRepo,
		handlerRepo: handlerRepo,
		taskRepo:    taskRepo,
		stopCh:      make(chan struct{}),
	}
}

func (e *Engine) SetEventNotifier(notifier ScalingEventBroadcaster) {
	e.eventNotifier = notifier
}

func (e *Engine) Start(ctx context.Context) {
	e.wg.Add(1)
	go e.evaluationLoop(ctx)
}

func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

func (e *Engine) evaluationLoop(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.evaluateAll(ctx)
		}
	}
}

func (e *Engine) evaluateAll(ctx context.Context) {
	policies, err := e.scalingRepo.ListEnabledPolicies(ctx)
	if err != nil {
		return
	}

	for _, policy := range policies {
		e.evaluatePolicy(ctx, &policy)
	}
}

type TaskTypeMetrics struct {
	TaskType     string
	WorkerCount  int
	TotalSlots   int
	UsedSlots    int
	IdleWorkers  int
	QueueWaiting int
	UtilPct      float64
}

func (e *Engine) getTaskTypeMetrics(ctx context.Context, taskType string) (*TaskTypeMetrics, error) {
	handlers, err := e.handlerRepo.FindByTaskType(ctx, taskType)
	if err != nil {
		return nil, err
	}

	workerIDs := make(map[uuid.UUID]bool)
	for _, h := range handlers {
		workerIDs[h.WorkerID] = true
	}

	workers, err := e.workerRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	var activeWorkers []models.Worker
	for _, w := range workers {
		if workerIDs[w.ID] && (w.Status == models.WorkerStatusOnline || w.Status == models.WorkerStatusDraining) {
			activeWorkers = append(activeWorkers, w)
		}
	}

	totalSlots := 0
	usedSlots := 0
	idleWorkers := 0
	for _, w := range activeWorkers {
		totalSlots += w.TotalSlots
		usedSlots += w.UsedSlots
		if w.UsedSlots == 0 {
			idleWorkers++
		}
	}

	utilPct := 0.0
	if totalSlots > 0 {
		utilPct = float64(usedSlots) / float64(totalSlots) * 100
	}

	queueCount, err := e.taskRepo.CountReadyByType(ctx, taskType)
	if err != nil {
		queueCount = 0
	}

	return &TaskTypeMetrics{
		TaskType:     taskType,
		WorkerCount:  len(activeWorkers),
		TotalSlots:   totalSlots,
		UsedSlots:    usedSlots,
		IdleWorkers:  idleWorkers,
		QueueWaiting: queueCount,
		UtilPct:      utilPct,
	}, nil
}

func (e *Engine) evaluatePolicy(ctx context.Context, policy *models.ScalingPolicy) {
	now := time.Now()

	if !policy.IsWithinScheduleWindow(now) {
		metrics, err := e.getTaskTypeMetrics(ctx, policy.TaskType)
		if err != nil {
			return
		}
		history := &models.ScalingHistory{
			PolicyID:        policy.ID,
			TaskType:        policy.TaskType,
			OperationType:   models.ScalingOpNoOp,
			Reason:          "Outside schedule window",
			SuggestedCount:  0,
			SnapshotWorkers: metrics.WorkerCount,
			SnapshotUtilPct: metrics.UtilPct,
			SnapshotQueue:   metrics.QueueWaiting,
			CreatedAt:       now,
		}
		_ = e.scalingRepo.InsertHistory(ctx, history)
		return
	}

	metrics, err := e.getTaskTypeMetrics(ctx, policy.TaskType)
	if err != nil {
		return
	}

	secondsSinceOp := -1
	if policy.LastOperationAt != nil {
		secondsSinceOp = int(now.Sub(*policy.LastOperationAt).Seconds())
	}

	inCooldown := policy.LastOperationAt != nil && secondsSinceOp >= 0 && secondsSinceOp < policy.CooldownSeconds

	var opType models.ScalingOperationType
	var reason string
	var suggestedCount int

	if metrics.QueueWaiting > policy.ScaleOutThreshold &&
		metrics.WorkerCount < policy.MaxWorkers &&
		!inCooldown {

		avgCapacity := 0
		if metrics.WorkerCount > 0 {
			avgCapacity = metrics.TotalSlots / metrics.WorkerCount
		}
		if avgCapacity <= 0 {
			avgCapacity = 10
		}

		needed := int(math.Ceil(float64(metrics.QueueWaiting) / float64(avgCapacity)))
		maxAdd := policy.MaxWorkers - metrics.WorkerCount
		if needed > maxAdd {
			needed = maxAdd
		}
		if needed < 1 {
			needed = 1
		}

		opType = models.ScalingOpScaleOut
		suggestedCount = needed
		reason = "Queue waiting tasks exceed scale-out threshold"

	} else if (100.0-metrics.UtilPct) > policy.ScaleInThresholdPct &&
		metrics.WorkerCount > policy.MinWorkers &&
		!inCooldown {

		workers, err := e.getWorkersForTaskType(ctx, policy.TaskType)
		if err != nil {
			return
		}

		removableCount := 0
		for _, w := range workers {
			runtimeSecs := int(now.Sub(w.RegisteredAt).Seconds())
			if runtimeSecs >= policy.ScaleInProtectionSecs && w.UsedSlots == 0 {
				removableCount++
			}
		}

		canRemove := metrics.WorkerCount - policy.MinWorkers
		if canRemove < 0 {
			canRemove = 0
		}
		if removableCount > canRemove {
			removableCount = canRemove
		}

		if removableCount > 0 {
			opType = models.ScalingOpScaleIn
			suggestedCount = removableCount
			reason = "Worker idle rate exceeds scale-in threshold"
		} else {
			opType = models.ScalingOpNoOp
			suggestedCount = 0
			reason = "Idle rate high but no workers eligible for scale-in"
		}
	} else {
		opType = models.ScalingOpNoOp
		suggestedCount = 0
		if inCooldown {
			reason = "In cooldown period"
		} else if metrics.WorkerCount >= policy.MaxWorkers && metrics.QueueWaiting > policy.ScaleOutThreshold {
			reason = "At max workers, cannot scale out further"
		} else if metrics.WorkerCount <= policy.MinWorkers && (100.0-metrics.UtilPct) > policy.ScaleInThresholdPct {
			reason = "At min workers, cannot scale in further"
		} else {
			reason = "Metrics within target range"
		}
	}

	if opType != models.ScalingOpNoOp {
		_ = e.scalingRepo.UpdateLastOperation(ctx, policy.ID, now)
	}

	history := &models.ScalingHistory{
		PolicyID:        policy.ID,
		TaskType:        policy.TaskType,
		OperationType:   opType,
		Reason:          reason,
		SuggestedCount:  suggestedCount,
		SnapshotWorkers: metrics.WorkerCount,
		SnapshotUtilPct: metrics.UtilPct,
		SnapshotQueue:   metrics.QueueWaiting,
		CreatedAt:       now,
	}
	_ = e.scalingRepo.InsertHistory(ctx, history)

	if opType != models.ScalingOpNoOp && e.eventNotifier != nil {
		event := &models.ScalingEvent{
			EventTime:      now,
			PolicyID:       policy.ID,
			TaskType:       policy.TaskType,
			OperationType:  opType,
			SuggestedCount: suggestedCount,
		}
		e.eventNotifier.BroadcastScalingEvent(event)
	}
}

func (e *Engine) getWorkersForTaskType(ctx context.Context, taskType string) ([]models.Worker, error) {
	handlers, err := e.handlerRepo.FindByTaskType(ctx, taskType)
	if err != nil {
		return nil, err
	}

	workerIDs := make(map[uuid.UUID]bool)
	for _, h := range handlers {
		workerIDs[h.WorkerID] = true
	}

	allWorkers, err := e.workerRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []models.Worker
	for _, w := range allWorkers {
		if workerIDs[w.ID] && w.Status == models.WorkerStatusOnline {
			result = append(result, w)
		}
	}
	return result, nil
}

func (e *Engine) GetPolicyMetrics(ctx context.Context) ([]models.ScalingPolicyMetrics, error) {
	policies, err := e.scalingRepo.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]models.ScalingPolicyMetrics, 0, len(policies))
	now := time.Now()

	for _, policy := range policies {
		metrics, err := e.getTaskTypeMetrics(ctx, policy.TaskType)
		if err != nil {
			continue
		}

		secondsSinceOp := -1
		if policy.LastOperationAt != nil {
			secondsSinceOp = int(now.Sub(*policy.LastOperationAt).Seconds())
		}

		result = append(result, models.ScalingPolicyMetrics{
			PolicyID:       policy.ID,
			TaskType:       policy.TaskType,
			CurrentWorkers: metrics.WorkerCount,
			UtilizationPct: metrics.UtilPct,
			QueueWaiting:   metrics.QueueWaiting,
			LastOperationAt: policy.LastOperationAt,
			SecondsSinceOp: secondsSinceOp,
		})
	}

	return result, nil
}
