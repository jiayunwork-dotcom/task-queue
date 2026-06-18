package tracing

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"task-queue/internal/models"
	"task-queue/internal/repository"
)

type Logger struct {
	repo    *repository.TraceRepository
	buffer  []*models.TraceEvent
	mu      sync.Mutex
	wg      sync.WaitGroup
	stopCh  chan struct{}
	flushCh chan struct{}
}

func NewLogger(repo *repository.TraceRepository) *Logger {
	return &Logger{
		repo:    repo,
		buffer:  make([]*models.TraceEvent, 0, 2000),
		stopCh:  make(chan struct{}),
		flushCh: make(chan struct{}, 1),
	}
}

func (l *Logger) Start(ctx context.Context) {
	l.wg.Add(1)
	go l.flushLoop(ctx)
}

func (l *Logger) Stop() {
	close(l.stopCh)
	l.wg.Wait()
}

func (l *Logger) Record(taskID uuid.UUID, taskType string, fromStatus, toStatus models.TaskStatus, trigger string, workerID *uuid.UUID, errMsg string) {
	ev := &models.TraceEvent{
		TaskID:     taskID,
		TaskType:   taskType,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Trigger:    trigger,
		WorkerID:   workerID,
		Error:      errMsg,
		OccurredAt: time.Now(),
	}
	l.mu.Lock()
	l.buffer = append(l.buffer, ev)
	if len(l.buffer) >= 500 {
		select {
		case l.flushCh <- struct{}{}:
		default:
		}
	}
	l.mu.Unlock()
}

func (l *Logger) RecordWithTime(taskID uuid.UUID, taskType string, fromStatus, toStatus models.TaskStatus, trigger string, workerID *uuid.UUID, errMsg string, occurredAt time.Time) {
	ev := &models.TraceEvent{
		TaskID:     taskID,
		TaskType:   taskType,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Trigger:    trigger,
		WorkerID:   workerID,
		Error:      errMsg,
		OccurredAt: occurredAt,
	}
	l.mu.Lock()
	l.buffer = append(l.buffer, ev)
	if len(l.buffer) >= 500 {
		select {
		case l.flushCh <- struct{}{}:
		default:
		}
	}
	l.mu.Unlock()
}

func (l *Logger) flushLoop(ctx context.Context) {
	defer l.wg.Done()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.flushAll(ctx)
			return
		case <-l.stopCh:
			l.flushAll(ctx)
			return
		case <-l.flushCh:
			l.flushAll(ctx)
		case <-ticker.C:
			l.flushAll(ctx)
		}
	}
}

func (l *Logger) flushAll(ctx context.Context) {
	l.mu.Lock()
	if len(l.buffer) == 0 {
		l.mu.Unlock()
		return
	}
	batch := l.buffer
	l.buffer = make([]*models.TraceEvent, 0, 2000)
	l.mu.Unlock()

	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = l.repo.BatchInsert(writeCtx, batch)
}
