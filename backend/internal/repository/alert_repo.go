package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"task-queue/internal/database"
	"task-queue/internal/models"
)

type AlertRepository struct {
	db *database.Database
}

func NewAlertRepository(db *database.Database) *AlertRepository {
	return &AlertRepository{db: db}
}

type AlertRuleCreate struct {
	Name            string
	TaskType        *string
	ConditionType   models.AlertConditionType
	Threshold       float64
	WindowMinutes   int
	CooldownSeconds int
	NotifyType      models.AlertNotifyType
	WebhookURL      *string
	Enabled         bool
}

type AlertRuleUpdate struct {
	Name            *string
	TaskType        **string
	ConditionType   *models.AlertConditionType
	Threshold       *float64
	WindowMinutes   *int
	CooldownSeconds *int
	NotifyType      *models.AlertNotifyType
	WebhookURL      **string
	Enabled         *bool
}

type AlertHistoryFilter struct {
	From     *time.Time
	To       *time.Time
	RuleID   *uuid.UUID
	RuleName string
	Limit    int
	Offset   int
}

func (r *AlertRepository) CreateRule(ctx context.Context, c *AlertRuleCreate) (*models.AlertRule, error) {
	now := time.Now()
	rule := &models.AlertRule{
		ID:              uuid.New(),
		Name:            c.Name,
		TaskType:        c.TaskType,
		ConditionType:   c.ConditionType,
		Threshold:       c.Threshold,
		WindowMinutes:   c.WindowMinutes,
		CooldownSeconds: c.CooldownSeconds,
		NotifyType:      c.NotifyType,
		WebhookURL:      c.WebhookURL,
		Enabled:         c.Enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	taskTypeVal := pgtype.Text{}
	if c.TaskType != nil {
		taskTypeVal = pgtype.Text{String: *c.TaskType, Valid: true}
	}
	webhookVal := pgtype.Text{}
	if c.WebhookURL != nil {
		webhookVal = pgtype.Text{String: *c.WebhookURL, Valid: true}
	}

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alert_rules (id, name, task_type, condition_type, threshold, window_minutes,
			cooldown_seconds, notify_type, webhook_url, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		rule.ID, rule.Name, taskTypeVal, rule.ConditionType, rule.Threshold,
		rule.WindowMinutes, rule.CooldownSeconds, rule.NotifyType, webhookVal,
		rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *AlertRepository) GetRule(ctx context.Context, id uuid.UUID) (*models.AlertRule, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, task_type, condition_type, threshold, window_minutes,
			cooldown_seconds, notify_type, webhook_url, enabled, last_triggered_at,
			created_at, updated_at
		 FROM alert_rules WHERE id = $1`, id)
	var rule models.AlertRule
	err := row.Scan(&rule.ID, &rule.Name, &rule.TaskType, &rule.ConditionType,
		&rule.Threshold, &rule.WindowMinutes, &rule.CooldownSeconds, &rule.NotifyType,
		&rule.WebhookURL, &rule.Enabled, &rule.LastTriggeredAt, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *AlertRepository) ListRules(ctx context.Context) ([]models.AlertRule, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, task_type, condition_type, threshold, window_minutes,
			cooldown_seconds, notify_type, webhook_url, enabled, last_triggered_at,
			created_at, updated_at
		 FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []models.AlertRule{}
	for rows.Next() {
		var rule models.AlertRule
		err := rows.Scan(&rule.ID, &rule.Name, &rule.TaskType, &rule.ConditionType,
			&rule.Threshold, &rule.WindowMinutes, &rule.CooldownSeconds, &rule.NotifyType,
			&rule.WebhookURL, &rule.Enabled, &rule.LastTriggeredAt, &rule.CreatedAt, &rule.UpdatedAt)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AlertRepository) ListEnabledRules(ctx context.Context) ([]models.AlertRule, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, task_type, condition_type, threshold, window_minutes,
			cooldown_seconds, notify_type, webhook_url, enabled, last_triggered_at,
			created_at, updated_at
		 FROM alert_rules WHERE enabled = true ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []models.AlertRule{}
	for rows.Next() {
		var rule models.AlertRule
		err := rows.Scan(&rule.ID, &rule.Name, &rule.TaskType, &rule.ConditionType,
			&rule.Threshold, &rule.WindowMinutes, &rule.CooldownSeconds, &rule.NotifyType,
			&rule.WebhookURL, &rule.Enabled, &rule.LastTriggeredAt, &rule.CreatedAt, &rule.UpdatedAt)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AlertRepository) UpdateRule(ctx context.Context, id uuid.UUID, u *AlertRuleUpdate) (*models.AlertRule, error) {
	type column struct {
		expr string
		val  interface{}
	}
	cols := []column{{expr: "updated_at = NOW()", val: nil}}

	if u.Name != nil {
		cols = append(cols, column{expr: fmt.Sprintf("name = $%d", len(cols)+1), val: *u.Name})
	}
	if u.TaskType != nil {
		// outer ptr non-nil means caller wants to update this col
		t := pgtype.Text{}
		if *u.TaskType != nil {
			t = pgtype.Text{String: **u.TaskType, Valid: true}
		}
		cols = append(cols, column{expr: fmt.Sprintf("task_type = $%d", len(cols)+1), val: t})
	}
	if u.ConditionType != nil {
		cols = append(cols, column{expr: fmt.Sprintf("condition_type = $%d", len(cols)+1), val: string(*u.ConditionType)})
	}
	if u.Threshold != nil {
		cols = append(cols, column{expr: fmt.Sprintf("threshold = $%d", len(cols)+1), val: *u.Threshold})
	}
	if u.WindowMinutes != nil {
		cols = append(cols, column{expr: fmt.Sprintf("window_minutes = $%d", len(cols)+1), val: *u.WindowMinutes})
	}
	if u.CooldownSeconds != nil {
		cols = append(cols, column{expr: fmt.Sprintf("cooldown_seconds = $%d", len(cols)+1), val: *u.CooldownSeconds})
	}
	if u.NotifyType != nil {
		cols = append(cols, column{expr: fmt.Sprintf("notify_type = $%d", len(cols)+1), val: string(*u.NotifyType)})
	}
	if u.WebhookURL != nil {
		t := pgtype.Text{}
		if *u.WebhookURL != nil {
			t = pgtype.Text{String: **u.WebhookURL, Valid: true}
		}
		cols = append(cols, column{expr: fmt.Sprintf("webhook_url = $%d", len(cols)+1), val: t})
	}
	if u.Enabled != nil {
		cols = append(cols, column{expr: fmt.Sprintf("enabled = $%d", len(cols)+1), val: *u.Enabled})
	}

	setClause := ""
	args := []interface{}{}
	for i, c := range cols {
		if i > 0 {
			setClause += ", "
		}
		setClause += c.expr
		if c.val != nil {
			args = append(args, c.val)
		}
	}
	args = append(args, id)
	query := fmt.Sprintf(`UPDATE alert_rules SET %s WHERE id = $%d`, setClause, len(args))

	_, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return r.GetRule(ctx, id)
}

func (r *AlertRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (r *AlertRepository) UpdateLastTriggered(ctx context.Context, id uuid.UUID, t time.Time) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE alert_rules SET last_triggered_at = $1, updated_at = NOW() WHERE id = $2`,
		t, id)
	return err
}

func (r *AlertRepository) InsertHistory(ctx context.Context, h *models.AlertHistory) error {
	h.ID = uuid.New()
	if h.TriggeredAt.IsZero() {
		h.TriggeredAt = time.Now()
	}

	taskTypeVal := pgtype.Text{}
	if h.TaskType != nil {
		taskTypeVal = pgtype.Text{String: *h.TaskType, Valid: true}
	}
	webhookVal := pgtype.Text{}
	if h.WebhookURL != nil {
		webhookVal = pgtype.Text{String: *h.WebhookURL, Valid: true}
	}
	webhookErrVal := pgtype.Text{}
	if h.WebhookError != nil {
		webhookErrVal = pgtype.Text{String: *h.WebhookError, Valid: true}
	}

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alert_history (id, rule_id, rule_name, task_type, condition_type,
			actual_value, threshold_value, comparison_description, webhook_url,
			webhook_success, webhook_error, triggered_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		h.ID, h.RuleID, h.RuleName, taskTypeVal, h.ConditionType,
		h.ActualValue, h.ThresholdValue, h.ComparisonDescription, webhookVal,
		h.WebhookSuccess, webhookErrVal, h.TriggeredAt,
	)
	return err
}

func (r *AlertRepository) ListHistory(ctx context.Context, f AlertHistoryFilter) ([]models.AlertHistory, int64, error) {
	where := `WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if f.From != nil {
		where += fmt.Sprintf(" AND triggered_at >= $%d", argIdx)
		args = append(args, *f.From)
		argIdx++
	}
	if f.To != nil {
		where += fmt.Sprintf(" AND triggered_at <= $%d", argIdx)
		args = append(args, *f.To)
		argIdx++
	}
	if f.RuleID != nil {
		where += fmt.Sprintf(" AND rule_id = $%d", argIdx)
		args = append(args, *f.RuleID)
		argIdx++
	}
	if f.RuleName != "" {
		where += fmt.Sprintf(" AND rule_name ILIKE $%d", argIdx)
		args = append(args, "%"+f.RuleName+"%")
		argIdx++
	}

	countSql := fmt.Sprintf(`SELECT COUNT(*) FROM alert_history %s`, where)
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
		SELECT id, rule_id, rule_name, task_type, condition_type,
			actual_value, threshold_value, comparison_description, webhook_url,
			webhook_success, webhook_error, triggered_at
		FROM alert_history %s
		ORDER BY triggered_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listSql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	history := []models.AlertHistory{}
	for rows.Next() {
		var h models.AlertHistory
		err := rows.Scan(&h.ID, &h.RuleID, &h.RuleName, &h.TaskType, &h.ConditionType,
			&h.ActualValue, &h.ThresholdValue, &h.ComparisonDescription, &h.WebhookURL,
			&h.WebhookSuccess, &h.WebhookError, &h.TriggeredAt)
		if err != nil {
			return nil, 0, err
		}
		history = append(history, h)
	}
	return history, total, nil
}
