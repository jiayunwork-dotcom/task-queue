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

export function formatDuration(ms?: number): string {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`
  return `${(ms / 60000).toFixed(2)}m`
}
