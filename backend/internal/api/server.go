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
	"task-queue/internal/autoscaling"
	"task-queue/internal/config"
	"task-queue/internal/metrics"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/ratelimit"
	"task-queue/internal/repository"
	"task-queue/internal/retry"
	"task-queue/internal/worker"
)

type RateLimitConfigManager interface {
	GetConfig(taskType string) *ratelimit.RateLimitConfig
	GetAllConfigs() map[string]*ratelimit.RateLimitConfig
	SetConfig(ctx context.Context, cfg *ratelimit.RateLimitConfig) error
	DeleteConfig(ctx context.Context, taskType string) error
}

type RateLimiter interface {
	GetCurrentRate(ctx context.Context, taskType string) (float64, error)
}

type WaitQueueStats interface {
	GetAllWaitCounts() map[string]int
}

type Server struct {
	cfg                *config.Config
	app                *fiber.App
	taskRepo           *repository.TaskRepository
	workerRepo         *repository.WorkerRepository
	handlerRepo        *repository.HandlerRepository
	deadRepo           *repository.DeadLetterRepository
	dagRepo            *repository.DAGRepository
	traceRepo          *repository.TraceRepository
	alertRepo          *repository.AlertRepository
	scalingRepo        *repository.ScalingRepository
	auditLog           *audit.Logger
	scheduler          *queue.PriorityScheduler
	delaySched         *queue.DelayScheduler
	workerMgr          *worker.Manager
	retryEngine        *retry.Engine
	dagEngine          *retry.DAGEngine
	metrics            *metrics.Collector
	scalingEngine      *autoscaling.Engine
	rateLimitConfigMgr RateLimitConfigManager
	rateLimiter        RateLimiter
	waitQueueStats     WaitQueueStats
}

