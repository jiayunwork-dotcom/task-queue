package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
	"task-queue/internal/config"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/repository"
)

type Manager struct {
	cfg         *config.WorkerConfig
	queueCfg    *config.QueueConfig
	workerRepo  *repository.WorkerRepository
	handlerRepo *repository.HandlerRepository
	taskRepo    *repository.TaskRepository
	auditLog    func(ctx context.Context, entityType string, entityID uuid.UUID, action string)

	runningTasks map[uuid.UUID]*RunningTask
	slots        *semaphore.Weighted
	mu           sync.RWMutex
	wg           sync.WaitGroup
	stopCh       chan struct{}
	stopping     bool

	selfWorker   *models.Worker
	handlers     map[string]string
	handlerMu    sync.RWMutex

	retryCallback func(ctx context.Context, task *models.Task, err error, retryCount int)
	deadCallback  func(ctx context.Context, task *models.Task, err string)
	leaseTTL      time.Duration
}

type RunningTask struct {
	TaskID      uuid.UUID
	Priority    models.Priority
	ExecutionID uuid.UUID
	WorkerID    uuid.UUID
	HandlerID   string
	CancelFn    context.CancelFunc
	Context     context.Context
	StartTime   time.Time
	Timeout     time.Duration
}

func NewManager(
	cfg *config.WorkerConfig,
	queueCfg *config.QueueConfig,
	workerRepo *repository.WorkerRepository,
	handlerRepo *repository.HandlerRepository,
	taskRepo *repository.TaskRepository,
	selfWorker *models.Worker,
) *Manager {
	return &Manager{
		cfg:          cfg,
		queueCfg:     queueCfg,
		workerRepo:   workerRepo,
		handlerRepo:  handlerRepo,
		taskRepo:     taskRepo,
		runningTasks: make(map[uuid.UUID]*RunningTask),
		slots:        semaphore.NewWeighted(int64(cfg.DefaultSlots)),
		stopCh:       make(chan struct{}),
		selfWorker:   selfWorker,
		handlers:     make(map[string]string),
		leaseTTL:     time.Duration(queueCfg.LeaseTTL) * time.Second,
	}
}

func (m *Manager) SetAuditLogger(fn func(ctx context.Context, entityType string, entityID uuid.UUID, action string)) {
	m.auditLog = fn
}

func (m *Manager) SetCallbacks(
	retryFn func(ctx context.Context, task *models.Task, err error, retryCount int),
	deadFn func(ctx context.Context, task *models.Task, err string),
) {
	m.retryCallback = retryFn
	m.deadCallback = deadFn
}

func (m *Manager) RegisterHandler(taskType string, endpoint string) {
	m.handlerMu.Lock()
	defer m.handlerMu.Unlock()
	m.handlers[taskType] = endpoint
}

func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.runningTasks)
}

func (m *Manager) GetAvailableSlots() int {
	return int(m.slots.TryAcquire(0))
}

func (m *Manager) Start(ctx context.Context, dispatchCh <-chan queue.DispatchRequest, preemptCh <-chan uuid.UUID) {
	m.wg.Add(3)
	go m.heartbeatLoop(ctx)
	go m.leaseRenewalLoop(ctx)
	go m.dispatchConsumer(ctx, dispatchCh)

	if preemptCh != nil {
		m.wg.Add(1)
		go m.preemptionHandler(ctx, preemptCh)
	}
}

func (m *Manager) Stop(ctx context.Context) {
	m.stopping = true
	close(m.stopCh)

	m.mu.RLock()
	count := len(m.runningTasks)
	m.mu.RUnlock()
	if count > 0 {
		_ = m.workerRepo.UpdateStatus(ctx, m.selfWorker.ID, models.WorkerStatusDraining)
	}

	timeout := time.After(time.Duration(m.cfg.GracefulShutdownTimeout) * time.Second)
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-timeout:
		m.mu.Lock()
		for id, rt := range m.runningTasks {
			rt.CancelFn()
			delete(m.runningTasks, id)
		}
		m.mu.Unlock()
	}

	_ = m.workerRepo.UpdateStatus(ctx, m.selfWorker.ID, models.WorkerStatusOffline)
}

