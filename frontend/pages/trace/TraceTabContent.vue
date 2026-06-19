<template>
  <div class="space-y-6">
    <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex flex-col md:flex-row gap-4 md:items-end">
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 flex-1">
          <div>
            <UFormLabel class="text-xs">时间范围</UFormLabel>
            <USelect
              v-model="filters.timeRange"
              :options="timeRangeOptions"
              size="sm"
            />
          </div>
          <div v-if="filters.timeRange === 'custom'">
            <UFormLabel class="text-xs">开始时间</UFormLabel>
            <UInput
              v-model="filters.customFrom"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div v-if="filters.timeRange === 'custom'">
            <UFormLabel class="text-xs">结束时间</UFormLabel>
            <UInput
              v-model="filters.customTo"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">任务类型</UFormLabel>
            <UInput
              v-model="filters.taskType"
              placeholder="所有类型"
              size="sm"
            />
          </div>
          <div class="md:col-span-2">
            <UFormLabel class="text-xs">最终状态</UFormLabel>
            <div class="flex flex-wrap gap-2">
              <label
                v-for="opt in statusOptions"
                :key="opt.value"
                class="inline-flex items-center gap-1 px-2 py-1 rounded-md border cursor-pointer transition-colors text-xs"
                :class="filters.finalStatuses.includes(opt.value)
                  ? 'bg-blue-100 dark:bg-blue-900 border-blue-300 dark:border-blue-700'
                  : 'bg-gray-50 dark:bg-gray-700 border-gray-200 dark:border-gray-600'"
                @click="toggleStatus(opt.value)"
              >
                <input type="checkbox" class="rounded" :checked="filters.finalStatuses.includes(opt.value)" @click.stop />
                <span>{{ opt.label }}</span>
              </label>
            </div>
          </div>
        </div>
        <div class="flex gap-2">
          <UButton size="sm" variant="outline" @click="resetFilters">重置</UButton>
          <UButton size="sm" @click="loadData">查询</UButton>
        </div>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">任务ID</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">类型</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">最终状态</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">总耗时</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">入队等待</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">执行耗时</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">重试间隔</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">节点数</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <template v-for="trace in traces" :key="trace.task_id">
              <tr
                class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
                @click="toggleExpand(trace.task_id)"
              >
                <td class="px-6 py-4">
                  <span class="font-mono text-xs text-gray-600 dark:text-gray-400">{{ trace.task_id.slice(0, 8) }}...</span>
                </td>
                <td class="px-6 py-4">
                  <span class="font-medium text-gray-900 dark:text-white">{{ trace.task_type }}</span>
                </td>
                <td class="px-6 py-4">
                  <UBadge :color="statusColor(trace.final_status)" variant="subtle" size="sm">
                    {{ trace.final_status }}
                  </UBadge>
                </td>
                <td class="px-6 py-4 font-mono text-sm">{{ formatDuration(trace.total_duration_ms) }}</td>
                <td class="px-6 py-4 font-mono text-sm text-gray-600 dark:text-gray-400">{{ formatDuration(trace.queue_wait_ms) }}</td>
                <td class="px-6 py-4 font-mono text-sm text-gray-600 dark:text-gray-400">{{ formatDuration(trace.execution_ms) }}</td>
                <td class="px-6 py-4 font-mono text-sm text-gray-600 dark:text-gray-400">{{ formatDuration(trace.retry_interval_ms) }}</td>
                <td class="px-6 py-4 text-sm">{{ trace.node_count }}</td>
              </tr>
              <tr v-if="expanded.has(trace.task_id)" class="bg-gray-50 dark:bg-gray-900/30">
                <td colspan="8" class="px-6 py-4">
                  <div v-if="expandedDetails[trace.task_id]">
                    <p class="text-sm font-semibold mb-3">状态变迁时间线</p>
                    <div class="overflow-x-auto pb-4">
                      <div class="flex items-start min-w-max py-4 px-2">
                        <template v-for="(ev, idx) in expandedDetails[trace.task_id].events" :key="ev.id">
                          <div class="flex flex-col items-center max-w-[180px]">
                            <div
                              class="w-4 h-4 rounded-full border-2 border-white dark:border-gray-800 z-10"
                              :class="statusDotColor(ev.to_status)"
                            ></div>
                            <div class="mt-2 text-center px-2">
                              <p class="text-xs font-semibold">{{ ev.to_status }}</p>
                              <p class="text-[10px] text-gray-500">{{ formatDate(ev.occurred_at) }}</p>
                              <p v-if="ev.worker_id" class="text-[10px] text-gray-500">Worker: {{ ev.worker_id.slice(0, 8) }}</p>
                              <p v-if="ev.error" class="text-[10px] text-red-500 mt-1 max-w-[160px] overflow-hidden text-ellipsis">{{ ev.error.slice(0, 30) }}...</p>
                            </div>
                          </div>
                          <div v-if="idx < expandedDetails[trace.task_id].events.length - 1" class="flex flex-col items-center pt-2">
                            <div class="h-1 bg-gray-300 dark:bg-gray-600" style="width:80px;margin-top:7px"></div>
                            <span class="text-[10px] text-gray-500 mt-1 whitespace-nowrap">{{ getIntervalDuration(trace.task_id, idx) }}</span>
                          </div>
                        </template>
                      </div>
                    </div>
                    <div v-if="expandedDetails[trace.task_id].retry_errors.length > 0" class="mt-4">
                      <p class="text-sm font-semibold mb-2">重试错误摘要</p>
                      <div class="space-y-2">
                        <div v-for="re in expandedDetails[trace.task_id].retry_errors" :key="re.attempt" class="bg-red-50 dark:bg-red-900/20 rounded-lg p-3">
                          <div class="flex items-center gap-2 text-xs text-gray-500 mb-1">
                            <span>Attempt {{ re.attempt }}</span>
                            <span>{{ re.timestamp }}</span>
                          </div>
                          <pre class="text-xs text-red-600 dark:text-red-400 whitespace-pre-wrap break-words">{{ re.error }}</pre>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-else class="text-center py-4 text-gray-500">
                    <Icon name="i-heroicons-arrow-path-16-solid" class="w-5 h-5 mx-auto animate-spin" />
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="traces.length === 0">
              <td colspan="8" class="px-6 py-16 text-center text-gray-400">
                <Icon name="i-heroicons-inbox-stack-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>暂无链路数据</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="px-6 py-4 bg-gray-50 dark:bg-gray-900/30 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <p class="text-sm text-gray-500">共 {{ total }} 条记录</p>
        <div class="flex gap-2">
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset === 0"
            @click="prevPage"
          >上一页</UButton>
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset + limit >= total"
            @click="nextPage"
          >下一页</UButton>
        </div>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-bold">瓶颈分析</h3>
        <div class="text-sm text-gray-500">
          样本数: {{ bottleneck?.total_samples || 0 }}
          <span v-if="bottleneck?.bottleneck_stage" class="ml-3">
            <UBadge color="red" size="sm">
              瓶颈: {{ stageLabel(bottleneck.bottleneck_stage) }} ({{ bottleneck.bottleneck_percent.toFixed(1) }}%)
            </UBadge>
          </span>
        </div>
      </div>
      <div v-if="bottleneck && Object.keys(bottleneck.stages).length > 0" class="h-80">
        <Bar :data="chartData" :options="chartOptions" />
      </div>
      <div v-else class="py-12 text-center text-gray-400">
        <Icon name="i-heroicons-chart-bar-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
        <p>暂无瓶颈分析数据</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import {
  useTraces, useTraceDetail, useBottleneckAnalysis,
  statusColor, formatDuration, formatDate, stageLabel,
  type TraceSummary, type TraceDetail, type BottleneckAnalysis,
} from '~/composables/useApi'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const timeRangeOptions = [
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: 'custom', label: '自定义' },
]