func NewServer(
	cfg *config.Config,
	taskRepo *repository.TaskRepository,
	workerRepo *repository.WorkerRepository,
	handlerRepo *repository.HandlerRepository,
	deadRepo *repository.DeadLetterRepository,
	dagRepo *repository.DAGRepository,
	traceRepo *repository.TraceRepository,
	alertRepo *repository.AlertRepository,
	scalingRepo *repository.ScalingRepository,
	auditLog *audit.Logger,
	scheduler *queue.PriorityScheduler,
	delaySched *queue.DelayScheduler,
	workerMgr *worker.Manager,
	retryEngine *retry.Engine,
	dagEngine *retry.DAGEngine,
	metricsColl *metrics.Collector,
	scalingEngine *autoscaling.Engine,
	rateLimitConfigMgr RateLimitConfigManager,
	rateLimiter RateLimiter,
	waitQueueStats WaitQueueStats,
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
		cfg:                cfg,
		app:                app,
		taskRepo:           taskRepo,
		workerRepo:         workerRepo,
		handlerRepo:        handlerRepo,
		deadRepo:           deadRepo,
		dagRepo:            dagRepo,
		traceRepo:          traceRepo,
		alertRepo:          alertRepo,
		scalingRepo:        scalingRepo,
		auditLog:           auditLog,
		scheduler:          scheduler,
		delaySched:         delaySched,
		workerMgr:          workerMgr,
		retryEngine:        retryEngine,
		dagEngine:          dagEngine,
		metrics:            metricsColl,
		scalingEngine:      scalingEngine,
		rateLimitConfigMgr: rateLimitConfigMgr,
		rateLimiter:        rateLimiter,
		waitQueueStats:     waitQueueStats,
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
	mon.Get("/duration-heatmap", s.GetDurationHeatmap)
	mon.Get("/duration-histogram", s.GetDurationHistogram)

	rl := api.Group("/rate-limit")
	rl.Get("/configs", s.ListRateLimitConfigs)
	rl.Get("/configs/:taskType", s.GetRateLimitConfig)
	rl.Put("/configs/:taskType", s.SetRateLimitConfig)
	rl.Delete("/configs/:taskType", s.DeleteRateLimitConfig)
	rl.Get("/status", s.GetRateLimitStatus)
	rl.Get("/throttle-stats", s.GetRateLimitThrottleStats)

	trace := api.Group("/trace")
	trace.Get("", s.ListTraces)
	trace.Get("/analysis/bottleneck", s.GetBottleneckAnalysis)
	trace.Get("/:taskId", s.GetTraceDetail)

	alerts := api.Group("/alerts")
	alerts.Get("/rules", s.ListAlertRules)
	alerts.Get("/rules/:id", s.GetAlertRule)
	alerts.Post("/rules", s.CreateAlertRule)
	alerts.Put("/rules/:id", s.UpdateAlertRule)
	alerts.Delete("/rules/:id", s.DeleteAlertRule)
	alerts.Patch("/rules/:id/toggle", s.ToggleAlertRule)
	alerts.Get("/history", s.ListAlertHistory)

	scaling := api.Group("/auto-scaling")
	scaling.Get("/policies", s.ListScalingPolicies)
	scaling.Get("/policies/:id", s.GetScalingPolicy)
	scaling.Post("/policies", s.CreateScalingPolicy)
	scaling.Put("/policies/:id", s.UpdateScalingPolicy)
	scaling.Delete("/policies/:id", s.DeleteScalingPolicy)
	scaling.Patch("/policies/:id/toggle", s.ToggleScalingPolicy)
	scaling.Get("/metrics", s.GetScalingMetrics)
	scaling.Get("/history", s.ListScalingHistory)

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
		_, _ = s.taskRepo.UpdateStatus(ctx, task.ID, models.TaskStatusReady)
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
	if _, err := s.taskRepo.UpdateStatus(ctx, id, models.TaskStatusCancelled); err != nil {
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
	if _, err := s.taskRepo.UpdateStatus(ctx, id, models.TaskStatusReady,
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
	nodesJSON := toJSON(req.Nodes)
	edgesJSON := toJSON(req.Edges)
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

func (s *Server) GetDurationHeatmap(c *fiber.Ctx) error {
	days := c.QueryInt("days", 7)
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	taskType := c.Query("type", "")
	compare := c.QueryBool("compare", false)

	if compare {
		data, err := s.taskRepo.GetDurationHeatmapCompare(c.UserContext(), days, taskType)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	}

	data, err := s.taskRepo.GetDurationHeatmap(c.UserContext(), days, taskType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

func (s *Server) GetDurationHistogram(c *fiber.Ctx) error {
	ctx := c.UserContext()
	taskType := c.Query("type", "")

	var from, to time.Time
	var err error
	if fromStr := c.Query("from"); fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid from time"})
		}
	} else {
		from = time.Now().Add(-1 * time.Hour)
	}
	if toStr := c.Query("to"); toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid to time"})
		}
	} else {
		to = time.Now()
	}

	if from.After(to) {
		return c.Status(400).JSON(fiber.Map{"error": "from must be before to"})
	}

	compareFromStr := c.Query("compare_from", "")
	compareToStr := c.Query("compare_to", "")

	if compareFromStr != "" && compareToStr != "" {
		var compareFrom, compareTo time.Time
		compareFrom, err = time.Parse(time.RFC3339, compareFromStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid compare_from time"})
		}
		compareTo, err = time.Parse(time.RFC3339, compareToStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid compare_to time"})
		}
		if compareFrom.After(compareTo) {
			return c.Status(400).JSON(fiber.Map{"error": "compare_from must be before compare_to"})
		}

		data, err := s.taskRepo.GetDurationHistogramCompare(ctx, from, to, compareFrom, compareTo, taskType)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	}

	data, err := s.taskRepo.GetDurationHistogram(ctx, from, to, taskType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

func (s *Server) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

type SetRateLimitConfigRequest struct {
	MaxPerSecond int  `json:"max_per_second"`
	WindowSizeMs int  `json:"window_size_ms"`
	Enabled      bool `json:"enabled"`
}

func (s *Server) ListRateLimitConfigs(c *fiber.Ctx) error {
	if s.rateLimitConfigMgr == nil {
		return c.Status(503).JSON(fiber.Map{"error": "rate limit not available"})
	}
	configs := s.rateLimitConfigMgr.GetAllConfigs()
	return c.JSON(configs)
}

func (s *Server) GetRateLimitConfig(c *fiber.Ctx) error {
	if s.rateLimitConfigMgr == nil {
		return c.Status(503).JSON(fiber.Map{"error": "rate limit not available"})
	}
	taskType := c.Params("taskType")
	cfg := s.rateLimitConfigMgr.GetConfig(taskType)
	if cfg == nil {
		return c.Status(404).JSON(fiber.Map{"error": "config not found"})
	}
	return c.JSON(cfg)
}

func (s *Server) SetRateLimitConfig(c *fiber.Ctx) error {
	if s.rateLimitConfigMgr == nil {
		return c.Status(503).JSON(fiber.Map{"error": "rate limit not available"})
	}
	taskType := c.Params("taskType")
	if taskType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "task type is required"})
	}

	var req SetRateLimitConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	cfg := &ratelimit.RateLimitConfig{
		TaskType:     taskType,
		MaxPerSecond: req.MaxPerSecond,
		WindowSizeMs: req.WindowSizeMs,
		Enabled:      req.Enabled,
	}

	if err := s.rateLimitConfigMgr.SetConfig(c.UserContext(), cfg); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	s.auditLog.Log("rate_limit_config", uuid.New(), "updated", audit.WithRemoteAddr(c.IP()),
		audit.WithExtra(map[string]interface{}{
			"task_type":      taskType,
			"max_per_second": req.MaxPerSecond,
			"window_size_ms": req.WindowSizeMs,
			"enabled":        req.Enabled,
		}))

	return c.JSON(cfg)
}

