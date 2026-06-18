package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/repository"
)

type Engine struct {
	taskRepo       *repository.TaskRepository
	deadRepo       *repository.DeadLetterRepository
	delayScheduler *queue.DelayScheduler
	scheduler      *queue.PriorityScheduler
	parser         cron.Parser
}

func NewEngine(
	taskRepo *repository.TaskRepository,
	deadRepo *repository.DeadLetterRepository,
	delayScheduler *queue.DelayScheduler,
	scheduler *queue.PriorityScheduler,
) *Engine {
	return &Engine{
		taskRepo:       taskRepo,
		deadRepo:       deadRepo,
		delayScheduler: delayScheduler,
		scheduler:      scheduler,
		parser:         cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

func (e *Engine) CalculateNextRetry(ctx context.Context, task *models.Task, err error, retryCount int) time.Time {
	now := time.Now()

	switch task.RetryMode {
	case models.RetryModeExponential:
		base := 10
		if task.RetryInterval > 0 {
			base = task.RetryInterval
		}
		interval := base * (1 << (retryCount - 1))
		if interval > 86400 {
			interval = 86400
		}
		return now.Add(time.Duration(interval) * time.Second)

	case models.RetryModeFixed:
		interval := 10
		if task.RetryInterval > 0 {
			interval = task.RetryInterval
		}
		return now.Add(time.Duration(interval) * time.Second)

	case models.RetryModeCron:
		if task.RetryCronExpr == "" {
			return now.Add(10 * time.Second)
		}
		sched, err := e.parser.Parse(task.RetryCronExpr)
		if err != nil {
			return now.Add(10 * time.Second)
		}
		next := sched.Next(now)
		if next.Sub(now) > 24*time.Hour {
			return now.Add(1 * time.Hour)
		}
		return next

	default:
		return now.Add(10 * time.Second)
	}
}

func (e *Engine) HandleRetry(ctx context.Context, task *models.Task, err error, retryCount int) {
	nextRun := e.CalculateNextRetry(ctx, task, err, retryCount)
	delay := time.Until(nextRun)

	if delay <= 5*time.Second {
		if _, err := e.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusReady); err != nil {
			return
		}
		_ = e.scheduler.EnqueueReady(ctx, task.ID, task.Priority)
		return
	}

	delaySeconds := int(delay.Seconds())
	scheduledAt := nextRun
	if _, err := e.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusDelayed,
		"scheduled_at", scheduledAt,
		"delay_seconds", delaySeconds); err != nil {
		return
	}
	_ = e.delayScheduler.EnqueueDelay(ctx, task.ID, scheduledAt)
}

func (e *Engine) HandleDeadLetter(ctx context.Context, task *models.Task, err string) {
	execs, _ := e.taskRepo.GetExecutions(ctx, task.ID)
	errHistory := make([]string, 0, len(execs))
	for _, exec := range execs {
		if exec.Error != "" {
			errHistory = append(errHistory,
				fmt.Sprintf("[Attempt %d %s] %s", exec.Attempt,
					exec.StartedAt.Format(time.RFC3339), exec.Error))
		}
	}
	if len(errHistory) == 0 {
		errHistory = append(errHistory, err)
	}
	_ = e.deadRepo.Add(ctx, task.ID, err, errHistory)
}

func (e *Engine) RetryDeadLetter(ctx context.Context, taskIDs []uuid.UUID) (int, error) {
	success := 0
	for _, id := range taskIDs {
		task, err := e.taskRepo.GetByID(ctx, id)
		if err != nil || task == nil {
			continue
		}
		if task.Status != models.TaskStatusDeadLetter {
			continue
		}
		if _, err := e.taskRepo.UpdateStatus(ctx, id, models.TaskStatusReady,
			"retry_count", 0, "last_error", nil, "completed_at", nil); err != nil {
			continue
		}
		if err := e.deadRepo.Remove(ctx, id); err != nil {
			continue
		}
		if err := e.scheduler.EnqueueReady(ctx, id, task.Priority); err != nil {
			continue
		}
		success++
	}
	return success, nil
}

func (e *Engine) DiscardDeadLetter(ctx context.Context, taskIDs []uuid.UUID) (int, error) {
	success := 0
	for _, id := range taskIDs {
		task, err := e.taskRepo.GetByID(ctx, id)
		if err != nil || task == nil {
			continue
		}
		if task.Status != models.TaskStatusDeadLetter {
			continue
		}
		if err := e.deadRepo.Remove(ctx, id); err != nil {
			continue
		}
		if _, err := e.taskRepo.UpdateStatus(ctx, id, models.TaskStatusCancelled); err != nil {
			continue
		}
		success++
	}
	return success, nil
}