func (m *Manager) heartbeatLoop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cfg.HeartbeatIntervalDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			used := len(m.runningTasks)
			m.mu.RUnlock()
			_ = m.workerRepo.Heartbeat(ctx, m.selfWorker.ID, used)

			m.handlerMu.RLock()
			handlers := make([]string, 0, len(m.handlers))
			for t := range m.handlers {
				handlers = append(handlers, t)
			}
			m.handlerMu.RUnlock()
			m.handlerMu.RLock()
			for taskType, endpoint := range m.handlers {
				h := &models.HandlerRegistration{
					TaskType:  taskType,
					HandlerID: fmt.Sprintf("%s-%s", m.selfWorker.ID.String(), taskType),
					WorkerID:  m.selfWorker.ID,
					Endpoint:  endpoint,
				}
				_ = m.handlerRepo.Register(ctx, h)
			}
			m.handlerMu.RUnlock()
		}
	}
}

func (m *Manager) leaseRenewalLoop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.leaseTTL / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			tasks := make([]*RunningTask, 0, len(m.runningTasks))
			for _, rt := range m.runningTasks {
				tasks = append(tasks, rt)
			}
			m.mu.RUnlock()

			for _, rt := range tasks {
				_, _ = m.taskRepo.RenewLease(ctx, rt.TaskID, rt.WorkerID, m.leaseTTL)
			}
		}
	}
}

func (m *Manager) dispatchConsumer(ctx context.Context, dispatchCh <-chan queue.DispatchRequest) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case req, ok := <-dispatchCh:
			if !ok {
				return
			}
			if m.stopping {
				continue
			}
			go m.processTask(ctx, req)
		}
	}
}

func (m *Manager) preemptionHandler(ctx context.Context, preemptCh <-chan uuid.UUID) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case _, ok := <-preemptCh:
			if !ok {
				return
			}
			m.mu.RLock()
			var preemptTarget *RunningTask
			for _, rt := range m.runningTasks {
				if rt.Priority <= models.PriorityNormal {
					preemptTarget = rt
					break
				}
			}
			m.mu.RUnlock()
			if preemptTarget != nil {
				preemptTarget.CancelFn()
			}
		}
	}
}

