package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"task-queue/internal/database"
	"task-queue/internal/models"
	"task-queue/internal/repository"
)

type QueueDepthProvider interface {
	GetQueueDepthsByType(ctx context.Context) (map[string]int64, error)
}

type Detector struct {
	alertRepo *repository.AlertRepository
	db        *database.Database
	queueProv QueueDepthProvider
	interval  time.Duration
	client    *http.Client
	stopCh    chan struct{}
	stopOnce  sync.Once
}

func NewDetector(
	alertRepo *repository.AlertRepository,
	db *database.Database,
	queueProv QueueDepthProvider,
) *Detector {
	return &Detector{
		alertRepo: alertRepo,
		db:        db,
		queueProv: queueProv,
		interval:  30 * time.Second,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

func (d *Detector) Start(ctx context.Context) {
	go d.run(ctx)
}

func (d *Detector) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
}

func (d *Detector) run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	log.Println("[alerts] detector started, interval:", d.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[alerts] detector stopped (context canceled)")
			return
		case <-d.stopCh:
			log.Println("[alerts] detector stopped")
			return
		case <-ticker.C:
			d.scanOnce(ctx)
		}
	}
}

func (d *Detector) scanOnce(ctx context.Context) {
	rules, err := d.alertRepo.ListEnabledRules(ctx)
	if err != nil {
		log.Printf("[alerts] list enabled rules error: %v", err)
		return
	}

	now := time.Now()
	for _, rule := range rules {
		if rule.LastTriggeredAt != nil {
			cooldownEnd := rule.LastTriggeredAt.Add(time.Duration(rule.CooldownSeconds) * time.Second)
			if now.Before(cooldownEnd) {
				continue
			}
		}
		d.evaluateRule(ctx, &rule, now)
	}
}

func (d *Detector) evaluateRule(ctx context.Context, rule *models.AlertRule, now time.Time) {
	var (
		actualValue float64
		desc        string
		triggered   bool
		err         error
	)

	switch rule.ConditionType {
	case models.AlertConditionDurationP95:
		actualValue, desc, triggered, err = d.evalDurationP95(ctx, rule)
	case models.AlertConditionFailureRate:
		actualValue, desc, triggered, err = d.evalFailureRate(ctx, rule)
	case models.AlertConditionQueueBacklog:
		actualValue, desc, triggered, err = d.evalQueueBacklog(ctx, rule)
	default:
		return
	}

	if err != nil {
		log.Printf("[alerts] evaluate rule %s (%s) error: %v", rule.ID, rule.Name, err)
		return
	}
	if !triggered {
		return
	}

	webhookSuccess := false
	var webhookErr *string

	if rule.WebhookURL != nil && *rule.WebhookURL != "" {
		if werr := d.sendWebhook(ctx, rule, actualValue, desc, now); werr != nil {
			msg := werr.Error()
			webhookErr = &msg
			log.Printf("[alerts] webhook error for rule %s: %v", rule.ID, werr)
		} else {
			webhookSuccess = true
		}
	}

	history := &models.AlertHistory{
		RuleID:                rule.ID,
		RuleName:              rule.Name,
		TaskType:              rule.TaskType,
		ConditionType:         rule.ConditionType,
		ActualValue:           actualValue,
		ThresholdValue:        rule.Threshold,
		ComparisonDescription: desc,
		WebhookURL:            rule.WebhookURL,
		WebhookSuccess:        webhookSuccess,
		WebhookError:          webhookErr,
		TriggeredAt:           now,
	}
	if herr := d.alertRepo.InsertHistory(ctx, history); herr != nil {
		log.Printf("[alerts] insert history error: %v", herr)
	}
	if uerr := d.alertRepo.UpdateLastTriggered(ctx, rule.ID, now); uerr != nil {
		log.Printf("[alerts] update last triggered error: %v", uerr)
	}

	log.Printf("[alerts] rule triggered: %s (%s), value=%.4f threshold=%.4f",
		rule.Name, rule.ConditionType, actualValue, rule.Threshold)
}