func (s *Server) DeleteRateLimitConfig(c *fiber.Ctx) error {
	if s.rateLimitConfigMgr == nil {
		return c.Status(503).JSON(fiber.Map{"error": "rate limit not available"})
	}
	taskType := c.Params("taskType")
	if taskType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "task type is required"})
	}

	if err := s.rateLimitConfigMgr.DeleteConfig(c.UserContext(), taskType); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	s.auditLog.Log("rate_limit_config", uuid.New(), "deleted", audit.WithRemoteAddr(c.IP()),
		audit.WithExtra(map[string]interface{}{
			"task_type": taskType,
		}))

	return c.JSON(fiber.Map{"status": "deleted"})
}

func (s *Server) GetRateLimitStatus(c *fiber.Ctx) error {
	if s.rateLimitConfigMgr == nil || s.rateLimiter == nil {
		return c.Status(503).JSON(fiber.Map{"error": "rate limit not available"})
	}

	ctx := c.UserContext()
	configs := s.rateLimitConfigMgr.GetAllConfigs()
	waitCounts := make(map[string]int)
	if s.waitQueueStats != nil {
		waitCounts = s.waitQueueStats.GetAllWaitCounts()
	}

	statusList := make([]*models.RateLimitStatus, 0, len(configs))
	for taskType, cfg := range configs {
		currentRate, _ := s.rateLimiter.GetCurrentRate(ctx, taskType)
		usagePercent := 0.0
		if cfg.MaxPerSecond > 0 {
			usagePercent = (currentRate / float64(cfg.MaxPerSecond)) * 100
		}
		status := &models.RateLimitStatus{
			TaskType:      taskType,
			CurrentRate:   currentRate,
			MaxPerSecond:  cfg.MaxPerSecond,
			WindowSizeMs:  cfg.WindowSizeMs,
			UsagePercent:  usagePercent,
			WaitQueueSize: waitCounts[taskType],
			Enabled:       cfg.Enabled,
		}
		statusList = append(statusList, status)
	}

	return c.JSON(statusList)
}

func (s *Server) GetRateLimitThrottleStats(c *fiber.Ctx) error {
	hours := c.QueryInt("hours", 1)
	if hours <= 0 {
		hours = 1
	}
	if hours > 24 {
		hours = 24
	}

	stats, err := s.metrics.GetThrottleCounts(c.UserContext(), time.Duration(hours)*time.Hour)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]*models.RateLimitThrottleStats, 0, len(stats))
	for taskType, count := range stats {
		result = append(result, &models.RateLimitThrottleStats{
			TaskType:      taskType,
			ThrottleCount: count,
			WindowHours:   hours,
		})
	}

	return c.JSON(result)
}

func (s *Server) ListTraces(c *fiber.Ctx) error {
	ctx := c.UserContext()
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	filter := repository.TraceFilter{}
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
	if t := c.Query("type"); t != "" {
		filter.TaskType = t
	}
	if statuses := c.Query("final_statuses"); statuses != "" {
		for _, s2 := range splitAndTrim(statuses, ",") {
			filter.FinalStatus = append(filter.FinalStatus, models.TaskStatus(s2))
		}
	} else if status := c.Query("final_status"); status != "" {
		filter.FinalStatus = append(filter.FinalStatus, models.TaskStatus(status))
	}

	traces, total, err := s.traceRepo.ListTraces(ctx, filter, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": traces, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) GetTraceDetail(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid task id"})
	}
	detail, err := s.traceRepo.GetTraceDetail(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}

