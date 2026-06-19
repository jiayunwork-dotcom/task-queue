package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Priority int

const (
	PriorityBulk     Priority = 0
	PriorityLow      Priority = 1
	PriorityNormal   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityHigh:
		return "high"
	case PriorityNormal:
		return "normal"
	case PriorityLow:
		return "low"
	case PriorityBulk:
		return "bulk"
	default:
		return "unknown"
	}
}

func PriorityFromString(s string) Priority {
	switch s {
	case "critical":
		return PriorityCritical
	case "high":
		return PriorityHigh
	case "normal":
		return PriorityNormal
	case "low":
		return PriorityLow
	case "bulk":
		return PriorityBulk
	default:
		return PriorityNormal
	}
}

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusDelayed    TaskStatus = "delayed"
	TaskStatusReady      TaskStatus = "ready"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusSuccess    TaskStatus = "success"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusDeadLetter TaskStatus = "dead_letter"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type RetryMode string

const (
	RetryModeExponential RetryMode = "exponential"
	RetryModeFixed       RetryMode = "fixed"
	RetryModeCron        RetryMode = "cron"
)

type DAGNodeStrategy string

const (
	DAGStrategyAbort    DAGNodeStrategy = "abort"
	DAGStrategySkip     DAGNodeStrategy = "skip"
	DAGStrategyRetry    DAGNodeStrategy = "retry"
)

type DAGStatus string

const (
	DAGStatusPending   DAGStatus = "pending"
	DAGStatusRunning   DAGStatus = "running"
	DAGStatusSuccess   DAGStatus = "success"
	DAGStatusFailed    DAGStatus = "failed"
	DAGStatusCancelled DAGStatus = "cancelled"
)

type WorkerStatus string

const (
	WorkerStatusOnline  WorkerStatus = "online"
	WorkerStatusOffline WorkerStatus = "offline"
	WorkerStatusDraining WorkerStatus = "draining"
)

type Task struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	Type            string          `json:"type" db:"type"`
	Payload         json.RawMessage `json:"payload" db:"payload"`
	Priority        Priority        `json:"priority" db:"priority"`
	Status          TaskStatus      `json:"status" db:"status"`
	DelaySeconds    int             `json:"delay_seconds,omitempty" db:"delay_seconds"`
	ScheduledAt     *time.Time      `json:"scheduled_at,omitempty" db:"scheduled_at"`
	MaxRetries      int             `json:"max_retries" db:"max_retries"`
	RetryCount      int             `json:"retry_count" db:"retry_count"`
	TimeoutSeconds  int             `json:"timeout_seconds" db:"timeout_seconds"`
	CallbackURL     string          `json:"callback_url,omitempty" db:"callback_url"`
	RetryMode       RetryMode       `json:"retry_mode" db:"retry_mode"`
	RetryInterval   int             `json:"retry_interval,omitempty" db:"retry_interval"`
	RetryCronExpr   string          `json:"retry_cron_expr,omitempty" db:"retry_cron_expr"`
	LastError       string          `json:"last_error,omitempty" db:"last_error"`
	DAGID           *uuid.UUID      `json:"dag_id,omitempty" db:"dag_id"`
	DAGNodeID       *string         `json:"dag_node_id,omitempty" db:"dag_node_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	HandlerID       *string         `json:"handler_id,omitempty" db:"handler_id"`
	WorkerID        *uuid.UUID      `json:"worker_id,omitempty" db:"worker_id"`
	LeaseExpiresAt  *time.Time      `json:"lease_expires_at,omitempty" db:"lease_expires_at"`
}

type TaskExecution struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TaskID      uuid.UUID  `json:"task_id" db:"task_id"`
	Attempt     int        `json:"attempt" db:"attempt"`
	WorkerID    uuid.UUID  `json:"worker_id" db:"worker_id"`
	HandlerID   string     `json:"handler_id" db:"handler_id"`
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty" db:"ended_at"`
	Status      TaskStatus `json:"status" db:"status"`
	Error       string     `json:"error,omitempty" db:"error"`
	DurationMS  int64      `json:"duration_ms,omitempty" db:"duration_ms"`
}

