package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"task-queue/internal/database"
	"task-queue/internal/models"
)

type TaskRepository struct {
	db *database.Database
}

func NewTaskRepository(db *database.Database) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = models.TaskStatusPending
	}
	if task.MaxRetries < 0 {
		task.MaxRetries = 3
	}
	if task.TimeoutSeconds <= 0 {
		task.TimeoutSeconds = 60
	}
	if task.RetryMode == "" {
		task.RetryMode = models.RetryModeExponential
	}

	sql := `INSERT INTO tasks (id, type, payload, priority, status, delay_seconds, scheduled_at,
		max_retries, retry_count, timeout_seconds, callback_url, retry_mode, retry_interval,
		retry_cron_expr, dag_id, dag_node_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id`

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.Payload == nil {
		task.Payload = json.RawMessage(`{}`)
	}
	if task.ScheduledAt != nil && task.ScheduledAt.IsZero() {
		task.ScheduledAt = nil
	}

	_, err := r.db.Pool.Exec(ctx, sql,
		task.ID, task.Type, task.Payload, int(task.Priority), task.Status,
		task.DelaySeconds, task.ScheduledAt, task.MaxRetries, task.RetryCount,
		task.TimeoutSeconds, task.CallbackURL, task.RetryMode, task.RetryInterval,
		task.RetryCronExpr, task.DAGID, task.DAGNodeID, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	sql := `SELECT id, type, payload, priority, status, delay_seconds, scheduled_at,
		max_retries, retry_count, timeout_seconds, callback_url, retry_mode, retry_interval,
		retry_cron_expr, last_error, dag_id, dag_node_id, created_at, updated_at,
		started_at, completed_at, handler_id, worker_id, lease_expires_at
		FROM tasks WHERE id = $1`
	row := r.db.Pool.QueryRow(ctx, sql, id)
	var t models.Task
	var priorityInt int
	err := row.Scan(&t.ID, &t.Type, &t.Payload, &priorityInt, &t.Status,
		&t.DelaySeconds, &t.ScheduledAt, &t.MaxRetries, &t.RetryCount,
		&t.TimeoutSeconds, &t.CallbackURL, &t.RetryMode, &t.RetryInterval,
		&t.RetryCronExpr, &t.LastError, &t.DAGID, &t.DAGNodeID,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
		&t.HandlerID, &t.WorkerID, &t.LeaseExpiresAt)
	if err != nil {
		return nil, err
	}
	t.Priority = models.Priority(priorityInt)
	return &t, nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus, extra ...interface{}) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var currentStatus models.TaskStatus
	err = tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus)
	if err != nil {
		return err
	}

	now := time.Now()
	sql := `UPDATE tasks SET status = $1, updated_at = $2`
	args := []interface{}{status, now}
	argIdx := 3

	if status == models.TaskStatusRunning {
		sql += fmt.Sprintf(", started_at = $%d", argIdx)
		args = append(args, &now)
		argIdx++
	}
	if status == models.TaskStatusSuccess || status == models.TaskStatusFailed || status == models.TaskStatusDeadLetter {
		sql += fmt.Sprintf(", completed_at = $%d", argIdx)
		args = append(args, &now)
		argIdx++
	}
	if len(extra) > 0 {
		for i := 0; i < len(extra); i += 2 {
			field := extra[i].(string)
			sql += fmt.Sprintf(", %s = $%d", field, argIdx)
			args = append(args, extra[i+1])
			argIdx++
		}
	}
	sql += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *TaskRepository) LeaseTask(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, leaseTTL time.Duration) (bool, error) {
	expiresAt := time.Now().Add(leaseTTL)
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE tasks SET worker_id = $1, lease_expires_at = $2, updated_at = NOW()
		 WHERE id = $3 AND (lease_expires_at IS NULL OR lease_expires_at < NOW())`,
		workerID, expiresAt, taskID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func (r *TaskRepository) RenewLease(ctx context.Context, taskID uuid.UUID, workerID uuid.UUID, leaseTTL time.Duration) (bool, error) {
	expiresAt := time.Now().Add(leaseTTL)
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE tasks SET lease_expires_at = $1, updated_at = NOW()
		 WHERE id = $2 AND worker_id = $3 AND lease_expires_at > NOW()`,
		expiresAt, taskID, workerID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func (r *TaskRepository) IncrementRetry(ctx context.Context, id uuid.UUID, lastErr string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE tasks SET retry_count = retry_count + 1, last_error = $1, updated_at = NOW() WHERE id = $2`,
		lastErr, id)
	return err
}

func (r *TaskRepository) List(ctx context.Context, filter TaskFilter, limit, offset int) ([]models.Task, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Type != "" {
		where += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Priority != nil {
		where += fmt.Sprintf(" AND priority = $%d", argIdx)
		args = append(args, *filter.Priority)
		argIdx++
	}
	if filter.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filter.To)
		argIdx++
	}

	countSql := fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, where)
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSql, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSql := fmt.Sprintf(`SELECT id, type, payload, priority, status, delay_seconds, scheduled_at,
		max_retries, retry_count, timeout_seconds, callback_url, retry_mode, retry_interval,
		retry_cron_expr, last_error, dag_id, dag_node_id, created_at, updated_at,
		started_at, completed_at, handler_id, worker_id, lease_expires_at
		FROM tasks %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listSql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		var t models.Task
		var priorityInt int
		err := rows.Scan(&t.ID, &t.Type, &t.Payload, &priorityInt, &t.Status,
			&t.DelaySeconds, &t.ScheduledAt, &t.MaxRetries, &t.RetryCount,
			&t.TimeoutSeconds, &t.CallbackURL, &t.RetryMode, &t.RetryInterval,
			&t.RetryCronExpr, &t.LastError, &t.DAGID, &t.DAGNodeID,
			&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
			&t.HandlerID, &t.WorkerID, &t.LeaseExpiresAt)
		if err != nil {
			return nil, 0, err
		}
		t.Priority = models.Priority(priorityInt)
		tasks = append(tasks, t)
	}
	return tasks, total, nil
}

