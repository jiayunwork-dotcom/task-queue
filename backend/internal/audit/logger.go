package audit

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"task-queue/internal/models"
	"task-queue/internal/repository"
)

type Logger struct {
	repo   *repository.AuditRepository
	buffer []*models.AuditLog
	mu     sync.Mutex
	wg     sync.WaitGroup
	stopCh chan struct{}
	flushCh chan struct{}
}

func NewLogger(repo *repository.AuditRepository) *Logger {
	return &Logger{
		repo:   repo,
		buffer: make([]*models.AuditLog, 0, 1000),
		stopCh: make(chan struct{}),
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

func (l *Logger) Log(entityType string, entityID uuid.UUID, action string, opts ...LogOption) {
	log := &models.AuditLog{
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
	}
	for _, opt := range opts {
		opt(log)
	}
	l.mu.Lock()
	l.buffer = append(l.buffer, log)
	if len(l.buffer) >= 500 {
		select {
		case l.flushCh <- struct{}{}:
		default:
		}
	}
	l.mu.Unlock()
}

func (l *Logger) LogC(ctx context.Context, entityType string, entityID uuid.UUID, action string, opts ...LogOption) {
	log := &models.AuditLog{
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
	}
	for _, opt := range opts {
		opt(log)
	}
	_ = l.repo.Log(ctx, log)
}

type LogOption func(*models.AuditLog)

func WithOldState(v interface{}) LogOption {
	return func(l *models.AuditLog) {
		if b, err := json.Marshal(v); err == nil {
			l.OldState = b
		}
	}
}

func WithNewState(v interface{}) LogOption {
	return func(l *models.AuditLog) {
		if b, err := json.Marshal(v); err == nil {
			l.NewState = b
		}
	}
}

func WithOperator(op string) LogOption {
	return func(l *models.AuditLog) {
		l.Operator = op
	}
}

func WithRemoteAddr(addr string) LogOption {
	return func(l *models.AuditLog) {
		l.RemoteAddr = addr
	}
}

func (l *Logger) flushLoop(ctx context.Context) {
	defer l.wg.Done()
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
	l.buffer = make([]*models.AuditLog, 0, 1000)
	l.mu.Unlock()

	for _, log := range batch {
		_ = l.repo.Log(ctx, log)
	}
}