type Worker struct {
	ID              uuid.UUID    `json:"id" db:"id"`
	Name            string       `json:"name" db:"name"`
	Hostname        string       `json:"hostname" db:"hostname"`
	TotalSlots      int          `json:"total_slots" db:"total_slots"`
	UsedSlots       int          `json:"used_slots" db:"used_slots"`
	Status          WorkerStatus `json:"status" db:"status"`
	LastHeartbeatAt time.Time    `json:"last_heartbeat_at" db:"last_heartbeat_at"`
	RegisteredAt    time.Time    `json:"registered_at" db:"registered_at"`
	RunningTasks    []uuid.UUID  `json:"running_tasks,omitempty" db:"-"`
	TasksCompleted  int64        `json:"tasks_completed" db:"tasks_completed"`
	TasksFailed     int64        `json:"tasks_failed" db:"tasks_failed"`
}

type HandlerRegistration struct {
	ID         uuid.UUID `json:"id" db:"id"`
	TaskType   string    `json:"task_type" db:"task_type"`
	HandlerID  string    `json:"handler_id" db:"handler_id"`
	WorkerID   uuid.UUID `json:"worker_id" db:"worker_id"`
	Endpoint   string    `json:"endpoint" db:"endpoint"`
	RegisteredAt time.Time `json:"registered_at" db:"registered_at"`
}

type DAGTemplate struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description,omitempty" db:"description"`
	Nodes       json.RawMessage `json:"nodes" db:"nodes"`
	Edges       json.RawMessage `json:"edges" db:"edges"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

type DAGRun struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	TemplateID uuid.UUID       `json:"template_id" db:"template_id"`
	Status     DAGStatus       `json:"status" db:"status"`
	NodesState json.RawMessage `json:"nodes_state" db:"nodes_state"`
	Strategy   DAGNodeStrategy `json:"strategy" db:"strategy"`
	MaxRetries int             `json:"max_retries" db:"max_retries"`
	Payload    json.RawMessage `json:"payload,omitempty" db:"payload"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty" db:"started_at"`
	EndedAt    *time.Time      `json:"ended_at,omitempty" db:"ended_at"`
}

type DAGNode struct {
	ID           string          `json:"id"`
	TaskType     string          `json:"task_type"`
	Name         string          `json:"name"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Priority     Priority        `json:"priority"`
	Dependencies []string        `json:"dependencies"`
	Strategy     DAGNodeStrategy `json:"strategy"`
}

type DAGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AuditLog struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	EntityType string          `json:"entity_type" db:"entity_type"`
	EntityID   uuid.UUID       `json:"entity_id" db:"entity_id"`
	Action     string          `json:"action" db:"action"`
	OldState   json.RawMessage `json:"old_state,omitempty" db:"old_state"`
	NewState   json.RawMessage `json:"new_state,omitempty" db:"new_state"`
	Operator   string          `json:"operator,omitempty" db:"operator"`
	RemoteAddr string          `json:"remote_addr,omitempty" db:"remote_addr"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

type RateLimitStatus struct {
	TaskType      string  `json:"task_type"`
	CurrentRate   float64 `json:"current_rate"`
	MaxPerSecond  int     `json:"max_per_second"`
	WindowSizeMs  int     `json:"window_size_ms"`
	UsagePercent  float64 `json:"usage_percent"`
	WaitQueueSize int     `json:"wait_queue_size"`
	Enabled       bool    `json:"enabled"`
}

type RateLimitThrottleStats struct {
	TaskType     string `json:"task_type"`
	ThrottleCount int64  `json:"throttle_count"`
	WindowHours  int    `json:"window_hours"`
}

type MetricsSnapshot struct {
	QueueDepths       map[Priority]int64 `json:"queue_depths"`
	Throughput        float64            `json:"throughput"`
	SuccessRates      map[Priority]float64 `json:"success_rates"`
	FailureRates      map[Priority]float64 `json:"failure_rates"`
	AvgLatency        float64            `json:"avg_latency_ms"`
	WorkerUtilization float64            `json:"worker_utilization"`
	DeadLetterCount   int64              `json:"dead_letter_count"`
	WorkersOnline     int                `json:"workers_online"`
	WorkersOffline    int                `json:"workers_offline"`
	WorkersTotal      int                `json:"workers_total"`
	Timestamp         time.Time          `json:"timestamp"`
	ThrottleCounts    map[string]int64   `json:"throttle_counts,omitempty"`
}

type TraceEvent struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	TaskID      uuid.UUID   `json:"task_id" db:"task_id"`
	TaskType    string      `json:"task_type" db:"task_type"`
	FromStatus  TaskStatus  `json:"from_status" db:"from_status"`
	ToStatus    TaskStatus  `json:"to_status" db:"to_status"`
	Trigger     string      `json:"trigger" db:"trigger"`
	WorkerID    *uuid.UUID  `json:"worker_id,omitempty" db:"worker_id"`
	Error       string      `json:"error,omitempty" db:"error"`
	OccurredAt  time.Time   `json:"occurred_at" db:"occurred_at"`
	FinalStatus *TaskStatus `json:"final_status,omitempty" db:"-"`
}

