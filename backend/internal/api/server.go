package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"task-queue/internal/audit"
	"task-queue/internal/config"
	"task-queue/internal/metrics"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/repository"
	"task-queue/internal/retry"
	"task-queue/internal/worker"
)

type Server struct {
	cfg          *config.Config
	app          *fiber.App
	taskRepo     *repository.TaskRepository
	workerRepo   *repository.WorkerRepository
	handlerRepo  *repository.HandlerRepository
	deadRepo     *repository.DeadLetterRepository
	dagRepo      *repository.DAGRepository
	auditLog     *audit.Logger
	scheduler    *queue.PriorityScheduler
	delaySched   *queue.DelayScheduler
	workerMgr    *worker.Manager
	retryEngine  *retry.Engine
	dagEngine    *retry.DAGEngine
	metrics      *metrics.Collector
}

func NewServer(
	cfg *config.Config,
	taskRepo *repository.TaskRepository,
	workerRepo *repository.WorkerRepository,
	handlerRepo *repository.HandlerRepository,
	deadRepo *repository.DeadLetterRepository,
	dagRepo *repository.DAGRepository,
	auditLog *audit.Logger,
	scheduler *queue.PriorityScheduler,
	delaySched *queue.DelayScheduler,
	workerMgr *worker.Manager,
	retryEngine *retry.Engine,
	dagEngine *retry.DAGEngine,
	metricsColl *metrics.Collector,
) *Server {
	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		BodyLimit:    50 * 1024 * 1024,
	})

	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "*",
	}))
	app.Use(logger.New())

	s := &Server{
		cfg:         cfg,
		app:         app,
		taskRepo:    taskRepo,
		workerRepo:  workerRepo,
		handlerRepo: handlerRepo,
		deadRepo:    deadRepo,
		dagRepo:     dagRepo,
		auditLog:    auditLog,
		scheduler:   scheduler,
		delaySched:  delaySched,
		workerMgr:   workerMgr,
		retryEngine: retryEngine,
		dagEngine:   dagEngine,
		metrics:     metricsColl,
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	api := s.app.Group("/api/v1")

	tasks := api.Group("/tasks")
	tasks.Post("", s.CreateTask)
	tasks.Get("", s.ListTasks)
	tasks.Get("/:id", s.GetTask)
	tasks.Get("/:id/executions", s.GetTaskExecutions)
	tasks.Post("/:id/cancel", s.CancelTask)
	tasks.Post("/:id/retry", s.RetryTask)

	workers := api.Group("/workers")
	workers.Get("", s.ListWorkers)
	workers.Get("/:id", s.GetWorker)
	workers.Post("/register", s.RegisterWorker)
	workers.Post("/:id/heartbeat", s.WorkerHeartbeat)
	workers.Post("/:id/shutdown", s.ShutdownWorker)

	handlers := api.Group("/handlers")
	handlers.Post("/register", s.RegisterHandler)
	handlers.Get("/:taskType", s.ListHandlersByType)

	dead := api.Group("/dead-letter")
	dead.Get("", s.ListDeadLetter)
	dead.Get("/:id", s.GetDeadLetterDetail)
	dead.Post("/:id/retry", s.RetryDeadLetter)
	dead.Post("/:id/discard", s.DiscardDeadLetter)
	dead.Post("/batch-retry", s.BatchRetryDeadLetter)
	dead.Post("/batch-discard", s.BatchDiscardDeadLetter)
	dead.Get("/stats/by-error", s.DeadLetterByError)

	dag := api.Group("/dags")
	dag.Post("/templates", s.CreateDAGTemplate)
	dag.Get("/templates", s.ListDAGTemplates)
	dag.Get("/templates/:id", s.GetDAGTemplate)
	dag.Post("/templates/:id/run", s.RunDAG)
	dag.Get("/runs", s.ListDAGRuns)
	dag.Get("/runs/:id", s.GetDAGRun)

	mon := api.Group("/metrics")
	mon.Get("/snapshot", s.GetMetricsSnapshot)
	mon.Get("/throughput-history", s.GetThroughputHistory)
	mon.Get("/queue-depths", s.GetQueueDepths)

	s.app.Get("/health", s.HealthCheck)
}

