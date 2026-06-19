import type { UseFetchOptions } from 'nuxt/app'

export interface Task {
  id: string
  type: string
  payload: any
  priority: number
  status: string
  delay_seconds: number
  scheduled_at?: string
  max_retries: number
  retry_count: number
  timeout_seconds: number
  callback_url?: string
  retry_mode: string
  retry_interval?: number
  retry_cron_expr?: string
  last_error?: string
  dag_id?: string
  dag_node_id?: string
  created_at: string
  updated_at: string
  started_at?: string
  completed_at?: string
  handler_id?: string
  worker_id?: string
}

export interface TaskExecution {
  id: string
  task_id: string
  attempt: number
  worker_id: string
  handler_id: string
  started_at: string
  ended_at?: string
  status: string
  error?: string
  duration_ms?: number
}

export interface Worker {
  id: string
  name: string
  hostname: string
  total_slots: number
  used_slots: number
  status: string
  last_heartbeat_at: string
  registered_at: string
  running_tasks?: string[]
  tasks_completed: number
  tasks_failed: number
}

export interface DAGTemplate {
  id: string
  name: string
  description: string
  nodes: DAGNode[]
  edges: DAGEdge[]
  created_at: string
  updated_at: string
}

export interface DAGNode {
  id: string
  task_type: string
  name: string
  payload?: any
  priority: number
  dependencies: string[]
  strategy: string
}

export interface DAGEdge {
  from: string
  to: string
}

export interface DAGRun {
  id: string
  template_id: string
  status: string
  nodes_state: Record<string, any>
  strategy: string
  max_retries: number
  payload?: any
  created_at: string
  updated_at: string
  started_at?: string
  ended_at?: string
}

export interface MetricsSnapshot {
  queue_depths: Record<string, number>
  throughput: number
  success_rates: Record<string, number>
  failure_rates: Record<string, number>
  avg_latency_ms: number
  worker_utilization: number
  dead_letter_count: number
  workers_online: number
  workers_offline: number
  workers_total: number
  timestamp: string
  throttle_counts?: Record<string, number>
}

export interface RateLimitConfig {
  task_type: string
  max_per_second: number
  window_size_ms: number
  enabled: boolean
  updated_at: number
}

export interface RateLimitStatus {
  task_type: string
  current_rate: number
  max_per_second: number
  window_size_ms: number
  usage_percent: number
  wait_queue_size: number
  enabled: boolean
}

export interface RateLimitThrottleStats {
  task_type: string
  throttle_count: number
  window_hours: number
}

export interface TraceEvent {
  id: string
  task_id: string
  task_type: string
  from_status: string
  to_status: string
  trigger: string
  worker_id?: string
  error?: string
  occurred_at: string
}

export interface TraceSummary {
  task_id: string
  task_type: string
  final_status: string
  created_at: string
  completed_at?: string
  total_duration_ms: number
  queue_wait_ms: number
  execution_ms: number
  retry_interval_ms: number
  node_count: number
}

export interface TraceInterval {
  from_status: string
  to_status: string
  duration_ms: number
}

export interface RetryError {
  attempt: number
  error: string
  timestamp: string
}

export interface TraceDetail {
  task_id: string
  task_type: string
  final_status: string
  created_at: string
  completed_at?: string
  total_duration_ms: number
  queue_wait_ms: number
  execution_ms: number
  retry_interval_ms: number
  events: TraceEvent[]
  intervals: TraceInterval[]
  retry_errors: RetryError[]
}

export interface StageStats {
  p50_ms: number
  p90_ms: number
  p99_ms: number
  avg_ms: number
  percent_of_total: number
}

export interface BottleneckAnalysis {
  task_type: string
  total_samples: number
  time_from: string
  time_to: string
  stages: Record<string, StageStats>
  bottleneck_stage?: string
  bottleneck_percent: number
}

const API_BASE = '/api/v1'

function useApi<T>(path: string, options: UseFetchOptions<T> = {}) {
  const config = useRuntimeConfig()
  return useFetch(path, {
    baseURL: config.public.apiBase || API_BASE,
    ...options,
  })
}

export function useTasks(params?: Record<string, any>) {
  return useApi<any>('/tasks', {
    query: params,
    method: 'GET',
  })
}

export function useTask(id: string) {
  return useApi<Task>(`/tasks/${id}`)
}

export function useTaskExecutions(id: string) {
  return useApi<TaskExecution[]>(`/tasks/${id}/executions`)
}