func (s *Server) GetBottleneckAnalysis(c *fiber.Ctx) error {
	ctx := c.UserContext()
	taskType := c.Query("type", "")
	var from, to time.Time
	var err error
	if fromStr := c.Query("from"); fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid from time"})
		}
	} else {
		from = time.Now().Add(-1 * time.Hour)
	}
	if toStr := c.Query("to"); toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid to time"})
		}
	} else {
		to = time.Now()
	}

	result, err := s.traceRepo.AnalyzeBottleneck(ctx, from, to, taskType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		result = append(result, s[start:])
	}
	return result
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

type CreateAlertRuleRequest struct {
	Name            string  `json:"name" validate:"required"`
	TaskType        *string `json:"task_type"`
	ConditionType   string  `json:"condition_type" validate:"required"`
	Threshold       float64 `json:"threshold" validate:"required"`
	WindowMinutes   int     `json:"window_minutes"`
	CooldownMinutes int     `json:"cooldown_minutes"`
	NotifyType      string  `json:"notify_type"`
	WebhookURL      *string `json:"webhook_url"`
	Enabled         *bool   `json:"enabled"`
}

func (s *Server) CreateAlertRule(c *fiber.Ctx) error {
	var req CreateAlertRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if req.ConditionType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "condition_type is required"})
	}
	if req.Threshold <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "threshold must be positive"})
	}

	condType := models.AlertConditionType(req.ConditionType)
	switch condType {
	case models.AlertConditionDurationP95, models.AlertConditionFailureRate, models.AlertConditionQueueBacklog:
	default:
		return c.Status(400).JSON(fiber.Map{"error": "invalid condition_type"})
	}

	notifyType := models.AlertNotifyWebhook
	if req.NotifyType != "" {
		notifyType = models.AlertNotifyType(req.NotifyType)
		if notifyType != models.AlertNotifyWebhook {
			return c.Status(400).JSON(fiber.Map{"error": "invalid notify_type"})
		}
	}
	if notifyType == models.AlertNotifyWebhook && (req.WebhookURL == nil || *req.WebhookURL == "") {
		return c.Status(400).JSON(fiber.Map{"error": "webhook_url is required for webhook notification"})
	}

	window := req.WindowMinutes
	if window <= 0 {
		window = 5
	}
	cooldownSeconds := req.CooldownMinutes * 60
	if cooldownSeconds <= 0 {
		cooldownSeconds = 300
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	create := &repository.AlertRuleCreate{
		Name:            req.Name,
		TaskType:        req.TaskType,
		ConditionType:   condType,
		Threshold:       req.Threshold,
		WindowMinutes:   window,
		CooldownSeconds: cooldownSeconds,
		NotifyType:      notifyType,
		WebhookURL:      req.WebhookURL,
		Enabled:         enabled,
	}

	rule, err := s.alertRepo.CreateRule(c.UserContext(), create)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("alert_rule", rule.ID, "created", audit.WithRemoteAddr(c.IP()))
	return c.Status(201).JSON(rule)
}

func (s *Server) ListAlertRules(c *fiber.Ctx) error {
	rules, err := s.alertRepo.ListRules(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(rules)
}

func (s *Server) GetAlertRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	rule, err := s.alertRepo.GetRule(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "rule not found"})
	}
	return c.JSON(rule)
}

type UpdateAlertRuleRequest struct {
	Name            *string `json:"name"`
	TaskType        **string `json:"task_type"`
	ConditionType   *string `json:"condition_type"`
	Threshold       *float64 `json:"threshold"`
	WindowMinutes   *int    `json:"window_minutes"`
	CooldownMinutes *int    `json:"cooldown_minutes"`
	NotifyType      *string `json:"notify_type"`
	WebhookURL      **string `json:"webhook_url"`
	Enabled         *bool   `json:"enabled"`
}