func (d *Detector) evalDurationP95(ctx context.Context, rule *models.AlertRule) (float64, string, bool, error) {
	windowStart := time.Now().Add(-time.Duration(rule.WindowMinutes) * time.Minute)

	typeDurations, err := d.getCompletedDurations(ctx, windowStart, rule.TaskType)
	if err != nil {
		return 0, "", false, err
	}

	tt := "ALL"
	if rule.TaskType != nil {
		tt = *rule.TaskType
	}
	desc := ""
	triggered := false
	var globalP95 float64

	if rule.TaskType != nil {
		durations := typeDurations[*rule.TaskType]
		if len(durations) == 0 {
			return 0, fmt.Sprintf("[%s] 窗口内无完成任务数据", tt), false, nil
		}
		p95 := calcPercentile(durations, 0.95)
		globalP95 = float64(p95)
		desc = fmt.Sprintf("[%s] P95耗时 %.2fms 超过阈值 %.2fms", tt, p95, rule.Threshold)
		triggered = float64(p95) > rule.Threshold
	} else {
		allDurations := make([]int64, 0)
		for _, ds := range typeDurations {
			allDurations = append(allDurations, ds...)
		}
		if len(allDurations) == 0 {
			return 0, fmt.Sprintf("[%s] 窗口内无完成任务数据", tt), false, nil
		}
		p95 := calcPercentile(allDurations, 0.95)
		globalP95 = float64(p95)
		desc = fmt.Sprintf("[%s] P95耗时 %.2fms 超过阈值 %.2fms", tt, p95, rule.Threshold)
		triggered = float64(p95) > rule.Threshold
	}

	return globalP95, desc, triggered, nil
}

func (d *Detector) evalFailureRate(ctx context.Context, rule *models.AlertRule) (float64, string, bool, error) {
	windowStart := time.Now().Add(-time.Duration(rule.WindowMinutes) * time.Minute)

	typeStats, err := d.getCompletionStats(ctx, windowStart, rule.TaskType)
	if err != nil {
		return 0, "", false, err
	}

	tt := "ALL"
	if rule.TaskType != nil {
		tt = *rule.TaskType
	}

	var total, failed int64
	if rule.TaskType != nil {
		st := typeStats[*rule.TaskType]
		total = st.total
		failed = st.failed
	} else {
		for _, st := range typeStats {
			total += st.total
			failed += st.failed
		}
	}

	if total == 0 {
		return 0, fmt.Sprintf("[%s] 窗口内无完成任务数据", tt), false, nil
	}

	rate := float64(failed) / float64(total) * 100.0
	desc := fmt.Sprintf("[%s] 失败率 %.2f%% (失败%d/总数%d) 超过阈值 %.2f%%",
		tt, rate, failed, total, rule.Threshold)
	triggered := rate > rule.Threshold

	return rate, desc, triggered, nil
}

func (d *Detector) evalQueueBacklog(ctx context.Context, rule *models.AlertRule) (float64, string, bool, error) {
	var depthMap map[string]int64
	var err error

	if d.queueProv != nil {
		depthMap, err = d.queueProv.GetQueueDepthsByType(ctx)
		if err != nil {
			return 0, "", false, err
		}
	}
	if depthMap == nil {
		depthMap, err = d.getReadyDepths(ctx, rule.TaskType)
		if err != nil {
			return 0, "", false, err
		}
	}

	tt := "ALL"
	var val int64
	if rule.TaskType != nil {
		tt = *rule.TaskType
		val = depthMap[*rule.TaskType]
	} else {
		for _, v := range depthMap {
			val += v
		}
	}

	desc := fmt.Sprintf("[%s] Ready队列积压 %d 超过阈值 %.0f", tt, val, rule.Threshold)
	triggered := float64(val) > rule.Threshold

	return float64(val), desc, triggered, nil
}