export async function createTask(data: any) {
  const config = useRuntimeConfig()
  return await $fetch<Task>('/tasks', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data,
  })
}

export async function cancelTask(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/tasks/${id}/cancel`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
  })
}

export async function retryTask(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/tasks/${id}/retry`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
  })
}

export function useWorkers() {
  return useApi<Worker[]>('/workers')
}

export function useWorker(id: string) {
  return useApi<Worker>(`/workers/${id}`)
}

export async function registerWorker(data: any) {
  const config = useRuntimeConfig()
  return await $fetch<Worker>('/workers/register', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data,
  })
}

export function useDeadLetters(params?: Record<string, any>) {
  return useApi<any>('/dead-letter', {
    query: params,
    method: 'GET',
  })
}

export function useDeadLetterDetail(id: string) {
  return useApi<any>(`/dead-letter/${id}`)
}

export async function retryDeadLetter(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/dead-letter/${id}/retry`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
  })
}

export async function discardDeadLetter(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/dead-letter/${id}/discard`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
  })
}

export async function batchRetryDeadLetters(ids: string[]) {
  const config = useRuntimeConfig()
  return await $fetch('/dead-letter/batch-retry', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: { ids },
  })
}

export async function batchDiscardDeadLetters(ids: string[]) {
  const config = useRuntimeConfig()
  return await $fetch('/dead-letter/batch-discard', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: { ids },
  })
}

export function useDeadLetterByError() {
  return useApi<Record<string, number>>('/dead-letter/stats/by-error')
}

export function useDAGTemplates() {
  return useApi<DAGTemplate[]>('/dags/templates')
}

export function useDAGTemplate(id: string) {
  return useApi<DAGTemplate>(`/dags/templates/${id}`)
}

export async function createDAGTemplate(data: any) {
  const config = useRuntimeConfig()
  return await $fetch<DAGTemplate>('/dags/templates', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data,
  })
}

export async function runDAG(id: string, data?: any) {
  const config = useRuntimeConfig()
  return await $fetch<any>(`/dags/templates/${id}/run`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data || {},
  })
}

export function useDAGRuns(params?: Record<string, any>) {
  return useApi<any>('/dags/runs', {
    query: params,
    method: 'GET',
  })
}

export function useDAGRun(id: string) {
  return useApi<DAGRun>(`/dags/runs/${id}`)
}

export function useMetricsSnapshot() {
  return useApi<MetricsSnapshot>('/metrics/snapshot')
}

export function useThroughputHistory(hours = 24) {
  return useApi<Record<string, number>>('/metrics/throughput-history', {
    query: { hours },
  })
}

export function useQueueDepths() {
  return useApi<Record<string, number>>('/metrics/queue-depths')
}

export interface DurationHeatmapCell {
  hour: number
  date: string
  p50_ms: number
  p95_ms: number
  p99_ms: number
  sample_size: number
  is_anomaly: boolean
}

export interface DurationHeatmapData {
  task_type: string
  dates: string[]
  hours: number[]
  matrix: (DurationHeatmapCell | null)[][]
}

export interface DurationHeatmapCompareData {
  current: DurationHeatmapData
  previous: DurationHeatmapData
}

export interface DurationHistogramBucket {
  range: string
  range_start_ms: number
  range_end_ms?: number | null
  count: number
  percentage: number
}

export interface DurationHistogramData {
  task_type: string
  time_from: string
  time_to: string
  total_count: number
  buckets: DurationHistogramBucket[]
  avg_ms: number
  p50_ms: number
  p90_ms: number
  p95_ms: number
  p99_ms: number
}

export interface DurationHistogramCompareData {
  first: DurationHistogramData
  second: DurationHistogramData
}

export function useDurationHeatmap(params?: Record<string, any>) {
  return useApi<DurationHeatmapData>('/metrics/duration-heatmap', {
    query: params,
    method: 'GET',
  })
}

export function useDurationHeatmapCompare(params?: Record<string, any>) {
  return useApi<DurationHeatmapCompareData>('/metrics/duration-heatmap', {
    query: { ...params, compare: true },
    method: 'GET',
  })
}

export function useDurationHistogram(params?: Record<string, any>) {
  return useApi<DurationHistogramData>('/metrics/duration-histogram', {
    query: params,
    method: 'GET',
  })
}

export function useDurationHistogramCompare(params?: Record<string, any>) {
  return useApi<DurationHistogramCompareData>('/metrics/duration-histogram', {
    query: params,
    method: 'GET',
  })
}

export function useRateLimitConfigs() {
  return useApi<Record<string, RateLimitConfig>>('/rate-limit/configs')
}