func (m *Manager) processTask(parentCtx context.Context, req queue.DispatchRequest) {
	if !m.slots.TryAcquire(1) {
		return
	}
	defer m.slots.Release(1)

	task, err := m.taskRepo.GetByID(parentCtx, req.TaskID)
	if err != nil || task == nil {
		return
	}

	ok, err := m.taskRepo.LeaseTask(parentCtx, task.ID, m.selfWorker.ID, m.leaseTTL)
	if err != nil || !ok {
		return
	}

	m.handlerMu.RLock()
	endpoint, hasHandler := m.handlers[task.Type]
	m.handlerMu.RUnlock()
	if !hasHandler {
		altHandlers, _ := m.handlerRepo.FindByTaskType(parentCtx, task.Type)
		if len(altHandlers) > 0 {
			endpoint = altHandlers[0].Endpoint
			hasHandler = true
		}
	}
	if !hasHandler {
		_ = m.taskRepo.UpdateStatus(parentCtx, task.ID, models.TaskStatusFailed,
			"last_error", fmt.Sprintf("no handler registered for task type: %s", task.Type))
		return
	}

	taskCtx, cancel := context.WithTimeout(parentCtx, time.Duration(task.TimeoutSeconds)*time.Second)
	defer cancel()

	exec := &models.TaskExecution{
		TaskID:    task.ID,
		Attempt:   task.RetryCount + 1,
		WorkerID:  m.selfWorker.ID,
		HandlerID: fmt.Sprintf("%s-%s", m.selfWorker.ID.String(), task.Type),
	}
	_ = m.taskRepo.CreateExecution(parentCtx, exec)

	rt := &RunningTask{
		TaskID:      task.ID,
		Priority:    task.Priority,
		ExecutionID: exec.ID,
		WorkerID:    m.selfWorker.ID,
		HandlerID:   exec.HandlerID,
		CancelFn:    cancel,
		Context:     taskCtx,
		StartTime:   time.Now(),
		Timeout:     time.Duration(task.TimeoutSeconds) * time.Second,
	}
	m.mu.Lock()
	m.runningTasks[task.ID] = rt
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.runningTasks, task.ID)
		m.mu.Unlock()
	}()

	_ = m.taskRepo.UpdateStatus(parentCtx, task.ID, models.TaskStatusRunning,
		"worker_id", m.selfWorker.ID,
		"handler_id", exec.HandlerID)
	if m.auditLog != nil {
		m.auditLog(parentCtx, "task", task.ID, "start_execution")
	}

	result, execErr := m.callHandler(taskCtx, endpoint, task)

	preempted := taskCtx.Err() == context.Canceled && execErr == nil
	if preempted {
		_ = m.taskRepo.CompleteExecution(parentCtx, exec.ID, models.TaskStatusReady, "preempted")
		_ = m.taskRepo.UpdateStatus(parentCtx, task.ID, models.TaskStatusReady,
			"worker_id", nil, "handler_id", nil, "lease_expires_at", nil)
		if m.auditLog != nil {
			m.auditLog(parentCtx, "task", task.ID, "preempted")
		}
		return
	}

	if execErr != nil || !result.Success {
		errMsg := execErr.Error()
		if execErr == nil && result.Error != "" {
			errMsg = result.Error
		}
		_ = m.taskRepo.CompleteExecution(parentCtx, exec.ID, models.TaskStatusFailed, errMsg)
		_ = m.workerRepo.IncrementStats(parentCtx, m.selfWorker.ID, 0, 1)
		_ = m.taskRepo.IncrementRetry(parentCtx, task.ID, errMsg)

		if task.RetryCount+1 >= task.MaxRetries {
			_ = m.taskRepo.UpdateStatus(parentCtx, task.ID, models.TaskStatusDeadLetter,
				"last_error", errMsg)
			if m.deadCallback != nil {
				m.deadCallback(parentCtx, task, errMsg)
			}
			if m.auditLog != nil {
				m.auditLog(parentCtx, "task", task.ID, "dead_letter")
			}
		} else {
			if m.retryCallback != nil {
				m.retryCallback(parentCtx, task, execErr, task.RetryCount+1)
			}
			if m.auditLog != nil {
				m.auditLog(parentCtx, "task", task.ID, fmt.Sprintf("retry_%d", task.RetryCount+1))
			}
		}
	} else {
		_ = m.taskRepo.CompleteExecution(parentCtx, exec.ID, models.TaskStatusSuccess, "")
		_ = m.workerRepo.IncrementStats(parentCtx, m.selfWorker.ID, 1, 0)
		_ = m.taskRepo.UpdateStatus(parentCtx, task.ID, models.TaskStatusSuccess)
		if m.auditLog != nil {
			m.auditLog(parentCtx, "task", task.ID, "success")
		}
		if task.CallbackURL != "" {
			go m.sendCallback(task.CallbackURL, task, result)
		}
	}
}

