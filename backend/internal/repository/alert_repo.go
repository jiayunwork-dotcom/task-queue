package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alert_rules (id, name, task_type, condition_type, threshold, window_minutes,
			cooldown_seconds, notify_type, webhook_url, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		rule.ID, rule.Name, rule.TaskType, rule.ConditionType, rule.Threshold,
		rule.WindowMinutes, rule.CooldownSeconds, rule.NotifyType, rule.WebhookURL,
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
	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if u.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx+1))
		args = append(args, *u.Name)
		argIdx++
	}
	if u.TaskType != nil {
		sets = append(sets, fmt.Sprintf("task_type = $%d", argIdx+1))
		args = append(args, *u.TaskType)
		argIdx++
	}
	if u.ConditionType != nil {
		sets = append(sets, fmt.Sprintf("condition_type = $%d", argIdx+1))
		args = append(args, *u.ConditionType)
		argIdx++
	}
	if u.Threshold != nil {
		sets = append(sets, fmt.Sprintf("threshold = $%d", argIdx+1))
		args = append(args, *u.Threshold)
		argIdx++
	}
	if u.WindowMinutes != nil {
		sets = append(sets, fmt.Sprintf("window_minutes = $%d", argIdx+1))
		args = append(args, *u.WindowMinutes)
		argIdx++
	}
	if u.CooldownSeconds != nil {
		sets = append(sets, fmt.Sprintf("cooldown_seconds = $%d", argIdx+1))
		args = append(args, *u.CooldownSeconds)
		argIdx++
	}
	if u.NotifyType != nil {
		sets = append(sets, fmt.Sprintf("notify_type = $%d", argIdx+1))
		args = append(args, *u.NotifyType)
		argIdx++
	}
	if u.WebhookURL != nil {
		sets = append(sets, fmt.Sprintf("webhook_url = $%d", argIdx+1))
		args = append(args, *u.WebhookURL)
		argIdx++
	}
	if u.Enabled != nil {
		sets = append(sets, fmt.Sprintf("enabled = $%d", argIdx+1))
		args = append(args, *u.Enabled)
		argIdx++
	}

	setClause := ""
	for i, s := range sets {
		if i > 0 {
			setClause += ", "
		}
		setClause += s
	}

	args = append(args, id)
	query := fmt.Sprintf(`UPDATE alert_rules SET %s WHERE id = $%d`, setClause, argIdx+1)

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
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alert_history (id, rule_id, rule_name, task_type, condition_type,
			actual_value, threshold_value, comparison_description, webhook_url,
			webhook_success, webhook_error, triggered_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		h.ID, h.RuleID, h.RuleName, h.TaskType, h.ConditionType,
		h.ActualValue, h.ThresholdValue, h.ComparisonDescription, h.WebhookURL,
		h.WebhookSuccess, h.WebhookError, h.TriggeredAt,
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