type TraceSummary struct {
	TaskID           uuid.UUID  `json:"task_id"`
	TaskType         string     `json:"task_type"`
	FinalStatus      TaskStatus `json:"final_status"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	TotalDurationMs  int64      `json:"total_duration_ms"`
	QueueWaitMs      int64      `json:"queue_wait_ms"`
	ExecutionMs      int64      `json:"execution_ms"`
	RetryIntervalMs  int64      `json:"retry_interval_ms"`
	NodeCount        int        `json:"node_count"`
}

type TraceDetail struct {
	TaskID           uuid.UUID       `json:"task_id"`
	TaskType         string          `json:"task_type"`
	FinalStatus      TaskStatus      `json:"final_status"`
	CreatedAt        time.Time       `json:"created_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	TotalDurationMs  int64           `json:"total_duration_ms"`
	QueueWaitMs      int64           `json:"queue_wait_ms"`
	ExecutionMs      int64           `json:"execution_ms"`
	RetryIntervalMs  int64           `json:"retry_interval_ms"`
	Events           []TraceEvent    `json:"events"`
	Intervals        []TraceInterval `json:"intervals"`
	RetryErrors      []RetryError    `json:"retry_errors"`
}

type TraceInterval struct {
	FromStatus  TaskStatus `json:"from_status"`
	ToStatus    TaskStatus `json:"to_status"`
	DurationMs  int64      `json:"duration_ms"`
}

type RetryError struct {
	Attempt   int    `json:"attempt"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

type BottleneckAnalysis struct {
	TaskType       string                `json:"task_type"`
	TotalSamples   int64                 `json:"total_samples"`
	TimeFrom       time.Time             `json:"time_from"`
	TimeTo         time.Time             `json:"time_to"`
	Stages         map[string]StageStats `json:"stages"`
	Bottleneck     *string               `json:"bottleneck_stage"`
	BottleneckPct  float64               `json:"bottleneck_percent"`
}

type StageStats struct {
	P50Ms int64   `json:"p50_ms"`
	P90Ms int64   `json:"p90_ms"`
	P99Ms int64   `json:"p99_ms"`
	AvgMs float64 `json:"avg_ms"`
	Pct   float64 `json:"percent_of_total"`
}

type AlertConditionType string

const (
	AlertConditionDurationP95   AlertConditionType = "duration_p95"
	AlertConditionFailureRate   AlertConditionType = "failure_rate"
	AlertConditionQueueBacklog  AlertConditionType = "queue_backlog"
)

type AlertNotifyType string

const (
	AlertNotifyWebhook AlertNotifyType = "webhook"
)

type AlertRule struct {
	ID               uuid.UUID          `json:"id" db:"id"`
	Name             string             `json:"name" db:"name"`
	TaskType         *string            `json:"task_type,omitempty" db:"task_type"`
	ConditionType    AlertConditionType `json:"condition_type" db:"condition_type"`
	Threshold        float64            `json:"threshold" db:"threshold"`
	WindowMinutes    int                `json:"window_minutes" db:"window_minutes"`
	CooldownSeconds  int                `json:"cooldown_seconds" db:"cooldown_seconds"`
	NotifyType       AlertNotifyType    `json:"notify_type" db:"notify_type"`
	WebhookURL       *string            `json:"webhook_url,omitempty" db:"webhook_url"`
	Enabled          bool               `json:"enabled" db:"enabled"`
	LastTriggeredAt  *time.Time         `json:"last_triggered_at,omitempty" db:"last_triggered_at"`
	CreatedAt        time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" db:"updated_at"`
}