export function useRateLimitStatus() {
  return useApi<RateLimitStatus[]>('/rate-limit/status')
}

export function useRateLimitThrottleStats(hours = 1) {
  return useApi<RateLimitThrottleStats[]>('/rate-limit/throttle-stats', {
    query: { hours },
  })
}

export async function setRateLimitConfig(taskType: string, data: {
  max_per_second: number
  window_size_ms: number
  enabled: boolean
}) {
  const config = useRuntimeConfig()
  return await $fetch<RateLimitConfig>(`/rate-limit/configs/${taskType}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'PUT',
    body: data,
  })
}

export async function deleteRateLimitConfig(taskType: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/rate-limit/configs/${taskType}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'DELETE',
  })
}

export function priorityLabel(p: number): string {
  const map: Record<number, string> = {
    0: 'Bulk',
    1: 'Low',
    2: 'Normal',
    3: 'High',
    4: 'Critical',
  }
  return map[p] || 'Unknown'
}

export function priorityColor(p: number): string {
  const map: Record<number, string> = {
    0: 'gray',
    1: 'blue',
    2: 'green',
    3: 'orange',
    4: 'red',
  }
  return map[p] || 'gray'
}

export function statusColor(s: string): string {
  const map: Record<string, string> = {
    pending: 'gray',
    delayed: 'sky',
    ready: 'blue',
    running: 'amber',
    success: 'green',
    failed: 'red',
    dead_letter: 'rose',
    cancelled: 'slate',
  }
  return map[s] || 'gray'
}

export function workerStatusColor(s: string): string {
  const map: Record<string, string> = {
    online: 'green',
    offline: 'gray',
    draining: 'amber',
  }
  return map[s] || 'gray'
}

export function dagStatusColor(s: string): string {
  const map: Record<string, string> = {
    pending: 'gray',
    running: 'amber',
    success: 'green',
    failed: 'red',
    cancelled: 'slate',
  }
  return map[s] || 'gray'
}

export function formatDate(s?: string): string {
  if (!s) return '-'
  const d = new Date(s)
  return d.toLocaleString()
}

export function formatDuration(ms?: number | string | null): string {
  if (ms === null || ms === undefined || ms === '') return '-'
  const num = Number(ms)
  if (isNaN(num) || num < 0) return '-'
  if (num === 0) return '0ms'
  if (num < 1000) return `${num}ms`
  if (num < 60000) return `${(num / 1000).toFixed(2)}s`
  return `${(num / 60000).toFixed(2)}m`
}

export function stageLabel(s: string): string {
  const map: Record<string, string> = {
    queue_wait: '队列等待',
    execution: '任务执行',
    retry_interval: '重试间隔',
    other: '其他',
  }
  return map[s] || s
}

export function useTraces(params?: Record<string, any>) {
  return useApi<any>('/trace', {
    query: params,
    method: 'GET',
  })
}

export function useTraceDetail(taskId: string) {
  return useApi<TraceDetail>(`/trace/${taskId}`)
}

export function useBottleneckAnalysis(params?: Record<string, any>) {
  return useApi<BottleneckAnalysis>('/trace/analysis/bottleneck', {
    query: params,
    method: 'GET',
  })
}

export type AlertConditionType = 'duration_p95' | 'failure_rate' | 'queue_backlog'
export type AlertNotifyType = 'webhook'

export interface AlertRule {
  id: string
  name: string
  task_type?: string | null
  condition_type: AlertConditionType
  threshold: number
  window_minutes: number
  cooldown_seconds: number
  notify_type: AlertNotifyType
  webhook_url?: string | null
  enabled: boolean
  last_triggered_at?: string | null
  created_at: string
  updated_at: string
}

export interface AlertHistory {
  id: string
  rule_id: string
  rule_name: string
  task_type?: string | null
  condition_type: AlertConditionType
  actual_value: number
  threshold_value: number
  comparison_description: string
  webhook_url?: string | null
  webhook_success: boolean
  webhook_error?: string | null
  triggered_at: string
}

export function alertConditionLabel(t: AlertConditionType): string {
  const map: Record<AlertConditionType, string> = {
    duration_p95: 'P95耗时',
    failure_rate: '失败率',
    queue_backlog: '队列积压',
  }
  return map[t] || t
}

export function alertConditionColor(t: AlertConditionType): string {
  const map: Record<AlertConditionType, string> = {
    duration_p95: 'blue',
    failure_rate: 'red',
    queue_backlog: 'amber',
  }
  return map[t] || 'gray'
}

export function alertConditionUnit(t: AlertConditionType): string {
  const map: Record<AlertConditionType, string> = {
    duration_p95: 'ms',
    failure_rate: '%',
    queue_backlog: '个',
  }
  return map[t] || ''
}

export function formatAlertValue(t: AlertConditionType, v: number): string {
  const unit = alertConditionUnit(t)
  if (t === 'failure_rate') {
    return `${v.toFixed(2)}${unit}`
  }
  if (t === 'duration_p95') {
    return formatDuration(v)
  }
  return `${v}${unit}`
}

export function useAlertRules() {
  return useApi<AlertRule[]>('/alerts/rules', {
    method: 'GET',
  })
}

export function useAlertHistory(params?: Record<string, any>) {
  return useApi<any>('/alerts/history', {
    query: params,
    method: 'GET',
  })
}

export async function createAlertRule(data: any) {
  const config = useRuntimeConfig()
  return await $fetch<AlertRule>('/alerts/rules', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data,
  })
}

export async function updateAlertRule(id: string, data: any) {
  const config = useRuntimeConfig()
  return await $fetch<AlertRule>(`/alerts/rules/${id}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'PUT',
    body: data,
  })
}