const statusOptions = [
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
  { value: 'dead_letter', label: '死信' },
  { value: 'cancelled', label: '取消' },
]

const filters = ref({
  timeRange: '1h',
  customFrom: '',
  customTo: '',
  taskType: '',
  finalStatuses: [] as string[],
})

const limit = 20
const offset = ref(0)
const traces = ref<TraceSummary[]>([])
const total = ref(0)
const expanded = ref<Set<string>>(new Set())
const expandedDetails = ref<Record<string, TraceDetail>>({})
const bottleneck = ref<BottleneckAnalysis | null>(null)

function statusDotColor(s: string) {
  const map: Record<string, string> = {
    success: 'bg-green-500',
    failed: 'bg-red-500',
    dead_letter: 'bg-red-500',
    running: 'bg-amber-500',
    cancelled: 'bg-slate-500',
  }
  return map[s] || 'bg-blue-500'
}

function getIntervalDuration(taskId: string, idx: number): string {
  const detail = expandedDetails.value[taskId]
  if (!detail || !detail.intervals || !detail.intervals[idx]) return '-'
  return formatDuration(detail.intervals[idx].duration_ms)
}

function toggleStatus(v: string) {
  const idx = filters.value.finalStatuses.indexOf(v)
  if (idx >= 0) {
    filters.value.finalStatuses.splice(idx, 1)
  } else {
    filters.value.finalStatuses.push(v)
  }
}