func (s *Server) Listen(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Listen(s.cfg.Server.Addr())
	}()
	select {
	case <-ctx.Done():
		return s.app.Shutdown()
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

type CreateTaskRequest struct {
	Type           string          `json:"type" validate:"required"`
	Payload        json.RawMessage `json:"payload"`
	Priority       string          `json:"priority"`
	DelaySeconds   int             `json:"delay_seconds"`
	MaxRetries     int             `json:"max_retries"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	CallbackURL    string          `json:"callback_url"`
	RetryMode      string          `json:"retry_mode"`
	RetryInterval  int             `json:"retry_interval"`
	RetryCronExpr  string          `json:"retry_cron_expr"`
}

func (s *Server) CreateTask(c *fiber.Ctx) error {
	var req CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Type == "" {
		return c.Status(400).JSON(fiber.Map{"error": "task type is required"})
	}

	priority := models.PriorityNormal
	if req.Priority != "" {
		priority = models.PriorityFromString(req.Priority)
	}
	retryMode := models.RetryModeExponential
	if req.RetryMode != "" {
		retryMode = models.RetryMode(req.RetryMode)
	}
	payload := req.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	task := &models.Task{
		Type:           req.Type,
		Payload:        payload,
		Priority:       priority,
		DelaySeconds:   req.DelaySeconds,
		MaxRetries:     maxRetries,
		TimeoutSeconds: timeout,
		CallbackURL:    req.CallbackURL,
		RetryMode:      retryMode,
		RetryInterval:  req.RetryInterval,
		RetryCronExpr:  req.RetryCronExpr,
	}

	ctx := c.UserContext()
	if task.DelaySeconds > 0 {
		scheduledAt := time.Now().Add(time.Duration(task.DelaySeconds) * time.Second)
		task.ScheduledAt = &scheduledAt
		task.Status = models.TaskStatusDelayed
	} else {
		task.Status = models.TaskStatusReady
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	s.auditLog.Log("task", task.ID, "created", audit.WithRemoteAddr(c.IP()))

	if task.Status == models.TaskStatusDelayed {
		_ = s.delaySched.EnqueueDelay(ctx, task.ID, *task.ScheduledAt)
	} else {
		_ = s.scheduler.EnqueueReady(ctx, task.ID, task.Priority)
		_ = s.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusReady)
	}

	return c.Status(201).JSON(task)
}

func (s *Server) ListTasks(c *fiber.Ctx) error {
	ctx := c.UserContext()
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	filter := repository.TaskFilter{}
	if status := c.Query("status"); status != "" {
		filter.Status = models.TaskStatus(status)
	}
	if t := c.Query("type"); t != "" {
		filter.Type = t
	}
	if p := c.Query("priority"); p != "" {
		pri := models.PriorityFromString(p)
		filter.Priority = &pri
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &t
		}
	}

	tasks, total, err := s.taskRepo.List(ctx, filter, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": tasks, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) GetTask(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	task, err := s.taskRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "task not found"})
	}
	return c.JSON(task)
}

func (s *Server) GetTaskExecutions(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	execs, err := s.taskRepo.GetExecutions(c.UserContext(), id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(execs)
}

func (s *Server) CancelTask(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	ctx := c.UserContext()
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "task not found"})
	}
	if task.Status == models.TaskStatusSuccess || task.Status == models.TaskStatusFailed ||
		task.Status == models.TaskStatusDeadLetter || task.Status == models.TaskStatusCancelled {
		return c.Status(400).JSON(fiber.Map{"error": "task already in terminal state"})
	}
	if err := s.taskRepo.UpdateStatus(ctx, id, models.TaskStatusCancelled); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("task", id, "cancelled", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "cancelled"})
}

func (s *Server) RetryTask(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	ctx := c.UserContext()
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "task not found"})
	}
	if err := s.taskRepo.UpdateStatus(ctx, id, models.TaskStatusReady,
		"retry_count", 0, "completed_at", nil, "last_error", nil); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	_ = s.scheduler.EnqueueReady(ctx, id, task.Priority)
	s.auditLog.Log("task", id, "manual_retry", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "queued_for_retry"})
}

func (s *Server) ListWorkers(c *fiber.Ctx) error {
	workers, err := s.workerRepo.List(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(workers)
}

func (s *Server) GetWorker(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid worker id"})
	}
	w, err := s.workerRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "worker not found"})
	}
	taskIDs, _ := s.workerRepo.GetTaskIDsByWorker(c.UserContext(), id)
	w.RunningTasks = taskIDs
	return c.JSON(w)
}

type RegisterWorkerRequest struct {
	ID         *uuid.UUID `json:"id"`
	Name       string     `json:"name"`
	Hostname   string     `json:"hostname"`
	TotalSlots int        `json:"total_slots"`
}

func (s *Server) RegisterWorker(c *fiber.Ctx) error {
	var req RegisterWorkerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	slots := req.TotalSlots
	if slots <= 0 {
		slots = s.cfg.Worker.DefaultSlots
	}
	w := &models.Worker{
		Name:       req.Name,
		Hostname:   req.Hostname,
		TotalSlots: slots,
		Status:     models.WorkerStatusOnline,
	}
	if req.ID != nil {
		w.ID = *req.ID
	}
	if err := s.workerRepo.Register(c.UserContext(), w); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("worker", w.ID, "registered", audit.WithRemoteAddr(c.IP()))
	return c.Status(201).JSON(w)
}

type HeartbeatRequest struct {
	UsedSlots int `json:"used_slots"`
}

func (s *Server) WorkerHeartbeat(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid worker id"})
	}
	var req HeartbeatRequest
	if err := c.BodyParser(&req); err == nil {
		if err := s.workerRepo.Heartbeat(c.UserContext(), id, req.UsedSlots); err != nil {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if err := s.workerRepo.Heartbeat(c.UserContext(), id, 0); err != nil {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *Server) ShutdownWorker(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid worker id"})
	}
	if err := s.workerRepo.UpdateStatus(c.UserContext(), id, models.WorkerStatusDraining); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("worker", id, "shutdown_initiated", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "draining"})
}

type RegisterHandlerRequest struct {
	TaskType string `json:"task_type"`
	WorkerID string `json:"worker_id"`
	Endpoint string `json:"endpoint"`
}

func (s *Server) RegisterHandler(c *fiber.Ctx) error {
	var req RegisterHandlerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.TaskType == "" || req.WorkerID == "" || req.Endpoint == "" {
		return c.Status(400).JSON(fiber.Map{"error": "task_type, worker_id, endpoint required"})
	}
	wID, err := uuid.Parse(req.WorkerID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid worker_id"})
	}
	h := &models.HandlerRegistration{
		TaskType:  req.TaskType,
		HandlerID: req.WorkerID + "-" + req.TaskType,
		WorkerID:  wID,
		Endpoint:  req.Endpoint,
	}
	if err := s.handlerRepo.Register(c.UserContext(), h); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(h)
}

func (s *Server) ListHandlersByType(c *fiber.Ctx) error {
	handlers, err := s.handlerRepo.FindByTaskType(c.UserContext(), c.Params("taskType"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(handlers)
}

func (s *Server) ListDeadLetter(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}
	tasks, total, err := s.deadRepo.List(c.UserContext(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": tasks, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) GetDeadLetterDetail(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	ctx := c.UserContext()
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "task not found"})
	}
	if task.Status != models.TaskStatusDeadLetter {
		return c.Status(400).JSON(fiber.Map{"error": "task is not in dead letter"})
	}
	history, _ := s.deadRepo.GetErrorHistory(ctx, id)
	execs, _ := s.taskRepo.GetExecutions(ctx, id)
	return c.JSON(fiber.Map{
		"task":           task,
		"error_history":  history,
		"executions":     execs,
	})
}

func (s *Server) RetryDeadLetter(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	n, err := s.retryEngine.RetryDeadLetter(c.UserContext(), []uuid.UUID{id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if n == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "failed to retry task"})
	}
	s.auditLog.Log("task", id, "dead_letter_retried", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "retried"})
}

func (s *Server) DiscardDeadLetter(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	n, err := s.retryEngine.DiscardDeadLetter(c.UserContext(), []uuid.UUID{id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if n == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "failed to discard task"})
	}
	s.auditLog.Log("task", id, "dead_letter_discarded", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "discarded"})
}

type BatchRequest struct {
	IDs []string `json:"ids"`
}

func (s *Server) BatchRetryDeadLetter(c *fiber.Ctx) error {
	var req BatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, s2 := range req.IDs {
		if id, err := uuid.Parse(s2); err == nil {
			ids = append(ids, id)
		}
	}
	n, err := s.retryEngine.RetryDeadLetter(c.UserContext(), ids)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	for _, id := range ids {
		s.auditLog.Log("task", id, "batch_retried", audit.WithRemoteAddr(c.IP()))
	}
	return c.JSON(fiber.Map{"retried": n})
}

func (s *Server) BatchDiscardDeadLetter(c *fiber.Ctx) error {
	var req BatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, s2 := range req.IDs {
		if id, err := uuid.Parse(s2); err == nil {
			ids = append(ids, id)
		}
	}
	n, err := s.retryEngine.DiscardDeadLetter(c.UserContext(), ids)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	for _, id := range ids {
		s.auditLog.Log("task", id, "batch_discarded", audit.WithRemoteAddr(c.IP()))
	}
	return c.JSON(fiber.Map{"discarded": n})
}

func (s *Server) DeadLetterByError(c *fiber.Ctx) error {
	stats, err := s.deadRepo.GroupByError(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

type CreateDAGTemplateRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Nodes       []models.DAGNode   `json:"nodes"`
	Edges       []models.DAGEdge   `json:"edges"`
}

func (s *Server) CreateDAGTemplate(c *fiber.Ctx) error {
	var req CreateDAGTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name required"})
	}
	if len(req.Nodes) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "at least one node required"})
	}
	nodesJSON, _ := toJSON(req.Nodes)
	edgesJSON, _ := toJSON(req.Edges)
	tpl := &models.DAGTemplate{
		Name:        req.Name,
		Description: req.Description,
		Nodes:       nodesJSON,
		Edges:       edgesJSON,
	}
	if err := s.dagRepo.CreateTemplate(c.UserContext(), tpl); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("dag_template", tpl.ID, "created", audit.WithRemoteAddr(c.IP()))
	return c.Status(201).JSON(tpl)
}

func (s *Server) ListDAGTemplates(c *fiber.Ctx) error {
	tpls, err := s.dagRepo.ListTemplates(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tpls)
}

func (s *Server) GetDAGTemplate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	tpl, err := s.dagRepo.GetTemplate(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(tpl)
}

type RunDAGRequest struct {
	Payload    []byte `json:"payload"`
	Strategy   string `json:"strategy"`
	MaxRetries int    `json:"max_retries"`
}

func (s *Server) RunDAG(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid template id"})
	}
	var req RunDAGRequest
	if err := c.BodyParser(&req); err == nil && req.Payload == nil {
		req.Payload = []byte(`{}`)
	}
	strategy := models.DAGStrategyAbort
	if req.Strategy != "" {
		strategy = models.DAGNodeStrategy(req.Strategy)
	}
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	runID, err := s.dagEngine.StartDAG(c.UserContext(), id, req.Payload, strategy, maxRetries)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"run_id": runID})
}

func (s *Server) ListDAGRuns(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}
	runs, total, err := s.dagRepo.ListRuns(c.UserContext(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": runs, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) GetDAGRun(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	run, err := s.dagRepo.GetRun(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(run)
}

func (s *Server) GetMetricsSnapshot(c *fiber.Ctx) error {
	snap := s.metrics.Snapshot(c.UserContext())
	return c.JSON(snap)
}

func (s *Server) GetThroughputHistory(c *fiber.Ctx) error {
	hours := c.QueryInt("hours", 24)
	if hours > 168 {
		hours = 168
	}
	history, err := s.metrics.GetThroughputHistory(c.UserContext(), hours)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(history)
}

func (s *Server) GetQueueDepths(c *fiber.Ctx) error {
	depths, err := s.scheduler.GetQueueDepths(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(depths)
}

func (s *Server) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