func (s *Server) UpdateAlertRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	var req UpdateAlertRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	update := &repository.AlertRuleUpdate{
		Name:      req.Name,
		TaskType:  req.TaskType,
		Threshold: req.Threshold,
		Enabled:   req.Enabled,
	}
	if req.ConditionType != nil {
		ct := models.AlertConditionType(*req.ConditionType)
		switch ct {
		case models.AlertConditionDurationP95, models.AlertConditionFailureRate, models.AlertConditionQueueBacklog:
			update.ConditionType = &ct
		default:
			return c.Status(400).JSON(fiber.Map{"error": "invalid condition_type"})
		}
	}
	if req.WindowMinutes != nil && *req.WindowMinutes > 0 {
		update.WindowMinutes = req.WindowMinutes
	}
	if req.CooldownMinutes != nil && *req.CooldownMinutes > 0 {
		cs := *req.CooldownMinutes * 60
		update.CooldownSeconds = &cs
	}
	if req.NotifyType != nil {
		nt := models.AlertNotifyType(*req.NotifyType)
		if nt != models.AlertNotifyWebhook {
			return c.Status(400).JSON(fiber.Map{"error": "invalid notify_type"})
		}
		update.NotifyType = &nt
	}
	if req.WebhookURL != nil {
		update.WebhookURL = req.WebhookURL
	}

	rule, err := s.alertRepo.UpdateRule(c.UserContext(), id, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("alert_rule", id, "updated", audit.WithRemoteAddr(c.IP()))
	return c.JSON(rule)
}

func (s *Server) ToggleAlertRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	rule, err := s.alertRepo.GetRule(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "rule not found"})
	}
	newEnabled := !rule.Enabled
	update := &repository.AlertRuleUpdate{Enabled: &newEnabled}
	updated, err := s.alertRepo.UpdateRule(c.UserContext(), id, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	action := "disabled"
	if newEnabled {
		action = "enabled"
	}
	s.auditLog.Log("alert_rule", id, action, audit.WithRemoteAddr(c.IP()))
	return c.JSON(updated)
}