function resetFilters() {
  filters.value = {
    timeRange: '1h',
    customFrom: '',
    customTo: '',
    taskType: '',
    finalStatuses: [],
  }
  offset.value = 0
  loadData()
}

function getTimeRange() {
  const now = new Date()
  let from: Date
  switch (filters.value.timeRange) {
    case '1h':
      from = new Date(now.getTime() - 3600 * 1000)
      break
    case '6h':
      from = new Date(now.getTime() - 6 * 3600 * 1000)
      break
    case '24h':
      from = new Date(now.getTime() - 24 * 3600 * 1000)
      break
    case 'custom':
      from = filters.value.customFrom ? new Date(filters.value.customFrom) : new Date(now.getTime() - 3600 * 1000)
      const to = filters.value.customTo ? new Date(filters.value.customTo) : now
      return { from, to }
    default:
      from = new Date(now.getTime() - 3600 * 1000)
  }
  return { from, to: now }
}

async function loadData() {
  const { from, to } = getTimeRange()
  const params: Record<string, any> = {
    limit,
    offset: offset.value,
    from: from.toISOString(),
    to: to.toISOString(),
  }
  if (filters.value.taskType) params.type = filters.value.taskType
  if (filters.value.finalStatuses.length > 0) {
    params.final_statuses = filters.value.finalStatuses.join(',')
  }

  const { data } = await useTraces(params)
  if (data.value) {
    traces.value = (data.value as any).items || []
    total.value = (data.value as any).total || 0
  }

  const bnParams: Record<string, any> = {
    from: from.toISOString(),
    to: to.toISOString(),
  }
  if (filters.value.taskType) bnParams.type = filters.value.taskType
  const { data: bnData } = await useBottleneckAnalysis(bnParams)
  if (bnData.value) {
    bottleneck.value = bnData.value as any
  }
}

function prevPage() {
  if (offset.value >= limit) {
    offset.value -= limit
    loadData()
  }
}

function nextPage() {
  offset.value += limit
  loadData()
}

async function toggleExpand(taskId: string) {
  if (expanded.value.has(taskId)) {
    expanded.value.delete(taskId)
    delete expandedDetails.value[taskId]
  } else {
    expanded.value.add(taskId)
    const { data } = await useTraceDetail(taskId)
    if (data.value) {
      expandedDetails.value[taskId] = data.value as any
    }
  }
}

const chartData = computed(() => {
  if (!bottleneck.value) return { labels: [], datasets: [] }
  const stages = Object.keys(bottleneck.value.stages)
  const bn = bottleneck.value.bottleneck_stage
  return {
    labels: stages.map(s => stageLabel(s)),
    datasets: [
      {
        label: 'P50',
        data: stages.map(s => bottleneck.value!.stages[s].p50_ms),
        backgroundColor: stages.map(s => s === bn ? 'rgba(239,68,68,0.8)' : 'rgba(59,130,246,0.8)'),
      },
      {
        label: 'P90',
        data: stages.map(s => bottleneck.value!.stages[s].p90_ms),
        backgroundColor: stages.map(s => s === bn ? 'rgba(239,68,68,0.65)' : 'rgba(59,130,246,0.65)'),
      },
      {
        label: 'P99',
        data: stages.map(s => bottleneck.value!.stages[s].p99_ms),
        backgroundColor: stages.map(s => s === bn ? 'rgba(239,68,68,0.5)' : 'rgba(59,130,246,0.5)'),
      },
    ],
  }
})

const chartOptions = {
  indexAxis: 'y' as const,
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { position: 'top' as const },
    tooltip: {
      callbacks: {
        label: (ctx: any) => `${ctx.dataset.label}: ${formatDuration(ctx.raw)}`,
      },
    },
  },
  scales: {
    x: {
      ticks: {
        callback: (val: any) => formatDuration(val) ,
      },
    },
  },
}

onMounted(() => loadData())
</script>
