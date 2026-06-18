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

type WorkerRepository struct {
	db *database.Database
}

func NewWorkerRepository(db *database.Database) *WorkerRepository {
	return &WorkerRepository{db: db}
}

func (r *WorkerRepository) Register(ctx context.Context, w *models.Worker) error {
	now := time.Now()
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	w.RegisteredAt = now
	w.LastHeartbeatAt = now
	if w.Status == "" {
		w.Status = models.WorkerStatusOnline
	}

	sql := `INSERT INTO workers (id, name, hostname, total_slots, used_slots, status, last_heartbeat_at, registered_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			hostname = EXCLUDED.hostname,
			total_slots = EXCLUDED.total_slots,
			status = EXCLUDED.status,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at
		RETURNING id, tasks_completed, tasks_failed, registered_at`

	var completed, failed int64
	err := r.db.Pool.QueryRow(ctx, sql,
		w.ID, w.Name, w.Hostname, w.TotalSlots, w.UsedSlots, w.Status,
		w.LastHeartbeatAt, w.RegisteredAt).Scan(&w.ID, &completed, &failed, &w.RegisteredAt)
	if err != nil {
		return err
	}
	w.TasksCompleted = completed
	w.TasksFailed = failed
	return nil
}

func (r *WorkerRepository) Heartbeat(ctx context.Context, id uuid.UUID, usedSlots int) error {
	now := time.Now()
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE workers SET last_heartbeat_at = $1, used_slots = $2 WHERE id = $3`,
		now, usedSlots, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("worker not found")
	}
	return nil
}

func (r *WorkerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.WorkerStatus) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE workers SET status = $1, last_heartbeat_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (r *WorkerRepository) IncrementStats(ctx context.Context, id uuid.UUID, success, failed int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE workers SET tasks_completed = tasks_completed + $1, tasks_failed = tasks_failed + $2 WHERE id = $3`,
		success, failed, id)
	return err
}

func (r *WorkerRepository) FindTimedOut(ctx context.Context, timeout time.Duration) ([]models.Worker, error) {
	cutoff := time.Now().Add(-timeout)
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, hostname, total_slots, used_slots, status, last_heartbeat_at, registered_at,
			tasks_completed, tasks_failed
		 FROM workers WHERE status != 'offline' AND last_heartbeat_at < $1`,
		cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workers := []models.Worker{}
	for rows.Next() {
		var w models.Worker
		err := rows.Scan(&w.ID, &w.Name, &w.Hostname, &w.TotalSlots, &w.UsedSlots,
			&w.Status, &w.LastHeartbeatAt, &w.RegisteredAt, &w.TasksCompleted, &w.TasksFailed)
		if err != nil {
			return nil, err
		}
		workers = append(workers, w)
	}
	return workers, nil
}

func (r *WorkerRepository) List(ctx context.Context) ([]models.Worker, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, hostname, total_slots, used_slots, status, last_heartbeat_at, registered_at,
			tasks_completed, tasks_failed
		 FROM workers ORDER BY registered_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workers := []models.Worker{}
	for rows.Next() {
		var w models.Worker
		err := rows.Scan(&w.ID, &w.Name, &w.Hostname, &w.TotalSlots, &w.UsedSlots,
			&w.Status, &w.LastHeartbeatAt, &w.RegisteredAt, &w.TasksCompleted, &w.TasksFailed)
		if err != nil {
			return nil, err
		}
		workers = append(workers, w)
	}
	return workers, nil
}

func (r *WorkerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Worker, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, hostname, total_slots, used_slots, status, last_heartbeat_at, registered_at,
			tasks_completed, tasks_failed
		 FROM workers WHERE id = $1`, id)
	var w models.Worker
	err := row.Scan(&w.ID, &w.Name, &w.Hostname, &w.TotalSlots, &w.UsedSlots,
		&w.Status, &w.LastHeartbeatAt, &w.RegisteredAt, &w.TasksCompleted, &w.TasksFailed)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkerRepository) GetTaskIDsByWorker(ctx context.Context, workerID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM tasks WHERE worker_id = $1 AND status = 'running' ORDER BY started_at ASC`,
		workerID)
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

type HandlerRepository struct {
	db *database.Database
}

func NewHandlerRepository(db *database.Database) *HandlerRepository {
	return &HandlerRepository{db: db}
}

func (r *HandlerRepository) Register(ctx context.Context, h *models.HandlerRegistration) error {
	now := time.Now()
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	h.RegisteredAt = now
	sql := `INSERT INTO handler_registrations (id, task_type, handler_id, worker_id, endpoint, registered_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (handler_id) DO UPDATE SET
			task_type = EXCLUDED.task_type,
			worker_id = EXCLUDED.worker_id,
			endpoint = EXCLUDED.endpoint,
			registered_at = EXCLUDED.registered_at`
	_, err := r.db.Pool.Exec(ctx, sql, h.ID, h.TaskType, h.HandlerID, h.WorkerID, h.Endpoint, h.RegisteredAt)
	return err
}

func (r *HandlerRepository) FindByTaskType(ctx context.Context, taskType string) ([]models.HandlerRegistration, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT h.id, h.task_type, h.handler_id, h.worker_id, h.endpoint, h.registered_at
		 FROM handler_registrations h
		 JOIN workers w ON w.id = h.worker_id
		 WHERE h.task_type = $1 AND w.status = 'online'
		 ORDER BY h.registered_at ASC`,
		taskType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	handlers := []models.HandlerRegistration{}
	for rows.Next() {
		var h models.HandlerRegistration
		err := rows.Scan(&h.ID, &h.TaskType, &h.HandlerID, &h.WorkerID, &h.Endpoint, &h.RegisteredAt)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, h)
	}
	return handlers, nil
}