type AlertHistory struct {
	ID                   uuid.UUID          `json:"id" db:"id"`
	RuleID               uuid.UUID          `json:"rule_id" db:"rule_id"`
	RuleName             string             `json:"rule_name" db:"rule_name"`
	TaskType             *string            `json:"task_type,omitempty" db:"task_type"`
	ConditionType        AlertConditionType `json:"condition_type" db:"condition_type"`
	ActualValue          float64            `json:"actual_value" db:"actual_value"`
	ThresholdValue       float64            `json:"threshold_value" db:"threshold_value"`
	ComparisonDescription string            `json:"comparison_description" db:"comparison_description"`
	WebhookURL           *string            `json:"webhook_url,omitempty" db:"webhook_url"`
	WebhookSuccess       bool               `json:"webhook_success" db:"webhook_success"`
	WebhookError         *string            `json:"webhook_error,omitempty" db:"webhook_error"`
	TriggeredAt          time.Time          `json:"triggered_at" db:"triggered_at"`
}

type DurationHeatmapCell struct {
	Hour       int     `json:"hour"`
	Date       string  `json:"date"`
	P50Ms      int64   `json:"p50_ms"`
	P95Ms      int64   `json:"p95_ms"`
	P99Ms      int64   `json:"p99_ms"`
	SampleSize int64   `json:"sample_size"`
	IsAnomaly  bool    `json:"is_anomaly"`
}

type DurationHeatmapData struct {
	TaskType  string                 `json:"task_type"`
	Dates     []string               `json:"dates"`
	Hours     []int                  `json:"hours"`
	Matrix    [][]*DurationHeatmapCell `json:"matrix"`
}

type DurationHeatmapCompareData struct {
	Current  *DurationHeatmapData `json:"current"`
	Previous *DurationHeatmapData `json:"previous"`
}

type DurationHistogramBucket struct {
	Range      string  `json:"range"`
	RangeStart int64   `json:"range_start_ms"`
	RangeEnd   *int64  `json:"range_end_ms,omitempty"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type DurationHistogramData struct {
	TaskType   string                   `json:"task_type"`
	TimeFrom   time.Time                `json:"time_from"`
	TimeTo     time.Time                `json:"time_to"`
	TotalCount int64                    `json:"total_count"`
	Buckets    []DurationHistogramBucket `json:"buckets"`
	AvgMs      float64                  `json:"avg_ms"`
	P50Ms      int64                    `json:"p50_ms"`
	P90Ms      int64                    `json:"p90_ms"`
	P95Ms      int64                    `json:"p95_ms"`
	P99Ms      int64                    `json:"p99_ms"`
}

type DurationHistogramCompareData struct {
	First  *DurationHistogramData `json:"first"`
	Second *DurationHistogramData `json:"second"`
}

type ScalingOperationType string

const (
	ScalingOpScaleOut ScalingOperationType = "scale_out"
	ScalingOpScaleIn  ScalingOperationType = "scale_in"
	ScalingOpNoOp     ScalingOperationType = "no_op"
)

type ScheduleWindow struct {
	Days       []int  `json:"days" db:"days"`
	StartTime  string `json:"start_time" db:"start_time"`
	EndTime    string `json:"end_time" db:"end_time"`
}

type ScalingEvent struct {
	EventTime      time.Time           `json:"event_time"`
	PolicyID       uuid.UUID           `json:"policy_id"`
	TaskType       string              `json:"task_type"`
	OperationType  ScalingOperationType `json:"operation_type"`
	SuggestedCount int                 `json:"suggested_count"`
}

type ScalingPolicy struct {
	ID                    uuid.UUID        `json:"id" db:"id"`
	TaskType              string           `json:"task_type" db:"task_type"`
	TargetUtilizationPct  float64          `json:"target_utilization_pct" db:"target_utilization_pct"`
	MinWorkers            int              `json:"min_workers" db:"min_workers"`
	MaxWorkers            int              `json:"max_workers" db:"max_workers"`
	CooldownSeconds       int              `json:"cooldown_seconds" db:"cooldown_seconds"`
	ScaleInProtectionSecs int              `json:"scale_in_protection_secs" db:"scale_in_protection_secs"`
	ScaleOutThreshold     int              `json:"scale_out_threshold" db:"scale_out_threshold"`
	ScaleInThresholdPct   float64          `json:"scale_in_threshold_pct" db:"scale_in_threshold_pct"`
	Enabled               bool             `json:"enabled" db:"enabled"`
	ScheduleWindows       []ScheduleWindow `json:"schedule_windows" db:"schedule_windows"`
	LastOperationAt       *time.Time       `json:"last_operation_at,omitempty" db:"last_operation_at"`
	CreatedAt             time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time        `json:"updated_at" db:"updated_at"`
}

type ScalingHistory struct {
	ID              uuid.UUID           `json:"id" db:"id"`
	PolicyID        uuid.UUID           `json:"policy_id" db:"policy_id"`
	TaskType        string              `json:"task_type" db:"task_type"`
	OperationType   ScalingOperationType `json:"operation_type" db:"operation_type"`
	Reason          string              `json:"reason" db:"reason"`
	SuggestedCount  int                 `json:"suggested_count" db:"suggested_count"`
	SnapshotWorkers int                 `json:"snapshot_workers" db:"snapshot_workers"`
	SnapshotUtilPct float64             `json:"snapshot_util_pct" db:"snapshot_util_pct"`
	SnapshotQueue   int                 `json:"snapshot_queue" db:"snapshot_queue"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
}