type TaskFilter struct {
	Status   models.TaskStatus
	Type     string
	Priority *models.Priority
	From     *time.Time
	To       *time.Time
}

func (r *TaskRepository) FindExpiredLeases(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM tasks WHERE status = 'running' AND lease_expires_at < NOW() LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *TaskRepository) CreateExecution(ctx context.Context, exec *models.TaskExecution) error {
	now := time.Now()
	exec.ID = uuid.New()
	exec.StartedAt = now
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO task_executions (id, task_id, attempt, worker_id, handler_id, started_at, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		exec.ID, exec.TaskID, exec.Attempt, exec.WorkerID, exec.HandlerID, exec.StartedAt, models.TaskStatusRunning)
	return err
}

func (r *TaskRepository) CompleteExecution(ctx context.Context, execID uuid.UUID, status models.TaskStatus, execErr string) error {
	endedAt := time.Now()
	var durationMs int64
	var startedAt time.Time
	err := r.db.Pool.QueryRow(ctx, `SELECT started_at FROM task_executions WHERE id = $1`, execID).Scan(&startedAt)
	if err == nil {
		durationMs = endedAt.Sub(startedAt).Milliseconds()
	}
	_, err = r.db.Pool.Exec(ctx,
		`UPDATE task_executions SET ended_at = $1, status = $2, error = $3, duration_ms = $4 WHERE id = $5`,
		endedAt, status, execErr, durationMs, execID)
	return err
}

func (r *TaskRepository) GetExecutions(ctx context.Context, taskID uuid.UUID) ([]models.TaskExecution, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, task_id, attempt, worker_id, handler_id, started_at, ended_at, status, error, duration_ms
		 FROM task_executions WHERE task_id = $1 ORDER BY started_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	execs := []models.TaskExecution{}
	for rows.Next() {
		var e models.TaskExecution
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Attempt, &e.WorkerID, &e.HandlerID,
			&e.StartedAt, &e.EndedAt, &e.Status, &e.Error, &e.DurationMS); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, nil
}

func (r *TaskRepository) FindReadyByPriority(ctx context.Context, priority models.Priority, limit int) ([]uuid.UUID, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM tasks WHERE status = 'ready' AND priority = $1 ORDER BY created_at ASC LIMIT $2`,
		int(priority), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *TaskRepository) FindExpiredDelays(ctx context.Context, limit int) ([]uuid.UUID, error) {
	now := time.Now()
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM tasks WHERE status = 'delayed' AND scheduled_at <= $1 ORDER BY scheduled_at ASC LIMIT $2`,
		now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *TaskRepository) Tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *TaskRepository) ExecHistory(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	res, err := r.db.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}