func (d *Detector) getCompletedDurations(ctx context.Context, from time.Time, taskType *string) (map[string][]int64, error) {
	var rows pgx.Rows
	var err error

	if taskType != nil {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT te.task_type, te.duration_ms
			 FROM task_executions te
			 WHERE te.ended_at >= $1 AND te.ended_at IS NOT NULL
			   AND te.status IN ('success','failed','dead_letter')
			   AND te.task_type = $2`,
			from, *taskType)
	} else {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT t.type, te.duration_ms
			 FROM task_executions te
			 JOIN tasks t ON t.id = te.task_id
			 WHERE te.ended_at >= $1 AND te.ended_at IS NOT NULL
			   AND te.status IN ('success','failed','dead_letter')`,
			from)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]int64)
	for rows.Next() {
		var tt string
		var dur *int64
		if err := rows.Scan(&tt, &dur); err != nil {
			return nil, err
		}
		if dur != nil && *dur > 0 {
			result[tt] = append(result[tt], *dur)
		}
	}
	return result, nil
}

type compStat struct {
	total  int64
	failed int64
}

func (d *Detector) getCompletionStats(ctx context.Context, from time.Time, taskType *string) (map[string]compStat, error) {
	var rows pgx.Rows
	var err error

	if taskType != nil {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT te.status, COUNT(*)
			 FROM task_executions te
			 WHERE te.ended_at >= $1 AND te.ended_at IS NOT NULL
			   AND te.status IN ('success','failed','dead_letter')
			   AND te.task_type = $2
			 GROUP BY te.status`,
			from, *taskType)
	} else {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT t.type, te.status, COUNT(*)
			 FROM task_executions te
			 JOIN tasks t ON t.id = te.task_id
			 WHERE te.ended_at >= $1 AND te.ended_at IS NOT NULL
			   AND te.status IN ('success','failed','dead_letter')
			 GROUP BY t.type, te.status`,
			from)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]compStat)
	for rows.Next() {
		var tt string
		var status string
		var count int64
		if taskType != nil {
			tt = *taskType
			if err := rows.Scan(&status, &count); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&tt, &status, &count); err != nil {
				return nil, err
			}
		}
		st := result[tt]
		st.total += count
		if status == "failed" || status == "dead_letter" {
			st.failed += count
		}
		result[tt] = st
	}
	return result, nil
}

func (d *Detector) getReadyDepths(ctx context.Context, taskType *string) (map[string]int64, error) {
	var rows pgx.Rows
	var err error

	if taskType != nil {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT COUNT(*) FROM tasks WHERE status = 'ready' AND type = $1`, *taskType)
	} else {
		rows, err = d.db.Pool.Query(ctx,
			`SELECT type, COUNT(*) FROM tasks WHERE status = 'ready' GROUP BY type`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	if taskType != nil {
		var cnt int64
		if rows.Next() {
			if err := rows.Scan(&cnt); err != nil {
				return nil, err
			}
		}
		result[*taskType] = cnt
	} else {
		for rows.Next() {
			var tt string
			var cnt int64
			if err := rows.Scan(&tt, &cnt); err != nil {
				return nil, err
			}
			result[tt] = cnt
		}
	}
	return result, nil
}

func calcPercentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
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

type webhookPayload struct {
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	TaskType    *string   `json:"task_type,omitempty"`
	Condition   string    `json:"condition_type"`
	ActualValue float64   `json:"actual_value"`
	Threshold   float64   `json:"threshold_value"`
	Description string    `json:"description"`
	TriggeredAt time.Time `json:"triggered_at"`
}

func (d *Detector) sendWebhook(ctx context.Context, rule *models.AlertRule, actual float64, desc string, now time.Time) error {
	payload := webhookPayload{
		RuleID:      rule.ID.String(),
		RuleName:    rule.Name,
		TaskType:    rule.TaskType,
		Condition:   string(rule.ConditionType),
		ActualValue: actual,
		Threshold:   rule.Threshold,
		Description: desc,
		TriggeredAt: now,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *rule.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