type DAGEngine struct {
	taskRepo    *repository.TaskRepository
	dagRepo     *repository.DAGRepository
	scheduler   *queue.PriorityScheduler
	delaySched  *queue.DelayScheduler
	auditLog    func(ctx context.Context, entityType string, entityID uuid.UUID, action string)
}

func NewDAGEngine(
	taskRepo *repository.TaskRepository,
	dagRepo *repository.DAGRepository,
	scheduler *queue.PriorityScheduler,
	delaySched *queue.DelayScheduler,
) *DAGEngine {
	return &DAGEngine{
		taskRepo:   taskRepo,
		dagRepo:    dagRepo,
		scheduler:  scheduler,
		delaySched: delaySched,
	}
}

func (e *DAGEngine) SetAuditLogger(fn func(ctx context.Context, entityType string, entityID uuid.UUID, action string)) {
	e.auditLog = fn
}

func (e *DAGEngine) StartDAG(ctx context.Context, templateID uuid.UUID, payload []byte, strategy models.DAGNodeStrategy, maxRetries int) (uuid.UUID, error) {
	tpl, err := e.dagRepo.GetTemplate(ctx, templateID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get template: %w", err)
	}

	var nodes []models.DAGNode
	var edges []models.DAGEdge
	if err := parseJSON(tpl.Nodes, &nodes); err != nil {
		return uuid.Nil, fmt.Errorf("parse nodes: %w", err)
	}
	if err := parseJSON(tpl.Edges, &edges); err != nil {
		return uuid.Nil, fmt.Errorf("parse edges: %w", err)
	}

	nodeState := make(map[string]map[string]interface{})
	for _, n := range nodes {
		nodeState[n.ID] = map[string]interface{}{
			"status": "pending",
			"retries": 0,
		}
	}
	stateJSON := toJSON(nodeState)

	run := &models.DAGRun{
		TemplateID: templateID,
		Status:     models.DAGStatusRunning,
		NodesState: stateJSON,
		Strategy:   strategy,
		MaxRetries: maxRetries,
		Payload:    payload,
	}
	if err := e.dagRepo.CreateRun(ctx, run); err != nil {
		return uuid.Nil, fmt.Errorf("create run: %w", err)
	}

	startedAt := time.Now()
	_ = e.dagRepo.UpdateRunStatus(ctx, run.ID, models.DAGStatusRunning, stateJSON)
	_ = setTimeField(ctx, e.dagRepo, run.ID, "started_at", startedAt)

	e.scheduleReadyNodes(ctx, run.ID, nodes, edges, nodeState, payload)

	if e.auditLog != nil {
		e.auditLog(ctx, "dag_run", run.ID, "started")
	}

	return run.ID, nil
}

func (e *DAGEngine) scheduleReadyNodes(
	ctx context.Context,
	runID uuid.UUID,
	nodes []models.DAGNode,
	edges []models.DAGEdge,
	nodeState map[string]map[string]interface{},
	payload []byte,
) {
	nodeMap := make(map[string]models.DAGNode)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	depsMap := make(map[string]map[string]struct{})
	for _, n := range nodes {
		depsMap[n.ID] = make(map[string]struct{})
	}
	for _, e := range edges {
		depsMap[e.To][e.From] = struct{}{}
	}

	for _, node := range nodes {
		state, _ := nodeState[node.ID]
		status, _ := state["status"].(string)
		if status != "pending" {
			continue
		}
		allDepsDone := true
		deps := depsMap[node.ID]
		for depID := range deps {
			depState, _ := nodeState[depID]
			depStatus, _ := depState["status"].(string)
			if depStatus != "success" {
				allDepsDone = false
				break
			}
		}
		if !allDepsDone {
			continue
		}

		task := &models.Task{
			Type:           node.TaskType,
			Payload:        node.Payload,
			Priority:       node.Priority,
			Status:         models.TaskStatusPending,
			MaxRetries:     3,
			TimeoutSeconds: 300,
			RetryMode:      models.RetryModeExponential,
			DAGID:          &runID,
			DAGNodeID:      &node.ID,
		}
		if len(payload) > 0 {
			task.Payload = payload
		}
		if err := e.taskRepo.Create(ctx, task); err != nil {
			continue
		}
		state["status"] = "running"
		state["task_id"] = task.ID.String()
		nodeState[node.ID] = state

		_ = e.scheduler.EnqueueReady(ctx, task.ID, task.Priority)
		_, _ = e.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusReady)
	}

	newState := toJSON(nodeState)
	_ = e.dagRepo.UpdateRunStatus(ctx, runID, models.DAGStatusRunning, newState)
}