type HandlerRequest struct {
	TaskID  uuid.UUID       `json:"task_id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type HandlerResponse struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (m *Manager) callHandler(ctx context.Context, endpoint string, task *models.Task) (*HandlerResponse, error) {
	body := HandlerRequest{
		TaskID:  task.ID,
		Type:    task.Type,
		Payload: task.Payload,
	}
	payload, _ := json.Marshal(body)

	httpClient := &http.Client{Timeout: time.Duration(task.TimeoutSeconds) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call handler: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result HandlerResponse
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &result)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if result.Error != "" {
			return &result, fmt.Errorf(result.Error)
		}
		return &result, nil
	}
	return &result, fmt.Errorf("handler returned status %d: %s", resp.StatusCode, string(respBody))
}

func (m *Manager) sendCallback(url string, task *models.Task, result *HandlerResponse) {
	cbBody := map[string]interface{}{
		"task_id": task.ID.String(),
		"status":  task.Status,
		"type":    task.Type,
		"result":  result,
	}
	payload, _ := json.Marshal(cbBody)
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func (m *Manager) PreemptLowPriorityTasks(ctx context.Context) int {
	m.mu.RLock()
	var toCancel []*RunningTask
	for _, rt := range m.runningTasks {
		if rt.Priority <= models.PriorityNormal {
			toCancel = append(toCancel, rt)
		}
	}
	m.mu.RUnlock()

	for _, rt := range toCancel {
		rt.CancelFn()
	}
	return len(toCancel)
}

type DeadLetterReaper struct {
	cfg         *config.WorkerConfig
	workerRepo  *repository.WorkerRepository
	taskRepo    *repository.TaskRepository
	deadRepo    *repository.DeadLetterRepository
	auditLog    func(ctx context.Context, entityType string, entityID uuid.UUID, action string)
	wg          sync.WaitGroup
	stopCh      chan struct{}
}

func NewDeadLetterReaper(
	cfg *config.WorkerConfig,
	workerRepo *repository.WorkerRepository,
	taskRepo *repository.TaskRepository,
	deadRepo *repository.DeadLetterRepository,
) *DeadLetterReaper {
	return &DeadLetterReaper{
		cfg:        cfg,
		workerRepo: workerRepo,
		taskRepo:   taskRepo,
		deadRepo:   deadRepo,
		stopCh:     make(chan struct{}),
	}
}

func (r *DeadLetterReaper) SetAuditLogger(fn func(ctx context.Context, entityType string, entityID uuid.UUID, action string)) {
	r.auditLog = fn
}

func (r *DeadLetterReaper) Start(ctx context.Context) {
	r.wg.Add(1)
	go r.reapLoop(ctx)
}

func (r *DeadLetterReaper) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

func (r *DeadLetterReaper) reapLoop(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.cfg.HeartbeatTimeoutDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reapTimedOutWorkers(ctx)
			r.recoverExpiredLeases(ctx)
		}
	}
}

func (r *DeadLetterReaper) reapTimedOutWorkers(ctx context.Context) {
	timeout := r.cfg.HeartbeatTimeoutDuration()
	workers, err := r.workerRepo.FindTimedOut(ctx, timeout)
	if err != nil {
		return
	}
	for _, w := range workers {
		_ = r.workerRepo.UpdateStatus(ctx, w.ID, models.WorkerStatusOffline)
		if r.auditLog != nil {
			r.auditLog(ctx, "worker", w.ID, "timed_out")
		}
		taskIDs, _ := r.workerRepo.GetTaskIDsByWorker(ctx, w.ID)
		for _, tid := range taskIDs {
			_ = r.taskRepo.UpdateStatus(ctx, tid, models.TaskStatusReady,
				"worker_id", nil, "handler_id", nil, "lease_expires_at", nil)
			task, _ := r.taskRepo.GetByID(ctx, tid)
			if task != nil {
				if r.auditLog != nil {
					r.auditLog(ctx, "task", tid, "recovered_from_dead_worker")
				}
			}
		}
	}
}

func (r *DeadLetterReaper) recoverExpiredLeases(ctx context.Context) {
	ids, err := r.taskRepo.FindExpiredLeases(ctx, 100)
	if err != nil {
		return
	}
	for _, id := range ids {
		_ = r.taskRepo.UpdateStatus(ctx, id, models.TaskStatusReady,
			"worker_id", nil, "handler_id", nil, "lease_expires_at", nil)
		if r.auditLog != nil {
			r.auditLog(ctx, "task", id, "lease_expired_requeued")
		}
	}
}
