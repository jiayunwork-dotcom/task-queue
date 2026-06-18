package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"task-queue/internal/config"
)

type Database struct {
	Pool *pgxpool.Pool
}

func New(cfg *config.PostgresConfig) (*Database, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.PoolSize)
	poolCfg.MinConns = int32(cfg.PoolSize / 5)
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute
	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Database{Pool: pool}, nil
}

func (d *Database) Close() {
	d.Pool.Close()
}

func (d *Database) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			priority SMALLINT NOT NULL DEFAULT 2,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			delay_seconds INTEGER NOT NULL DEFAULT 0,
			scheduled_at TIMESTAMPTZ,
			max_retries INTEGER NOT NULL DEFAULT 3,
			retry_count INTEGER NOT NULL DEFAULT 0,
			timeout_seconds INTEGER NOT NULL DEFAULT 60,
			callback_url VARCHAR(1024),
			retry_mode VARCHAR(32) NOT NULL DEFAULT 'exponential',
			retry_interval INTEGER NOT NULL DEFAULT 10,
			retry_cron_expr VARCHAR(128),
			last_error TEXT,
			dag_id UUID,
			dag_node_id VARCHAR(255),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			handler_id VARCHAR(255),
			worker_id UUID,
			lease_expires_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_dag_id ON tasks(dag_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority)`,

		`CREATE TABLE IF NOT EXISTS task_executions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			attempt INTEGER NOT NULL DEFAULT 1,
			worker_id UUID NOT NULL,
			handler_id VARCHAR(255) NOT NULL,
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ended_at TIMESTAMPTZ,
			status VARCHAR(32) NOT NULL DEFAULT 'running',
			error TEXT,
			duration_ms BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_executions_task_id ON task_executions(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_executions_worker_id ON task_executions(worker_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions(started_at DESC)`,

		`CREATE TABLE IF NOT EXISTS workers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			hostname VARCHAR(255) NOT NULL,
			total_slots INTEGER NOT NULL DEFAULT 10,
			used_slots INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(32) NOT NULL DEFAULT 'offline',
			last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			tasks_completed BIGINT NOT NULL DEFAULT 0,
			tasks_failed BIGINT NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workers_status ON workers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_workers_last_heartbeat ON workers(last_heartbeat_at)`,

		`CREATE TABLE IF NOT EXISTS handler_registrations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_type VARCHAR(255) NOT NULL,
			handler_id VARCHAR(255) NOT NULL,
			worker_id UUID NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
			endpoint VARCHAR(1024) NOT NULL,
			registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(handler_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_handler_registrations_task_type ON handler_registrations(task_type)`,
		`CREATE INDEX IF NOT EXISTS idx_handler_registrations_worker_id ON handler_registrations(worker_id)`,

		`CREATE TABLE IF NOT EXISTS dag_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			nodes JSONB NOT NULL DEFAULT '[]'::jsonb,
			edges JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS dag_runs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			template_id UUID NOT NULL REFERENCES dag_templates(id) ON DELETE CASCADE,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			nodes_state JSONB NOT NULL DEFAULT '{}'::jsonb,
			strategy VARCHAR(32) NOT NULL DEFAULT 'abort',
			max_retries INTEGER NOT NULL DEFAULT 3,
			payload JSONB DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			ended_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dag_runs_status ON dag_runs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_dag_runs_template_id ON dag_runs(template_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dag_runs_created_at ON dag_runs(created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			entity_type VARCHAR(64) NOT NULL,
			entity_id UUID NOT NULL,
			action VARCHAR(64) NOT NULL,
			old_state JSONB,
			new_state JSONB,
			operator VARCHAR(255),
			remote_addr VARCHAR(64),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`,

		`CREATE TABLE IF NOT EXISTS dead_letter_tasks (
			task_id UUID PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			dead_letter_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			reason TEXT NOT NULL,
			error_history JSONB NOT NULL DEFAULT '[]'::jsonb
		)`,
		`CREATE INDEX IF NOT EXISTS idx_dead_letter_tasks_reason ON dead_letter_tasks(reason)`,
		`CREATE INDEX IF NOT EXISTS idx_dead_letter_tasks_at ON dead_letter_tasks(dead_letter_at DESC)`,

		`CREATE TABLE IF NOT EXISTS metrics_history (
			id BIGSERIAL PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			queue_depths JSONB NOT NULL DEFAULT '{}'::jsonb,
			throughput DOUBLE PRECISION NOT NULL DEFAULT 0,
			success_rates JSONB NOT NULL DEFAULT '{}'::jsonb,
			failure_rates JSONB NOT NULL DEFAULT '{}'::jsonb,
			avg_latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
			worker_utilization DOUBLE PRECISION NOT NULL DEFAULT 0,
			dead_letter_count BIGINT NOT NULL DEFAULT 0,
			workers_online INTEGER NOT NULL DEFAULT 0,
			workers_offline INTEGER NOT NULL DEFAULT 0,
			workers_total INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_history_timestamp ON metrics_history(timestamp DESC)`,

		`CREATE TABLE IF NOT EXISTS task_trace_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id UUID NOT NULL,
			task_type VARCHAR(255) NOT NULL,
			from_status VARCHAR(32) NOT NULL DEFAULT '',
			to_status VARCHAR(32) NOT NULL,
			trigger VARCHAR(128) NOT NULL DEFAULT '',
			worker_id UUID,
			error TEXT,
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_trace_events_task_id ON task_trace_events(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_task_trace_events_occurred_at ON task_trace_events(occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_task_trace_events_type_status_time ON task_trace_events(task_type, occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_task_trace_events_composite ON task_trace_events(occurred_at DESC, task_type) INCLUDE (task_id, to_status, from_status, trigger, worker_id, error)`,
	}

	for _, stmt := range statements {
		if _, err := d.Pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("execute migration %s: %w", stmt[:50], err)
		}
	}

	return nil
}