export async function toggleAlertRule(id: string) {
  const config = useRuntimeConfig()
  return await $fetch<AlertRule>(`/alerts/rules/${id}/toggle`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'PATCH',
  })
}

export async function deleteAlertRule(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/alerts/rules/${id}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'DELETE',
  })
}

export type ScalingOperationType = 'scale_out' | 'scale_in' | 'no_op'

export interface ScalingPolicy {
  id: string
  task_type: string
  target_utilization_pct: number
  min_workers: number
  max_workers: number
  cooldown_seconds: number
  scale_in_protection_secs: number
  scale_out_threshold: number
  scale_in_threshold_pct: number
  enabled: boolean
  last_operation_at?: string | null
  created_at: string
  updated_at: string
}

export interface ScalingHistory {
  id: string
  policy_id: string
  task_type: string
  operation_type: ScalingOperationType
  reason: string
  suggested_count: number
  snapshot_workers: number
  snapshot_util_pct: number
  snapshot_queue: number
  created_at: string
}

export interface ScalingPolicyMetrics {
  policy_id: string
  task_type: string
  current_workers: number
  utilization_pct: number
  queue_waiting: number
  last_operation_at?: string | null
  seconds_since_op: number
}

export function scalingOpLabel(op: ScalingOperationType): string {
  const map: Record<ScalingOperationType, string> = {
    scale_out: '扩容',
    scale_in: '缩容',
    no_op: '无操作',
  }
  return map[op] || op
}

export function scalingOpColor(op: ScalingOperationType): string {
  const map: Record<ScalingOperationType, string> = {
    scale_out: 'green',
    scale_in: 'red',
    no_op: 'gray',
  }
  return map[op] || 'gray'
}

export function useScalingPolicies() {
  return useApi<ScalingPolicy[]>('/auto-scaling/policies', {
    method: 'GET',
  })
}

export function useScalingPolicy(id: string) {
  return useApi<ScalingPolicy>(`/auto-scaling/policies/${id}`)
}

export async function createScalingPolicy(data: any) {
  const config = useRuntimeConfig()
  return await $fetch<ScalingPolicy>('/auto-scaling/policies', {
    baseURL: config.public.apiBase || API_BASE,
    method: 'POST',
    body: data,
  })
}

export async function updateScalingPolicy(id: string, data: any) {
  const config = useRuntimeConfig()
  return await $fetch<ScalingPolicy>(`/auto-scaling/policies/${id}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'PUT',
    body: data,
  })
}

export async function toggleScalingPolicy(id: string) {
  const config = useRuntimeConfig()
  return await $fetch<ScalingPolicy>(`/auto-scaling/policies/${id}/toggle`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'PATCH',
  })
}

export async function deleteScalingPolicy(id: string) {
  const config = useRuntimeConfig()
  return await $fetch(`/auto-scaling/policies/${id}`, {
    baseURL: config.public.apiBase || API_BASE,
    method: 'DELETE',
  })
}

export function useScalingMetrics() {
  return useApi<ScalingPolicyMetrics[]>('/auto-scaling/metrics', {
    method: 'GET',
  })
}

export function useScalingHistory(params?: Record<string, any>) {
  return useApi<any>('/auto-scaling/history', {
    query: params,
    method: 'GET',
  })
}