func (s *Server) DeleteAlertRule(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := s.alertRepo.DeleteRule(c.UserContext(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("alert_rule", id, "deleted", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "deleted"})
}

func (s *Server) ListAlertHistory(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	filter := repository.AlertHistoryFilter{
		Limit:    limit,
		Offset:   offset,
		RuleName: c.Query("rule_name", ""),
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
	if rid := c.Query("rule_id"); rid != "" {
		if id, err := uuid.Parse(rid); err == nil {
			filter.RuleID = &id
		}
	}

	history, total, err := s.alertRepo.ListHistory(c.UserContext(), filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"items":  history,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type CreateScalingPolicyRequest struct {
	TaskType              string  `json:"task_type" validate:"required"`
	TargetUtilizationPct  float64 `json:"target_utilization_pct"`
	MinWorkers            int     `json:"min_workers"`
	MaxWorkers            int     `json:"max_workers"`
	CooldownSeconds       int     `json:"cooldown_seconds"`
	ScaleInProtectionSecs int     `json:"scale_in_protection_secs"`
	ScaleOutThreshold     int     `json:"scale_out_threshold"`
	ScaleInThresholdPct   float64 `json:"scale_in_threshold_pct"`
	Enabled               *bool   `json:"enabled"`
}

func (s *Server) CreateScalingPolicy(c *fiber.Ctx) error {
	var req CreateScalingPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if req.TaskType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "task_type is required"})
	}
	if req.MinWorkers < 0 {
		req.MinWorkers = 1
	}
	if req.MaxWorkers <= 0 {
		req.MaxWorkers = 10
	}
	if req.MaxWorkers < req.MinWorkers {
		return c.Status(400).JSON(fiber.Map{"error": "max_workers must be >= min_workers"})
	}
	if req.TargetUtilizationPct <= 0 || req.TargetUtilizationPct > 100 {
		req.TargetUtilizationPct = 70
	}
	if req.CooldownSeconds <= 0 {
		req.CooldownSeconds = 300
	}
	if req.ScaleInProtectionSecs <= 0 {
		req.ScaleInProtectionSecs = 600
	}
	if req.ScaleOutThreshold <= 0 {
		req.ScaleOutThreshold = 10
	}
	if req.ScaleInThresholdPct <= 0 || req.ScaleInThresholdPct >= 100 {
		req.ScaleInThresholdPct = 30
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	create := &repository.ScalingPolicyCreate{
		TaskType:              req.TaskType,
		TargetUtilizationPct:  req.TargetUtilizationPct,
		MinWorkers:            req.MinWorkers,
		MaxWorkers:            req.MaxWorkers,
		CooldownSeconds:       req.CooldownSeconds,
		ScaleInProtectionSecs: req.ScaleInProtectionSecs,
		ScaleOutThreshold:     req.ScaleOutThreshold,
		ScaleInThresholdPct:   req.ScaleInThresholdPct,
		Enabled:               enabled,
	}

	policy, err := s.scalingRepo.CreatePolicy(c.UserContext(), create)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("scaling_policy", policy.ID, "created", audit.WithRemoteAddr(c.IP()))
	return c.Status(201).JSON(policy)
}

func (s *Server) ListScalingPolicies(c *fiber.Ctx) error {
	policies, err := s.scalingRepo.ListPolicies(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(policies)
}

func (s *Server) GetScalingPolicy(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	policy, err := s.scalingRepo.GetPolicy(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "policy not found"})
	}
	return c.JSON(policy)
}

type UpdateScalingPolicyRequest struct {
	TargetUtilizationPct  float64 `json:"target_utilization_pct"`
	MinWorkers            int     `json:"min_workers"`
	MaxWorkers            int     `json:"max_workers"`
	CooldownSeconds       int     `json:"cooldown_seconds"`
	ScaleInProtectionSecs int     `json:"scale_in_protection_secs"`
	ScaleOutThreshold     int     `json:"scale_out_threshold"`
	ScaleInThresholdPct   float64 `json:"scale_in_threshold_pct"`
	Enabled               bool    `json:"enabled"`
}

func (s *Server) UpdateScalingPolicy(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}

	var raw map[string]interface{}
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	var req UpdateScalingPolicyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	update := &repository.ScalingPolicyUpdate{}

	if _, ok := raw["target_utilization_pct"]; ok {
		v := req.TargetUtilizationPct
		update.TargetUtilizationPct = &v
	}
	if _, ok := raw["min_workers"]; ok {
		v := req.MinWorkers
		update.MinWorkers = &v
	}
	if _, ok := raw["max_workers"]; ok {
		v := req.MaxWorkers
		update.MaxWorkers = &v
	}
	if _, ok := raw["cooldown_seconds"]; ok {
		v := req.CooldownSeconds
		update.CooldownSeconds = &v
	}
	if _, ok := raw["scale_in_protection_secs"]; ok {
		v := req.ScaleInProtectionSecs
		update.ScaleInProtectionSecs = &v
	}
	if _, ok := raw["scale_out_threshold"]; ok {
		v := req.ScaleOutThreshold
		update.ScaleOutThreshold = &v
	}
	if _, ok := raw["scale_in_threshold_pct"]; ok {
		v := req.ScaleInThresholdPct
		update.ScaleInThresholdPct = &v
	}
	if _, ok := raw["enabled"]; ok {
		v := req.Enabled
		update.Enabled = &v
	}

	updated, err := s.scalingRepo.UpdatePolicy(c.UserContext(), id, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("scaling_policy", id, "updated", audit.WithRemoteAddr(c.IP()))
	return c.JSON(updated)
}

func (s *Server) ToggleScalingPolicy(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	policy, err := s.scalingRepo.GetPolicy(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "policy not found"})
	}
	newEnabled := !policy.Enabled
	update := &repository.ScalingPolicyUpdate{Enabled: &newEnabled}
	updated, err := s.scalingRepo.UpdatePolicy(c.UserContext(), id, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	action := "disabled"
	if newEnabled {
		action = "enabled"
	}
	s.auditLog.Log("scaling_policy", id, action, audit.WithRemoteAddr(c.IP()))
	return c.JSON(updated)
}

func (s *Server) DeleteScalingPolicy(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := s.scalingRepo.DeletePolicy(c.UserContext(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	s.auditLog.Log("scaling_policy", id, "deleted", audit.WithRemoteAddr(c.IP()))
	return c.JSON(fiber.Map{"status": "deleted"})
}

func (s *Server) GetScalingMetrics(c *fiber.Ctx) error {
	if s.scalingEngine == nil {
		return c.Status(503).JSON(fiber.Map{"error": "scaling engine not available"})
	}
	metrics, err := s.scalingEngine.GetPolicyMetrics(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(metrics)
}

func (s *Server) ListScalingHistory(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 500 {
		limit = 500
	}

	filter := repository.ScalingHistoryFilter{
		Limit:    limit,
		Offset:   offset,
		TaskType: c.Query("task_type", ""),
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
	if pid := c.Query("policy_id"); pid != "" {
		if id, err := uuid.Parse(pid); err == nil {
			filter.PolicyID = &id
		}
	}

	history, total, err := s.scalingRepo.ListHistory(c.UserContext(), filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"items":  history,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