func (e *DAGEngine) HandleTaskComplete(ctx context.Context, task *models.Task) {
	if task.DAGID == nil {
		return
	}

	run, err := e.dagRepo.GetRun(ctx, *task.DAGID)
	if err != nil || run == nil {
		return
	}

	tpl, err := e.dagRepo.GetTemplate(ctx, run.TemplateID)
	if err != nil {
		return
	}

	var nodes []models.DAGNode
	var edges []models.DAGEdge
	_ = parseJSON(tpl.Nodes, &nodes)
	_ = parseJSON(tpl.Edges, &edges)

	var nodeState map[string]map[string]interface{}
	_ = parseJSON(run.NodesState, &nodeState)

	if task.DAGNodeID != nil {
		nodeID := *task.DAGNodeID
		if state, ok := nodeState[nodeID]; ok {
			if task.Status == models.TaskStatusSuccess {
				state["status"] = "success"
			} else if task.Status == models.TaskStatusFailed || task.Status == models.TaskStatusDeadLetter {
				strategy := models.DAGStrategyAbort
				for _, n := range nodes {
					if n.ID == nodeID {
						if n.Strategy != "" {
							strategy = n.Strategy
						}
						break
					}
				}
				if strategy == "" {
					strategy = run.Strategy
				}
				retries, _ := state["retries"].(int)
				if strategy == models.DAGStrategyRetry && retries < run.MaxRetries {
					state["retries"] = retries + 1
					state["status"] = "pending"
					newTask := &models.Task{
						Type:           task.Type,
						Payload:        task.Payload,
						Priority:       task.Priority,
						Status:         models.TaskStatusPending,
						MaxRetries:     3,
						TimeoutSeconds: task.TimeoutSeconds,
						RetryMode:      models.RetryModeExponential,
						DAGID:          task.DAGID,
						DAGNodeID:      task.DAGNodeID,
					}
					_ = e.taskRepo.Create(ctx, newTask)
					_, _ = e.taskRepo.UpdateStatus(ctx, newTask.ID, models.TaskStatusReady)
					_ = e.scheduler.EnqueueReady(ctx, newTask.ID, newTask.Priority)
					state["task_id"] = newTask.ID.String()
				} else if strategy == models.DAGStrategySkip {
					state["status"] = "skipped"
				} else {
					state["status"] = "failed"
					newState := toJSON(nodeState)
					_ = e.dagRepo.UpdateRunStatus(ctx, run.ID, models.DAGStatusFailed, newState)
					return
				}
			}
			nodeState[nodeID] = state
		}
	}

	allDone, allSuccess := checkDAGCompletion(nodeState)
	newState := toJSON(nodeState)
	if allDone {
		if allSuccess {
			_ = e.dagRepo.UpdateRunStatus(ctx, run.ID, models.DAGStatusSuccess, newState)
			now := time.Now()
			_ = setTimeField(ctx, e.dagRepo, run.ID, "ended_at", now)
			if e.auditLog != nil {
				e.auditLog(ctx, "dag_run", run.ID, "completed_success")
			}
		} else {
			_ = e.dagRepo.UpdateRunStatus(ctx, run.ID, models.DAGStatusFailed, newState)
			now := time.Now()
			_ = setTimeField(ctx, e.dagRepo, run.ID, "ended_at", now)
			if e.auditLog != nil {
				e.auditLog(ctx, "dag_run", run.ID, "completed_failed")
			}
		}
	} else {
		e.scheduleReadyNodes(ctx, run.ID, nodes, edges, nodeState, run.Payload)
	}
}

func checkDAGCompletion(nodeState map[string]map[string]interface{}) (allDone bool, allSuccess bool) {
	allDone = true
	allSuccess = true
	for _, state := range nodeState {
		status, _ := state["status"].(string)
		switch status {
		case "pending", "running":
			allDone = false
			allSuccess = false
		case "failed":
			allSuccess = false
		}
	}
	return
}

func parseJSON(raw []byte, v interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

func parseJSONBytes(raw []byte, v interface{}) error {
	return json.Unmarshal(raw, v)
}

func toJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func setTimeField(ctx context.Context, repo *repository.DAGRepository, id uuid.UUID, field string, t time.Time) error {
	return repo.UpdateTimeField(ctx, id, field, t)
}