func (r *HandlerRepository) RemoveByWorker(ctx context.Context, workerID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM handler_registrations WHERE worker_id = $1`, workerID)
	return err
}

type DeadLetterRepository struct {
	db *database.Database
}

func NewDeadLetterRepository(db *database.Database) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

func (r *DeadLetterRepository) Add(ctx context.Context, taskID uuid.UUID, reason string, errorHistory []string) error {
	historyJSON, _ := json.Marshal(errorHistory)
	sql := `INSERT INTO dead_letter_tasks (task_id, dead_letter_at, reason, error_history)
		VALUES ($1, NOW(), $2, $3)
		ON CONFLICT (task_id) DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, sql, taskID, reason, historyJSON)
	return err
}

func (r *DeadLetterRepository) Remove(ctx context.Context, taskID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM dead_letter_tasks WHERE task_id = $1`, taskID)
	return err
}

func (r *DeadLetterRepository) List(ctx context.Context, limit, offset int) ([]models.Task, int64, error) {
	countSql := `SELECT COUNT(*) FROM dead_letter_tasks d JOIN tasks t ON t.id = d.task_id`
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSql).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSql := `SELECT t.id, t.type, t.payload, t.priority, t.status, t.delay_seconds, t.scheduled_at,
		t.max_retries, t.retry_count, t.timeout_seconds, t.callback_url, t.retry_mode, t.retry_interval,
		t.retry_cron_expr, t.last_error, t.dag_id, t.dag_node_id, t.created_at, t.updated_at,
		t.started_at, t.completed_at, t.handler_id, t.worker_id, t.lease_expires_at
		FROM dead_letter_tasks d JOIN tasks t ON t.id = d.task_id
		ORDER BY d.dead_letter_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Pool.Query(ctx, listSql, limit, offset)
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

func (r *DeadLetterRepository) GroupByError(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT reason, COUNT(*) FROM dead_letter_tasks GROUP BY reason ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int64{}
	for rows.Next() {
		var reason string
		var count int64
		if err := rows.Scan(&reason, &count); err != nil {
			return nil, err
		}
		result[reason] = count
	}
	return result, nil
}

func (r *DeadLetterRepository) GetErrorHistory(ctx context.Context, taskID uuid.UUID) ([]string, error) {
	var raw json.RawMessage
	err := r.db.Pool.QueryRow(ctx, `SELECT error_history FROM dead_letter_tasks WHERE task_id = $1`, taskID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var history []string
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func (r *DeadLetterRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM dead_letter_tasks`).Scan(&count)
	return count, err
}

type AuditRepository struct {
	db *database.Database
}

func NewAuditRepository(db *database.Database) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Log(ctx context.Context, log *models.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now()
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO audit_logs (id, entity_type, entity_id, action, old_state, new_state, operator, remote_addr, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		log.ID, log.EntityType, log.EntityID, log.Action, log.OldState, log.NewState,
		log.Operator, log.RemoteAddr, log.CreatedAt)
	return err
}

func (r *AuditRepository) TxLog(tx pgx.Tx, ctx context.Context, log *models.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now()
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (id, entity_type, entity_id, action, old_state, new_state, operator, remote_addr, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		log.ID, log.EntityType, log.EntityID, log.Action, log.OldState, log.NewState,
		log.Operator, log.RemoteAddr, log.CreatedAt)
	return err
}

type DAGRepository struct {
	db *database.Database
}

func NewDAGRepository(db *database.Database) *DAGRepository {
	return &DAGRepository{db: db}
}

func (r *DAGRepository) CreateTemplate(ctx context.Context, tpl *models.DAGTemplate) error {
	now := time.Now()
	if tpl.ID == uuid.Nil {
		tpl.ID = uuid.New()
	}
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	if tpl.Nodes == nil {
		tpl.Nodes = json.RawMessage(`[]`)
	}
	if tpl.Edges == nil {
		tpl.Edges = json.RawMessage(`[]`)
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO dag_templates (id, name, description, nodes, edges, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tpl.ID, tpl.Name, tpl.Description, tpl.Nodes, tpl.Edges, tpl.CreatedAt, tpl.UpdatedAt)
	return err
}

func (r *DAGRepository) ListTemplates(ctx context.Context) ([]models.DAGTemplate, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, description, nodes, edges, created_at, updated_at FROM dag_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := []models.DAGTemplate{}
	for rows.Next() {
		var t models.DAGTemplate
		err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Nodes, &t.Edges, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *DAGRepository) GetTemplate(ctx context.Context, id uuid.UUID) (*models.DAGTemplate, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, description, nodes, edges, created_at, updated_at FROM dag_templates WHERE id = $1`, id)
	var t models.DAGTemplate
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Nodes, &t.Edges, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *DAGRepository) CreateRun(ctx context.Context, run *models.DAGRun) error {
	now := time.Now()
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	run.CreatedAt = now
	run.UpdatedAt = now
	if run.Status == "" {
		run.Status = models.DAGStatusPending
	}
	if run.NodesState == nil {
		run.NodesState = json.RawMessage(`{}`)
	}
	if run.Payload == nil {
		run.Payload = json.RawMessage(`{}`)
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO dag_runs (id, template_id, status, nodes_state, strategy, max_retries, payload, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		run.ID, run.TemplateID, run.Status, run.NodesState, run.Strategy,
		run.MaxRetries, run.Payload, run.CreatedAt, run.UpdatedAt)
	return err
}

func (r *DAGRepository) UpdateRunStatus(ctx context.Context, id uuid.UUID, status models.DAGStatus, nodeState json.RawMessage) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE dag_runs SET status = $1, nodes_state = $2, updated_at = NOW(),
		 started_at = CASE WHEN started_at IS NULL AND $1 = 'running' THEN NOW() ELSE started_at END,
		 ended_at = CASE WHEN $1 IN ('success','failed','cancelled') AND ended_at IS NULL THEN NOW() ELSE ended_at END
		 WHERE id = $3`,
		status, nodeState, id)
	return err
}

func (r *DAGRepository) GetRun(ctx context.Context, id uuid.UUID) (*models.DAGRun, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, template_id, status, nodes_state, strategy, max_retries, payload, created_at, updated_at, started_at, ended_at
		 FROM dag_runs WHERE id = $1`, id)
	var r2 models.DAGRun
	err := row.Scan(&r2.ID, &r2.TemplateID, &r2.Status, &r2.NodesState, &r2.Strategy,
		&r2.MaxRetries, &r2.Payload, &r2.CreatedAt, &r2.UpdatedAt, &r2.StartedAt, &r2.EndedAt)
	if err != nil {
		return nil, err
	}
	return &r2, nil
}

func (r *DAGRepository) ListRuns(ctx context.Context, limit, offset int) ([]models.DAGRun, int64, error) {
	var total int64
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM dag_runs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, template_id, status, nodes_state, strategy, max_retries, payload, created_at, updated_at, started_at, ended_at
		 FROM dag_runs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	runs := []models.DAGRun{}
	for rows.Next() {
		var r2 models.DAGRun
		err := rows.Scan(&r2.ID, &r2.TemplateID, &r2.Status, &r2.NodesState, &r2.Strategy,
			&r2.MaxRetries, &r2.Payload, &r2.CreatedAt, &r2.UpdatedAt, &r2.StartedAt, &r2.EndedAt)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, r2)
	}
	return runs, total, nil
}

func (r *DAGRepository) UpdateTimeField(ctx context.Context, id uuid.UUID, field string, t time.Time) error {
	sql := fmt.Sprintf(`UPDATE dag_runs SET %s = $1, updated_at = NOW() WHERE id = $2`, field)
	_, err := r.db.Pool.Exec(ctx, sql, t, id)
	return err
}
