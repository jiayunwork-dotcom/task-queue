package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"task-queue/internal/database"
	"task-queue/internal/models"
)

type ScalingRepository struct {
	db *database.Database
}

func NewScalingRepository(db *database.Database) *ScalingRepository {
	return &ScalingRepository{db: db}
}

type ScalingPolicyCreate struct {
	TaskType              string
	TargetUtilizationPct  float64
	MinWorkers            int
	MaxWorkers            int
	CooldownSeconds       int
	ScaleInProtectionSecs int
	ScaleOutThreshold     int
	ScaleInThresholdPct   float64
	Enabled               bool
}

type ScalingPolicyUpdate struct {
	TargetUtilizationPct  *float64
	MinWorkers            *int
	MaxWorkers            *int
	CooldownSeconds       *int
	ScaleInProtectionSecs *int
	ScaleOutThreshold     *int
	ScaleInThresholdPct   *float64
	Enabled               *bool
}

type ScalingHistoryFilter struct {
	PolicyID *uuid.UUID
	TaskType string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

func (r *ScalingRepository) CreatePolicy(ctx context.Context, c *ScalingPolicyCreate) (*models.ScalingPolicy, error) {
	now := time.Now()
	policy := &models.ScalingPolicy{
		ID:                    uuid.New(),
		TaskType:              c.TaskType,
		TargetUtilizationPct:  c.TargetUtilizationPct,
		MinWorkers:            c.MinWorkers,
		MaxWorkers:            c.MaxWorkers,
		CooldownSeconds:       c.CooldownSeconds,
		ScaleInProtectionSecs: c.ScaleInProtectionSecs,
		ScaleOutThreshold:     c.ScaleOutThreshold,
		ScaleInThresholdPct:   c.ScaleInThresholdPct,
		Enabled:               c.Enabled,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO scaling_policies (
			id, task_type, target_utilization_pct, min_workers, max_workers,
			cooldown_seconds, scale_in_protection_secs, scale_out_threshold,
			scale_in_threshold_pct, enabled, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		policy.ID, policy.TaskType, policy.TargetUtilizationPct, policy.MinWorkers,
		policy.MaxWorkers, policy.CooldownSeconds, policy.ScaleInProtectionSecs,
		policy.ScaleOutThreshold, policy.ScaleInThresholdPct, policy.Enabled,
		policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (r *ScalingRepository) GetPolicy(ctx context.Context, id uuid.UUID) (*models.ScalingPolicy, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, task_type, target_utilization_pct, min_workers, max_workers,
			cooldown_seconds, scale_in_protection_secs, scale_out_threshold,
			scale_in_threshold_pct, enabled, last_operation_at, created_at, updated_at
		 FROM scaling_policies WHERE id = $1`, id)
	var policy models.ScalingPolicy
	err := row.Scan(&policy.ID, &policy.TaskType, &policy.TargetUtilizationPct,
		&policy.MinWorkers, &policy.MaxWorkers, &policy.CooldownSeconds,
		&policy.ScaleInProtectionSecs, &policy.ScaleOutThreshold,
		&policy.ScaleInThresholdPct, &policy.Enabled, &policy.LastOperationAt,
		&policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *ScalingRepository) GetPolicyByTaskType(ctx context.Context, taskType string) (*models.ScalingPolicy, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, task_type, target_utilization_pct, min_workers, max_workers,
			cooldown_seconds, scale_in_protection_secs, scale_out_threshold,
			scale_in_threshold_pct, enabled, last_operation_at, created_at, updated_at
		 FROM scaling_policies WHERE task_type = $1`, taskType)
	var policy models.ScalingPolicy
	err := row.Scan(&policy.ID, &policy.TaskType, &policy.TargetUtilizationPct,
		&policy.MinWorkers, &policy.MaxWorkers, &policy.CooldownSeconds,
		&policy.ScaleInProtectionSecs, &policy.ScaleOutThreshold,
		&policy.ScaleInThresholdPct, &policy.Enabled, &policy.LastOperationAt,
		&policy.CreatedAt, &policy.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *ScalingRepository) ListPolicies(ctx context.Context) ([]models.ScalingPolicy, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, task_type, target_utilization_pct, min_workers, max_workers,
			cooldown_seconds, scale_in_protection_secs, scale_out_threshold,
			scale_in_threshold_pct, enabled, last_operation_at, created_at, updated_at
		 FROM scaling_policies ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []models.ScalingPolicy{}
	for rows.Next() {
		var p models.ScalingPolicy
		err := rows.Scan(&p.ID, &p.TaskType, &p.TargetUtilizationPct,
			&p.MinWorkers, &p.MaxWorkers, &p.CooldownSeconds,
			&p.ScaleInProtectionSecs, &p.ScaleOutThreshold,
			&p.ScaleInThresholdPct, &p.Enabled, &p.LastOperationAt,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (r *ScalingRepository) ListEnabledPolicies(ctx context.Context) ([]models.ScalingPolicy, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, task_type, target_utilization_pct, min_workers, max_workers,
			cooldown_seconds, scale_in_protection_secs, scale_out_threshold,
			scale_in_threshold_pct, enabled, last_operation_at, created_at, updated_at
		 FROM scaling_policies WHERE enabled = true ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := []models.ScalingPolicy{}
	for rows.Next() {
		var p models.ScalingPolicy
		err := rows.Scan(&p.ID, &p.TaskType, &p.TargetUtilizationPct,
			&p.MinWorkers, &p.MaxWorkers, &p.CooldownSeconds,
			&p.ScaleInProtectionSecs, &p.ScaleOutThreshold,
			&p.ScaleInThresholdPct, &p.Enabled, &p.LastOperationAt,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (r *ScalingRepository) UpdatePolicy(ctx context.Context, id uuid.UUID, u *ScalingPolicyUpdate) (*models.ScalingPolicy, error) {
	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 0

	addSet := func(exprTemplate string, val interface{}) {
		argIdx++
		sets = append(sets, fmt.Sprintf(exprTemplate, argIdx))
		args = append(args, val)
	}

	if u.TargetUtilizationPct != nil {
		addSet("target_utilization_pct = $%d", *u.TargetUtilizationPct)
	}
	if u.MinWorkers != nil {
		addSet("min_workers = $%d", *u.MinWorkers)
	}
	if u.MaxWorkers != nil {
		addSet("max_workers = $%d", *u.MaxWorkers)
	}
	if u.CooldownSeconds != nil {
		addSet("cooldown_seconds = $%d", *u.CooldownSeconds)
	}
	if u.ScaleInProtectionSecs != nil {
		addSet("scale_in_protection_secs = $%d", *u.ScaleInProtectionSecs)
	}
	if u.ScaleOutThreshold != nil {
		addSet("scale_out_threshold = $%d", *u.ScaleOutThreshold)
	}
	if u.ScaleInThresholdPct != nil {
		addSet("scale_in_threshold_pct = $%d", *u.ScaleInThresholdPct)
	}
	if u.Enabled != nil {
		addSet("enabled = $%d", *u.Enabled)
	}

	argIdx++
	setClause := ""
	for i, s := range sets {
		if i > 0 {
			setClause += ", "
		}
		setClause += s
	}
	args = append(args, id)
	query := fmt.Sprintf(`UPDATE scaling_policies SET %s WHERE id = $%d`, setClause, argIdx)

	_, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return r.GetPolicy(ctx, id)
}

func (r *ScalingRepository) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM scaling_policies WHERE id = $1`, id)
	return err
}

func (r *ScalingRepository) UpdateLastOperation(ctx context.Context, id uuid.UUID, t time.Time) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE scaling_policies SET last_operation_at = $1, updated_at = NOW() WHERE id = $2`,
		t, id)
	return err
}

func (r *ScalingRepository) InsertHistory(ctx context.Context, h *models.ScalingHistory) error {
	h.ID = uuid.New()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO scaling_history (
			id, policy_id, task_type, operation_type, reason,
			suggested_count, snapshot_workers, snapshot_util_pct,
			snapshot_queue, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		h.ID, h.PolicyID, h.TaskType, h.OperationType, h.Reason,
		h.SuggestedCount, h.SnapshotWorkers, h.SnapshotUtilPct,
		h.SnapshotQueue, h.CreatedAt,
	)
	return err
}

func (r *ScalingRepository) ListHistory(ctx context.Context, f ScalingHistoryFilter) ([]models.ScalingHistory, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if f.PolicyID != nil {
		where += fmt.Sprintf(" AND policy_id = $%d", argIdx)
		args = append(args, *f.PolicyID)
		argIdx++
	}
	if f.TaskType != "" {
		where += fmt.Sprintf(" AND task_type = $%d", argIdx)
		args = append(args, f.TaskType)
		argIdx++
	}
	if f.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *f.From)
		argIdx++
	}
	if f.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *f.To)
		argIdx++
	}

	countSql := fmt.Sprintf(`SELECT COUNT(*) FROM scaling_history %s`, where)
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSql, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	listSql := fmt.Sprintf(`
		SELECT id, policy_id, task_type, operation_type, reason,
			suggested_count, snapshot_workers, snapshot_util_pct,
			snapshot_queue, created_at
		FROM scaling_history %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listSql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	history := []models.ScalingHistory{}
	for rows.Next() {
		var h models.ScalingHistory
		err := rows.Scan(&h.ID, &h.PolicyID, &h.TaskType, &h.OperationType,
			&h.Reason, &h.SuggestedCount, &h.SnapshotWorkers,
			&h.SnapshotUtilPct, &h.SnapshotQueue, &h.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		history = append(history, h)
	}
	return history, total, nil
}