type ScalingPolicyMetrics struct {
	PolicyID       uuid.UUID `json:"policy_id"`
	TaskType       string    `json:"task_type"`
	CurrentWorkers int       `json:"current_workers"`
	UtilizationPct float64   `json:"utilization_pct"`
	QueueWaiting   int       `json:"queue_waiting"`
	LastOperationAt *time.Time `json:"last_operation_at,omitempty"`
	SecondsSinceOp int       `json:"seconds_since_op"`
}

func ParseTimeStr(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", parts[1])
	}
	return h, m, nil
}

func (w *ScheduleWindow) Contains(t time.Time) bool {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	dayMatch := false
	for _, d := range w.Days {
		if d == weekday {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false
	}

	startH, startM, err := ParseTimeStr(w.StartTime)
	if err != nil {
		return false
	}
	endH, endM, err := ParseTimeStr(w.EndTime)
	if err != nil {
		return false
	}

	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM
	currentMinutes := t.Hour()*60 + t.Minute()

	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

func (p *ScalingPolicy) IsWithinScheduleWindow(t time.Time) bool {
	if p.ScheduleWindows == nil || len(p.ScheduleWindows) == 0 {
		return true
	}
	for _, w := range p.ScheduleWindows {
		if w.Contains(t) {
			return true
		}
	}
	return false
}

func (p *ScalingPolicy) ScheduleWindowSummary() string {
	if p.ScheduleWindows == nil || len(p.ScheduleWindows) == 0 {
		return "Always"
	}

	dayNames := []string{"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	parts := make([]string, 0, len(p.ScheduleWindows))
	for _, w := range p.ScheduleWindows {
		if len(w.Days) == 0 {
			continue
		}

		dayStr := ""
		if len(w.Days) == 7 {
			dayStr = "Everyday"
		} else if len(w.Days) == 5 && w.Days[0] == 1 && w.Days[4] == 5 {
			dayStr = "Mon-Fri"
		} else if len(w.Days) == 2 && w.Days[0] == 6 && w.Days[1] == 7 {
			dayStr = "Sat-Sun"
		} else {
			dayNamesSelected := make([]string, 0, len(w.Days))
			for _, d := range w.Days {
				if d >= 1 && d <= 7 {
					dayNamesSelected = append(dayNamesSelected, dayNames[d])
				}
			}
			dayStr = strings.Join(dayNamesSelected, ",")
		}

		parts = append(parts, fmt.Sprintf("%s %s-%s", dayStr, w.StartTime, w.EndTime))
	}

	if len(parts) == 0 {
		return "Always"
	}
	return strings.Join(parts, "; ")
}
