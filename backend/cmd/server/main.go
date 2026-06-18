package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"task-queue/internal/api"
	"task-queue/internal/audit"
	"task-queue/internal/cache"
	"task-queue/internal/config"
	"task-queue/internal/database"
	"task-queue/internal/metrics"
	"task-queue/internal/models"
	"task-queue/internal/queue"
	"task-queue/internal/ratelimit"
	"task-queue/internal/repository"
	"task-queue/internal/retry"
	"task-queue/internal/tracing"
	"task-queue/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf("config loaded: postgres=%s:%d redis=%s:%d server_port=%d",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Redis.Host, cfg.Redis.Port, cfg.Server.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.New(&cfg.Postgres)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	log.Println("database migrated successfully")

	rdb, err := cache.New(&cfg.Redis)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()
	log.Println("redis connected successfully")

	taskRepo := repository.NewTaskRepository(db)
	workerRepo := repository.NewWorkerRepository(db)
	handlerRepo := repository.NewHandlerRepository(db)
	deadRepo := repository.NewDeadLetterRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	dagRepo := repository.NewDAGRepository(db)
	traceRepo := repository.NewTraceRepository(db)

	traceLogger := tracing.NewLogger(traceRepo)
	traceLogger.Start(ctx)

	taskRepo.SetTraceHook(func(taskID uuid.UUID, taskType string, fromStatus, toStatus models.TaskStatus, trigger string, workerID *uuid.UUID, errMsg string) {
		traceLogger.Record(taskID, taskType, fromStatus, toStatus, trigger, workerID, errMsg)
	})

	auditLogger := audit.NewLogger(auditRepo)
	auditLogger.Start(ctx)

	scheduler := queue.NewPriorityScheduler(&cfg.Queue, rdb, taskRepo)
	scheduler.SetAuditLogger(func(c context.Context, et string, eid uuid.UUID, a string) {
		auditLogger.LogC(c, et, eid, a)
	})

	delayScheduler := queue.NewDelayScheduler(&cfg.Queue, rdb, taskRepo, scheduler)
	delayScheduler.SetAuditLogger(func(c context.Context, et string, eid uuid.UUID, a string) {
		auditLogger.LogC(c, et, eid, a)
	})

	selfWorker := &models.Worker{
		ID:         uuid.New(),
		Name:       fmt.Sprintf("master-%s", uuid.New().String()[:8]),
		Hostname:   getHostname(),
		TotalSlots: cfg.Worker.DefaultSlots,
		Status:     models.WorkerStatusOnline,
	}
	if err := workerRepo.Register(ctx, selfWorker); err != nil {
		log.Fatalf("register self worker: %v", err)
	}

	workerManager := worker.NewManager(
		&cfg.Worker, &cfg.Queue, workerRepo, handlerRepo, taskRepo, selfWorker)
	workerManager.SetAuditLogger(func(c context.Context, et string, eid uuid.UUID, a string) {
		auditLogger.LogC(c, et, eid, a)
	})

	rateLimitConfigMgr := ratelimit.NewConfigManager(rdb)
	if err := rateLimitConfigMgr.LoadAllConfigs(ctx); err != nil {
		log.Printf("warning: failed to load rate limit configs: %v", err)
	}
	rateLimitConfigMgr.Start(ctx)

	waitQueue := ratelimit.NewWaitQueue()
	rateLimitStats := ratelimit.NewStatsCollector(rdb)
	rateLimitStats.Start(ctx)

	rateLimiter := ratelimit.NewRateLimiter(rdb, rateLimitConfigMgr, waitQueue, rateLimitStats)
	rateLimiter.Start(ctx)

	workerManager.SetRateLimiter(rateLimiter)

	metricsColl := metrics.NewCollector(rdb, workerRepo, taskRepo, deadRepo, scheduler)
	metricsColl.SetRateLimitStats(rateLimitStats)

	retryEngine := retry.NewEngine(taskRepo, deadRepo, delayScheduler, scheduler)

	dagEngine := retry.NewDAGEngine(taskRepo, dagRepo, scheduler, delayScheduler)
	dagEngine.SetAuditLogger(func(c context.Context, et string, eid uuid.UUID, a string) {
		auditLogger.LogC(c, et, eid, a)
	})

	workerManager.SetCallbacks(
		func(c context.Context, t *models.Task, e error, rc int) {
			retryEngine.HandleRetry(c, t, e, rc)
		},
		func(c context.Context, t *models.Task, es string) {
			retryEngine.HandleDeadLetter(c, t, es)
			dagEngine.HandleTaskComplete(c, t)
		},
	)

	metricsCallback := metricsCallbackFunc(metricsColl)
	setMetricsCallback(workerManager, metricsCallback, dagEngine)

	workerManager.RegisterHandler("__echo__", getLocalHandlerURL(cfg, "__echo__"))

	scheduler.Start(ctx, time.Duration(cfg.Scheduler.DispatchInterval)*time.Millisecond, cfg.Scheduler.BatchSize)
	delayScheduler.Start(ctx)
	workerManager.Start(ctx, scheduler.DispatchChan(), scheduler.PreemptChan())
	metricsColl.Start(ctx)

	reaper := worker.NewDeadLetterReaper(&cfg.Worker, workerRepo, taskRepo, deadRepo)
	reaper.SetAuditLogger(func(c context.Context, et string, eid uuid.UUID, a string) {
		auditLogger.LogC(c, et, eid, a)
	})
	reaper.Start(ctx)

	server := api.NewServer(
		cfg, taskRepo, workerRepo, handlerRepo, deadRepo, dagRepo, traceRepo,
		auditLogger, scheduler, delayScheduler, workerManager, retryEngine, dagEngine, metricsColl,
		rateLimitConfigMgr, rateLimiter, waitQueue)

	go registerInternalHandlers(cfg, handlerRepo, selfWorker)

	go startInternalHandlerServer(ctx, cfg, taskRepo, metricsCallback, workerManager)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("HTTP server starting on :%d", cfg.Server.Port)
		errCh <- server.Listen(ctx)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received signal: %s, shutting down...", sig)
		cancel()
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.Worker.GracefulShutdownTimeout)*time.Second)
	defer shutdownCancel()

	reaper.Stop()
	rateLimiter.Stop()
	rateLimitStats.Stop()
	rateLimitConfigMgr.Stop()
	metricsColl.Stop()
	workerManager.Stop(shutdownCtx)
	scheduler.Stop()
	delayScheduler.Stop()
	auditLogger.Stop()
	traceLogger.Stop()

	_ = server.Shutdown()
	log.Println("shutdown complete")
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func getLocalHandlerURL(cfg *config.Config, name string) string {
	return fmt.Sprintf("http://localhost:%d/internal/handler/%s", cfg.Server.Port, name)
}

