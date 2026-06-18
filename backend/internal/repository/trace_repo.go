package repository

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"task-queue/internal/database"
	"task-queue/internal/models"
)

type TraceRepository struct {
	db *database.Database
}

func NewTraceRepository(db *database.Database) *TraceRepository {
	return &TraceRepository{db: db}
}

func (r *TraceRepository) InsertEvent(ctx context.Context, ev *models.TraceEvent) error {
	ev.ID = uuid.New()
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now()
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO task_trace_events (id, task_id, task_type, from_status, to_status, trigger, worker_id, error, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		ev.ID, ev.TaskID, ev.TaskType, ev.FromStatus, ev.ToStatus, ev.Trigger, ev.WorkerID, ev.Error, ev.OccurredAt,
	)
	return err
}

func (r *TraceRepository) BatchInsert(ctx context.Context, events []*models.TraceEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, ev := range events {
		if ev.ID == uuid.Nil {
			ev.ID = uuid.New()
		}
		if ev.OccurredAt.IsZero() {
			ev.OccurredAt = time.Now()
		}
		batch.Queue(
			`INSERT INTO task_trace_events (id, task_id, task_type, from_status, to_status, trigger, worker_id, error, occurred_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			ev.ID, ev.TaskID, ev.TaskType, ev.FromStatus, ev.ToStatus, ev.Trigger, ev.WorkerID, ev.Error, ev.OccurredAt,
		)
	}
	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range events {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

type TraceFilter struct {
	From        *time.Time
	To          *time.Time
	TaskType    string
	FinalStatus []models.TaskStatus
}

func (r *TraceRepository) ListTraces(ctx context.Context, filter TraceFilter, limit, offset int) ([]models.TraceSummary, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.From != nil {
		where += fmt.Sprintf(" AND first_occurrence >= $%d", argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		where += fmt.Sprintf(" AND first_occurrence <= $%d", argIdx)
		args = append(args, *filter.To)
		argIdx++
	}
	if filter.TaskType != "" {
		where += fmt.Sprintf(" AND task_type = $%d", argIdx)
		args = append(args, filter.TaskType)
		argIdx++
	}
	if len(filter.FinalStatus) > 0 {
		placeholders := ""
		for i, s := range filter.FinalStatus {
			if i > 0 {
				placeholders += ","
			}
			placeholders += fmt.Sprintf("$%d", argIdx)
			args = append(args, s)
			argIdx++
		}
		where += fmt.Sprintf(" AND final_status IN (%s)", placeholders)
	}

	taskIDs := `
		SELECT DISTINCT ON (task_id)
			task_id,
			task_type,
			FIRST_VALUE(to_status) OVER (PARTITION BY task_id ORDER BY occurred_at DESC) as final_status,
			MIN(occurred_at) OVER (PARTITION BY task_id) as first_occurrence,
			MAX(occurred_at) OVER (PARTITION BY task_id) as last_occurrence,
			COUNT(*) OVER (PARTITION BY task_id) as node_count
		FROM task_trace_events
	`

	countSql := fmt.Sprintf(`
		SELECT COUNT(DISTINCT task_id)
		FROM (%s) t
		%s
	`, taskIDs, where)
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSql, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSql := fmt.Sprintf(`
		SELECT task_id, task_type, final_status, first_occurrence, last_occurrence, node_count
		FROM (%s) t
		%s
		ORDER BY (EXTRACT(EPOCH FROM (last_occurrence - first_occurrence)) * 1000)::BIGINT DESC
		LIMIT $%d OFFSET $%d
	`, taskIDs, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listSql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	preSummaries := []struct {
		TaskID      uuid.UUID
		TaskType    string
		FinalStatus models.TaskStatus
		FirstOcc    time.Time
		LastOcc     time.Time
		NodeCount   int
	}{}
	for rows.Next() {
		var s struct {
			TaskID      uuid.UUID
			TaskType    string
			FinalStatus models.TaskStatus
			FirstOcc    time.Time
			LastOcc     time.Time
			NodeCount   int
		}
		if err := rows.Scan(&s.TaskID, &s.TaskType, &s.FinalStatus, &s.FirstOcc, &s.LastOcc, &s.NodeCount); err != nil {
			return nil, 0, err
		}
		preSummaries = append(preSummaries, s)
	}

	result := make([]models.TraceSummary, 0, len(preSummaries))
	for _, ps := range preSummaries {
		events, err := r.getEventsByTaskID(ctx, ps.TaskID)
		if err != nil {
			continue
		}
		summary := r.buildSummary(events, ps)
		result = append(result, summary)
	}

	return result, total, nil
}

func (r *TraceRepository) GetTraceDetail(ctx context.Context, taskID uuid.UUID) (*models.TraceDetail, error) {
	events, err := r.getEventsByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("trace not found for task %s", taskID.String())
	}

	pre := struct {
		TaskID      uuid.UUID
		TaskType    string
		FinalStatus models.TaskStatus
		FirstOcc    time.Time
		LastOcc     time.Time
		NodeCount   int
	}{
		TaskID:      events[0].TaskID,
		TaskType:    events[0].TaskType,
		FinalStatus: events[len(events)-1].ToStatus,
		FirstOcc:    events[0].OccurredAt,
		LastOcc:     events[len(events)-1].OccurredAt,
		NodeCount:   len(events),
	}

	summary := r.buildSummary(events, pre)
	intervals := r.buildIntervals(events)
	retryErrors := r.buildRetryErrors(events)

	return &models.TraceDetail{
		TaskID:          summary.TaskID,
		TaskType:        summary.TaskType,
		FinalStatus:     summary.FinalStatus,
		CreatedAt:       summary.CreatedAt,
		CompletedAt:     summary.CompletedAt,
		TotalDurationMs: summary.TotalDurationMs,
		QueueWaitMs:     summary.QueueWaitMs,
		ExecutionMs:     summary.ExecutionMs,
		RetryIntervalMs: summary.RetryIntervalMs,
		Events:          events,
		Intervals:       intervals,
		RetryErrors:     retryErrors,
	}, nil
}

func (r *TraceRepository) AnalyzeBottleneck(ctx context.Context, from, to time.Time, taskType string) (*models.BottleneckAnalysis, error) {
	events, err := r.getEventsInRangeByType(ctx, from, to, taskType)
	if err != nil {
		return nil, err
	}

	taskEvents := make(map[uuid.UUID][]models.TraceEvent)
	for _, ev := range events {
		taskEvents[ev.TaskID] = append(taskEvents[ev.TaskID], ev)
	}

	stageDurations := make(map[string][]int64)
	totalDurations := make([]int64, 0)

	for _, evs := range taskEvents {
		sort.Slice(evs, func(i, j int) bool { return evs[i].OccurredAt.Before(evs[j].OccurredAt) })
		if len(evs) < 2 {
			continue
		}

		var taskTotal int64
		for i := 1; i < len(evs); i++ {
			d := evs[i].OccurredAt.Sub(evs[i-1].OccurredAt).Milliseconds()
			if d < 0 {
				d = 0
			}
			stageName := r.classifyStage(evs[i-1].ToStatus, evs[i].ToStatus)
			stageDurations[stageName] = append(stageDurations[stageName], d)
			taskTotal += d
		}
		if taskTotal > 0 {
			totalDurations = append(totalDurations, taskTotal)
		}
	}

	stageStats := make(map[string]models.StageStats)
	var totalAvg float64
	if len(totalDurations) > 0 {
		var sum int64
		for _, d := range totalDurations {
			sum += d
		}
		totalAvg = float64(sum) / float64(len(totalDurations))
	}

	for stage, durations := range stageDurations {
		if len(durations) == 0 {
			continue
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		var sum int64
		for _, d := range durations {
			sum += d
		}
		avg := float64(sum) / float64(len(durations))
		pct := 0.0
		if totalAvg > 0 {
			pct = (avg / totalAvg) * 100
		}
		stageStats[stage] = models.StageStats{
			P50Ms: percentile(durations, 0.50),
			P90Ms: percentile(durations, 0.90),
			P99Ms: percentile(durations, 0.99),
			AvgMs: avg,
			Pct:   pct,
		}
	}

	var bottleneck *string
	var bottleneckPct float64
	for stage, stats := range stageStats {
		if stats.Pct > bottleneckPct {
			bottleneckPct = stats.Pct
			s := stage
			bottleneck = &s
		}
	}
	if bottleneckPct < 60 {
		bottleneck = nil
	}

	return &models.BottleneckAnalysis{
		TaskType:      taskType,
		TotalSamples:  int64(len(taskEvents)),
		TimeFrom:      from,
		TimeTo:        to,
		Stages:        stageStats,
		Bottleneck:    bottleneck,
		BottleneckPct: bottleneckPct,
	}, nil
}

func (r *TraceRepository) getEventsByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.TraceEvent, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, task_id, task_type, from_status, to_status, trigger, worker_id, error, occurred_at
		 FROM task_trace_events WHERE task_id = $1 ORDER BY occurred_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (r *TraceRepository) getEventsInRangeByType(ctx context.Context, from, to time.Time, taskType string) ([]models.TraceEvent, error) {
	var rows pgx.Rows
	var err error
	if taskType != "" {
		rows, err = r.db.Pool.Query(ctx,
			`SELECT id, task_id, task_type, from_status, to_status, trigger, worker_id, error, occurred_at
			 FROM task_trace_events WHERE occurred_at >= $1 AND occurred_at <= $2 AND task_type = $3
			 ORDER BY task_id, occurred_at ASC`,
			from, to, taskType)
	} else {
		rows, err = r.db.Pool.Query(ctx,
			`SELECT id, task_id, task_type, from_status, to_status, trigger, worker_id, error, occurred_at
			 FROM task_trace_events WHERE occurred_at >= $1 AND occurred_at <= $2
			 ORDER BY task_id, occurred_at ASC`,
			from, to)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows pgx.Rows) ([]models.TraceEvent, error) {
	events := []models.TraceEvent{}
	for rows.Next() {
		var ev models.TraceEvent
		if err := rows.Scan(&ev.ID, &ev.TaskID, &ev.TaskType, &ev.FromStatus, &ev.ToStatus,
			&ev.Trigger, &ev.WorkerID, &ev.Error, &ev.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

type preSummary struct {
	TaskID      uuid.UUID
	TaskType    string
	FinalStatus models.TaskStatus
	FirstOcc    time.Time
	LastOcc     time.Time
	NodeCount   int
}

func (r *TraceRepository) buildSummary(events []models.TraceEvent, ps preSummary) models.TraceSummary {
	var queueWait, execution, retryInterval int64
	for i := 1; i < len(events); i++ {
		d := events[i].OccurredAt.Sub(events[i-1].OccurredAt).Milliseconds()
		if d < 0 {
			d = 0
		}
		stage := r.classifyStage(events[i-1].ToStatus, events[i].ToStatus)
		switch stage {
		case "queue_wait":
			queueWait += d
		case "execution":
			execution += d
		case "retry_interval":
			retryInterval += d
		}
	}

	total := ps.LastOcc.Sub(ps.FirstOcc).Milliseconds()
	if total < 0 {
		total = 0
	}
	var completedAt *time.Time
	terminal := map[models.TaskStatus]bool{
		models.TaskStatusSuccess:    true,
		models.TaskStatusFailed:     true,
		models.TaskStatusDeadLetter: true,
		models.TaskStatusCancelled:  true,
	}
	if terminal[ps.FinalStatus] {
		t := ps.LastOcc
		completedAt = &t
	}

	return models.TraceSummary{
		TaskID:          ps.TaskID,
		TaskType:        ps.TaskType,
		FinalStatus:     ps.FinalStatus,
		CreatedAt:       ps.FirstOcc,
		CompletedAt:     completedAt,
		TotalDurationMs: total,
		QueueWaitMs:     queueWait,
		ExecutionMs:     execution,
		RetryIntervalMs: retryInterval,
		NodeCount:       ps.NodeCount,
	}
}

func (r *TraceRepository) buildIntervals(events []models.TraceEvent) []models.TraceInterval {
	intervals := make([]models.TraceInterval, 0)
	for i := 1; i < len(events); i++ {
		d := events[i].OccurredAt.Sub(events[i-1].OccurredAt).Milliseconds()
		if d < 0 {
			d = 0
		}
		intervals = append(intervals, models.TraceInterval{
			FromStatus: events[i-1].ToStatus,
			ToStatus:   events[i].ToStatus,
			DurationMs: d,
		})
	}
	return intervals
}

func (r *TraceRepository) buildRetryErrors(events []models.TraceEvent) []models.RetryError {
	errors := make([]models.RetryError, 0)
	attempt := 0
	for _, ev := range events {
		if ev.ToStatus == models.TaskStatusRunning {
			attempt++
		}
		if ev.Error != "" && (ev.ToStatus == models.TaskStatusFailed || ev.ToStatus == models.TaskStatusDeadLetter) {
			errors = append(errors, models.RetryError{
				Attempt:   attempt,
				Error:     ev.Error,
				Timestamp: ev.OccurredAt.Format(time.RFC3339),
			})
		}
	}
	return errors
}

func (r *TraceRepository) classifyStage(from, to models.TaskStatus) string {
	waitFrom := map[models.TaskStatus]bool{
		models.TaskStatusPending: true,
		models.TaskStatusDelayed: true,
		models.TaskStatusReady:   true,
	}
	execTo := map[models.TaskStatus]bool{
		models.TaskStatusSuccess:    true,
		models.TaskStatusFailed:     true,
		models.TaskStatusDeadLetter: true,
	}
	if waitFrom[from] && to == models.TaskStatusRunning {
		return "queue_wait"
	}
	if from == models.TaskStatusRunning && execTo[to] {
		return "execution"
	}
	if (from == models.TaskStatusFailed || from == models.TaskStatusRunning) &&
		(to == models.TaskStatusReady || to == models.TaskStatusDelayed || to == models.TaskStatusPending) {
		return "retry_interval"
	}
	return "other"
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	weight := idx - float64(lower)
	return int64(float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight)
}
