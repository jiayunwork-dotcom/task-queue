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
	db         *database.Database
	traceHook  func(taskID uuid.UUID, taskType string, fromStatus, toStatus models.TaskStatus, trigger string, workerID *uuid.UUID, errMsg string)
}

func NewTaskRepository(db *database.Database) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) SetTraceHook(hook func(taskID uuid.UUID, taskType string, fromStatus, toStatus models.TaskStatus, trigger string, workerID *uuid.UUID, errMsg string)) {
	r.traceHook = hook
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
	if err != nil {
		return err
	}

	if r.traceHook != nil {
		r.traceHook(task.ID, task.Type, "", task.Status, "task_created", nil, "")
	}
	return nil
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

func (r *TaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus, extra ...interface{}) (models.TaskStatus, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var currentStatus models.TaskStatus
	var taskType string
	var workerID *uuid.UUID
	var lastError string
	err = tx.QueryRow(ctx, `SELECT status, type, worker_id, COALESCE(last_error, '') FROM tasks WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus, &taskType, &workerID, &lastError)
	if err != nil {
		return "", err
	}

	if currentStatus == status {
		_ = tx.Commit(ctx)
		return currentStatus, nil
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
			if field == "worker_id" {
				if wid, ok := extra[i+1].(uuid.UUID); ok {
					cp := wid
					workerID = &cp
				} else if widPtr, ok := extra[i+1].(*uuid.UUID); ok {
					workerID = widPtr
				}
			}
			if field == "last_error" {
				if e, ok := extra[i+1].(string); ok {
					lastError = e
				}
			}
		}
	}
	sql += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	if r.traceHook != nil {
		var errMsg string
		if status == models.TaskStatusFailed || status == models.TaskStatusDeadLetter {
			errMsg = lastError
		}
		r.traceHook(id, taskType, currentStatus, status, "status_change", workerID, errMsg)
	}

	return currentStatus, nil
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

func (r *TaskRepository) GetDurationHeatmap(ctx context.Context, days int, taskType string) (*models.DurationHeatmapData, error) {
	data, err := r.getDurationHeatmapForPeriod(ctx, days, 0, taskType)
	if err != nil {
		return nil, err
	}
	detectAnomalies(data)
	return data, nil
}

func (r *TaskRepository) GetDurationHeatmapCompare(ctx context.Context, days int, taskType string) (*models.DurationHeatmapCompareData, error) {
	if days <= 0 {
		days = 7
	}

	current, err := r.getDurationHeatmapForPeriod(ctx, days, 0, taskType)
	if err != nil {
		return nil, err
	}

	previous, err := r.getDurationHeatmapForPeriod(ctx, days, days, taskType)
	if err != nil {
		return nil, err
	}

	detectAnomalies(current)
	detectAnomalies(previous)

	return &models.DurationHeatmapCompareData{
		Current:  current,
		Previous: previous,
	}, nil
}

func (r *TaskRepository) getDurationHeatmapForPeriod(ctx context.Context, days int, offsetDays int, taskType string) (*models.DurationHeatmapData, error) {
	if days <= 0 {
		days = 7
	}

	where := `WHERE ended_at IS NOT NULL AND status IN ('success', 'failed')
		AND ended_at >= NOW() - ($1 + $2) * INTERVAL '1 day'
		AND ended_at < NOW() - $2 * INTERVAL '1 day'`
	args := []interface{}{days, offsetDays}
	argIdx := 3

	if taskType != "" {
		where += fmt.Sprintf(" AND task_id IN (SELECT id FROM tasks WHERE type = $%d)", argIdx)
		args = append(args, taskType)
		argIdx++
	}

	sql := fmt.Sprintf(`
		SELECT
			EXTRACT(HOUR FROM ended_at)::INTEGER as hour,
			DATE(ended_at) as date,
			COUNT(*) as sample_size,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_ms) as p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) as p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_ms) as p99
		FROM task_executions
		%s
		GROUP BY DATE(ended_at), EXTRACT(HOUR FROM ended_at)
		ORDER BY date, hour
	`, where)

	rows, err := r.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawCell struct {
		Hour       int
		Date       time.Time
		SampleSize int64
		P50        float64
		P95        float64
		P99        float64
	}

	cells := []rawCell{}
	for rows.Next() {
		var c rawCell
		if err := rows.Scan(&c.Hour, &c.Date, &c.SampleSize, &c.P50, &c.P95, &c.P99); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}

	now := time.Now()
	refDate := now.AddDate(0, 0, -offsetDays)
	dates := make([]string, days)
	for i := 0; i < days; i++ {
		d := refDate.AddDate(0, 0, -days+1+i)
		dates[i] = d.Format("2006-01-02")
	}

	hours := make([]int, 24)
	for i := 0; i < 24; i++ {
		hours[i] = i
	}

	dateIdx := make(map[string]int)
	for i, d := range dates {
		dateIdx[d] = i
	}

	matrix := make([][]*models.DurationHeatmapCell, 24)
	for h := 0; h < 24; h++ {
		matrix[h] = make([]*models.DurationHeatmapCell, days)
	}

	for _, c := range cells {
		dateStr := c.Date.Format("2006-01-02")
		di, ok := dateIdx[dateStr]
		if !ok {
			continue
		}
		if c.Hour < 0 || c.Hour > 23 {
			continue
		}
		matrix[c.Hour][di] = &models.DurationHeatmapCell{
			Hour:       c.Hour,
			Date:       dateStr,
			P50Ms:      int64(c.P50),
			P95Ms:      int64(c.P95),
			P99Ms:      int64(c.P99),
			SampleSize: c.SampleSize,
			IsAnomaly:  false,
		}
	}

	return &models.DurationHeatmapData{
		TaskType: taskType,
		Dates:    dates,
		Hours:    hours,
		Matrix:   matrix,
	}, nil
}

func detectAnomalies(data *models.DurationHeatmapData) {
	if data == nil || data.Matrix == nil {
		return
	}

	days := len(data.Dates)
	hours := len(data.Hours)
	if days == 0 || hours == 0 {
		return
	}

	for di := 0; di < days; di++ {
		for h := 0; h < hours; h++ {
			cell := data.Matrix[h][di]
			if cell == nil {
				continue
			}

			cellP95 := float64(cell.P95Ms)

			dayAvgP95 := 0.0
			dayCount := 0
			for oh := 0; oh < hours; oh++ {
				if oh == h {
					continue
				}
				otherCell := data.Matrix[oh][di]
				if otherCell != nil {
					dayAvgP95 += float64(otherCell.P95Ms)
					dayCount++
				}
			}
			dayAnomaly := false
			if dayCount > 0 {
				dayAvgP95 /= float64(dayCount)
				if dayAvgP95 > 0 && cellP95 > dayAvgP95*3 {
					dayAnomaly = true
				}
			}

			hourAvgP95 := 0.0
			hourCount := 0
			for odi := 0; odi < days; odi++ {
				if odi == di {
					continue
				}
				otherCell := data.Matrix[h][odi]
				if otherCell != nil {
					hourAvgP95 += float64(otherCell.P95Ms)
					hourCount++
				}
			}
			hourAnomaly := false
			if hourCount > 0 {
				hourAvgP95 /= float64(hourCount)
				if hourAvgP95 > 0 && cellP95 > hourAvgP95*3 {
					hourAnomaly = true
				}
			}

			cell.IsAnomaly = dayAnomaly || hourAnomaly
		}
	}
}

func (r *TaskRepository) GetDurationHistogram(ctx context.Context, timeFrom, timeTo time.Time, taskType string) (*models.DurationHistogramData, error) {
	where := `WHERE ended_at IS NOT NULL AND status IN ('success', 'failed')
		AND ended_at >= $1 AND ended_at <= $2`
	args := []interface{}{timeFrom, timeTo}
	argIdx := 3

	if taskType != "" {
		where += fmt.Sprintf(" AND task_id IN (SELECT id FROM tasks WHERE type = $%d)", argIdx)
		args = append(args, taskType)
		argIdx++
	}

	buckets := []struct {
		Range      string
		RangeStart int64
		RangeEnd   *int64
	}{
		{Range: "0-100ms", RangeStart: 0, RangeEnd: int64Ptr(100)},
		{Range: "100-500ms", RangeStart: 100, RangeEnd: int64Ptr(500)},
		{Range: "500-1000ms", RangeStart: 500, RangeEnd: int64Ptr(1000)},
		{Range: "1-5s", RangeStart: 1000, RangeEnd: int64Ptr(5000)},
		{Range: "5-10s", RangeStart: 5000, RangeEnd: int64Ptr(10000)},
		{Range: "10s以上", RangeStart: 10000, RangeEnd: nil},
	}

	var totalCount int64
	countSql := fmt.Sprintf(`SELECT COUNT(*) FROM task_executions %s`, where)
	if err := r.db.Pool.QueryRow(ctx, countSql, args...).Scan(&totalCount); err != nil {
		return nil, err
	}

	histogramBuckets := make([]models.DurationHistogramBucket, 0, len(buckets))
	for _, b := range buckets {
		var count int64
		var bucketSql string
		if b.RangeEnd != nil {
			bucketSql = fmt.Sprintf(`
				SELECT COUNT(*) FROM task_executions
				%s AND duration_ms >= $%d AND duration_ms < $%d
			`, where, argIdx, argIdx+1)
			bucketArgs := make([]interface{}, len(args))
			copy(bucketArgs, args)
			bucketArgs = append(bucketArgs, b.RangeStart, *b.RangeEnd)
			if err := r.db.Pool.QueryRow(ctx, bucketSql, bucketArgs...).Scan(&count); err != nil {
				return nil, err
			}
		} else {
			bucketSql = fmt.Sprintf(`
				SELECT COUNT(*) FROM task_executions
				%s AND duration_ms >= $%d
			`, where, argIdx)
			bucketArgs := make([]interface{}, len(args))
			copy(bucketArgs, args)
			bucketArgs = append(bucketArgs, b.RangeStart)
			if err := r.db.Pool.QueryRow(ctx, bucketSql, bucketArgs...).Scan(&count); err != nil {
				return nil, err
			}
		}

		percentage := 0.0
		if totalCount > 0 {
			percentage = float64(count) / float64(totalCount) * 100
		}

		histogramBuckets = append(histogramBuckets, models.DurationHistogramBucket{
			Range:      b.Range,
			RangeStart: b.RangeStart,
			RangeEnd:   b.RangeEnd,
			Count:      count,
			Percentage: percentage,
		})
	}

	var avgMs float64
	var p50Ms, p90Ms, p95Ms, p99Ms int64

	if totalCount > 0 {
		statsSql := fmt.Sprintf(`
			SELECT
				AVG(duration_ms) as avg,
				PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_ms) as p50,
				PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY duration_ms) as p90,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) as p95,
				PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_ms) as p99
			FROM task_executions
			%s
		`, where)

		var p50, p90, p95, p99 float64
		if err := r.db.Pool.QueryRow(ctx, statsSql, args...).Scan(&avgMs, &p50, &p90, &p95, &p99); err != nil {
			return nil, err
		}
		p50Ms = int64(p50)
		p90Ms = int64(p90)
		p95Ms = int64(p95)
		p99Ms = int64(p99)
	}

	return &models.DurationHistogramData{
		TaskType:   taskType,
		TimeFrom:   timeFrom,
		TimeTo:     timeTo,
		TotalCount: totalCount,
		Buckets:    histogramBuckets,
		AvgMs:      avgMs,
		P50Ms:      p50Ms,
		P90Ms:      p90Ms,
		P95Ms:      p95Ms,
		P99Ms:      p99Ms,
	}, nil
}

func (r *TaskRepository) GetDurationHistogramCompare(ctx context.Context, firstFrom, firstTo, secondFrom, secondTo time.Time, taskType string) (*models.DurationHistogramCompareData, error) {
	first, err := r.GetDurationHistogram(ctx, firstFrom, firstTo, taskType)
	if err != nil {
		return nil, err
	}

	second, err := r.GetDurationHistogram(ctx, secondFrom, secondTo, taskType)
	if err != nil {
		return nil, err
	}

	return &models.DurationHistogramCompareData{
		First:  first,
		Second: second,
	}, nil
}

func int64Ptr(v int64) *int64 {
	return &v
}