func registerInternalHandlers(cfg *config.Config, hr *repository.HandlerRepository, w *models.Worker) {
	time.Sleep(500 * time.Millisecond)
	ctx := context.Background()
	for _, t := range []string{"__echo__"} {
		hr.Register(ctx, &models.HandlerRegistration{
			TaskType: t, HandlerID: fmt.Sprintf("%s-%s", w.ID, t),
			WorkerID: w.ID, Endpoint: getLocalHandlerURL(cfg, t),
		})
	}
}

func metricsCallbackFunc(coll *metrics.Collector) func(*models.Task, int64, bool) {
	return func(t *models.Task, d int64, s bool) {
		coll.RecordTaskComplete(t.Priority, d, s)
	}
}

func setMetricsCallback(wm *worker.Manager, fn func(*models.Task, int64, bool), de *retry.DAGEngine) {
	_ = fn
	_ = wm
	_ = de
}

func startInternalHandlerServer(ctx context.Context, cfg *config.Config, tr *repository.TaskRepository,
	mcb func(*models.Task, int64, bool), wm *worker.Manager) {
	_ = wm
	_ = mcb
	http.HandleFunc("/internal/handler/__echo__", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TaskID  uuid.UUID       `json:"task_id"`
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result": map[string]interface{}{
				"echoed":  true,
				"type":    req.Type,
				"payload": req.Payload,
			},
		})
	})
	addr := fmt.Sprintf(":%d", cfg.Server.Port+100)
	srv := &http.Server{Addr: addr, Handler: http.DefaultServeMux}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()
	srv.ListenAndServe()
}
