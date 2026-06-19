<template>
  <div class="space-y-4">
    <div class="flex flex-col md:flex-row gap-4 md:items-end">
      <div class="flex-1">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <UFormLabel class="text-xs">时间范围</UFormLabel>
            <USelect
              v-model="timeRange"
              :options="timeRangeOptions"
              size="sm"
            />
          </div>
          <div v-if="timeRange === 'custom'">
            <UFormLabel class="text-xs">开始时间</UFormLabel>
            <UInput
              v-model="customFrom"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div v-if="timeRange === 'custom'">
            <UFormLabel class="text-xs">结束时间</UFormLabel>
            <UInput
              v-model="customTo"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">任务类型</UFormLabel>
            <UInput
              v-model="taskType"
              placeholder="所有类型"
              size="sm"
              @keyup.enter="loadData"
            />
          </div>
        </div>
        <div v-if="compareMode" class="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <div>
            <UFormLabel class="text-xs">对比时间范围</UFormLabel>
            <USelect
              v-model="compareTimeRange"
              :options="compareTimeRangeOptions"
              size="sm"
            />
          </div>
          <div v-if="compareTimeRange === 'custom'">
            <UFormLabel class="text-xs">对比开始时间</UFormLabel>
            <UInput
              v-model="compareCustomFrom"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div v-if="compareTimeRange === 'custom'">
            <UFormLabel class="text-xs">对比结束时间</UFormLabel>
            <UInput
              v-model="compareCustomTo"
              type="datetime-local"
              size="sm"
            />
          </div>
        </div>
      </div>
      <div class="flex gap-2 items-end flex-wrap">
        <div class="flex items-center gap-2 pb-1">
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              v-model="compareMode"
              class="sr-only peer"
              @change="onCompareModeChange"
            >
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
          <span class="text-sm text-gray-600 dark:text-gray-300">对比模式</span>
        </div>
        <UButton size="sm" variant="outline" @click="resetFilters">重置</UButton>
        <UButton size="sm" @click="loadData">查询</UButton>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-20">
      <Icon name="i-heroicons-arrow-path-16-solid" class="w-8 h-8 animate-spin text-blue-500" />
      <span class="ml-3 text-gray-500">加载中...</span>
    </div>

    <div v-else-if="!hasData" class="py-16 text-center text-gray-400">
      <Icon name="i-heroicons-chart-bar-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
      <p>暂无分布数据</p>
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="lg:col-span-2 bg-gray-50 dark:bg-gray-900/30 rounded-lg p-4">
        <h4 class="text-sm font-semibold mb-4 text-gray-700 dark:text-gray-300">耗时分布</h4>
        <div class="h-64">
          <Bar v-if="chartData" :data="chartData" :options="chartOptions" />
        </div>
      </div>

      <div class="bg-gray-50 dark:bg-gray-900/30 rounded-lg p-4">
        <h4 class="text-sm font-semibold mb-4 text-gray-700 dark:text-gray-300">统计指标</h4>
        <div v-if="!compareMode" class="space-y-3">
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">样本总数</span>
            <span class="font-semibold">{{ formatNumber(data?.total_count) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">平均值</span>
            <span class="font-semibold font-mono">{{ formatDuration(data?.avg_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P50 (中位数)</span>
            <span class="font-semibold font-mono text-blue-600 dark:text-blue-400">{{ formatDuration(data?.p50_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P90</span>
            <span class="font-semibold font-mono text-amber-600 dark:text-amber-400">{{ formatDuration(data?.p90_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P95</span>
            <span class="font-semibold font-mono text-orange-600 dark:text-orange-400">{{ formatDuration(data?.p95_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2">
            <span class="text-sm text-gray-600 dark:text-gray-400">P99</span>
            <span class="font-semibold font-mono text-red-600 dark:text-red-400">{{ formatDuration(data?.p99_ms) }}</span>
          </div>
        </div>
        <div v-else class="space-y-3">
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400"></span>
            <span class="text-sm font-medium text-blue-600 dark:text-blue-400 text-right">时段1</span>
            <span class="text-sm font-medium text-orange-600 dark:text-orange-400 text-right">时段2</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">样本总数</span>
            <span class="font-semibold text-right">{{ formatNumber(compareData?.first?.total_count) }}</span>
            <span class="font-semibold text-right">{{ formatNumber(compareData?.second?.total_count) }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">平均值</span>
            <span class="font-semibold font-mono text-right">{{ formatDuration(compareData?.first?.avg_ms) }}</span>
            <span class="font-semibold font-mono text-right">{{ formatDuration(compareData?.second?.avg_ms) }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">P50</span>
            <span class="font-semibold font-mono text-right text-blue-600 dark:text-blue-400">{{ formatDuration(compareData?.first?.p50_ms) }}</span>
            <span class="font-semibold font-mono text-right text-blue-600 dark:text-blue-400">{{ formatDuration(compareData?.second?.p50_ms) }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">P90</span>
            <span class="font-semibold font-mono text-right text-amber-600 dark:text-amber-400">{{ formatDuration(compareData?.first?.p90_ms) }}</span>
            <span class="font-semibold font-mono text-right text-amber-600 dark:text-amber-400">{{ formatDuration(compareData?.second?.p90_ms) }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 border-b border-gray-200 dark:border-gray-700 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">P95</span>
            <span class="font-semibold font-mono text-right text-orange-600 dark:text-orange-400">{{ formatDuration(compareData?.first?.p95_ms) }}</span>
            <span class="font-semibold font-mono text-right text-orange-600 dark:text-orange-400">{{ formatDuration(compareData?.second?.p95_ms) }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 py-2 items-center">
            <span class="text-sm text-gray-600 dark:text-gray-400">P99</span>
            <span class="font-semibold font-mono text-right text-red-600 dark:text-red-400">{{ formatDuration(compareData?.first?.p99_ms) }}</span>
            <span class="font-semibold font-mono text-right text-red-600 dark:text-red-400">{{ formatDuration(compareData?.second?.p99_ms) }}</span>
          </div>
        </div>
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
  useDurationHistogram,
  useDurationHistogramCompare,
  formatDuration,
  type DurationHistogramData,
  type DurationHistogramCompareData,
} from '~/composables/useApi'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const timeRangeOptions = [
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: '7d', label: '最近7天' },
  { value: 'custom', label: '自定义' },
]

const compareTimeRangeOptions = [
  { value: 'prev_period', label: '上一周期' },
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: '7d', label: '最近7天' },
  { value: 'custom', label: '自定义' },
]

const timeRange = ref('24h')
const customFrom = ref('')
const customTo = ref('')
const compareMode = ref(false)
const compareTimeRange = ref('prev_period')
const compareCustomFrom = ref('')
const compareCustomTo = ref('')
const taskType = ref('')
const data = ref<DurationHistogramData | null>(null)
const compareData = ref<DurationHistogramCompareData | null>(null)
const loading = ref(false)

const hasData = computed(() => {
  if (compareMode.value) {
    return compareData.value && (compareData.value.first.total_count > 0 || compareData.value.second.total_count > 0)
  }
  return data.value && data.value.total_count > 0
})

const chartData = computed(() => {
  if (compareMode.value && compareData.value) {
    const first = compareData.value.first
    const second = compareData.value.second
    const labels = (first?.buckets || []).map(b => b.range)
    return {
      labels,
      datasets: [
        {
          label: `时段1: ${formatDateRange(first?.time_from || '', first?.time_to || '')}`,
          data: (first?.buckets || []).map(b => b.count),
          backgroundColor: 'rgba(59, 130, 246, 0.8)',
          borderRadius: 4,
          barPercentage: 0.8,
          categoryPercentage: 0.8,
        },
        {
          label: `时段2: ${formatDateRange(second?.time_from || '', second?.time_to || '')}`,
          data: (second?.buckets || []).map(b => b.count),
          backgroundColor: 'rgba(249, 115, 22, 0.6)',
          borderRadius: 4,
          barPercentage: 0.8,
          categoryPercentage: 0.8,
        },
      ],
    }
  }

  if (!data.value || !data.value.buckets) return null
  return {
    labels: data.value.buckets.map(b => b.range),
    datasets: [
      {
        label: '任务数量',
        data: data.value.buckets.map(b => b.count),
        backgroundColor: [
          'rgba(34, 197, 94, 0.8)',
          'rgba(132, 204, 22, 0.8)',
          'rgba(234, 179, 8, 0.8)',
          'rgba(249, 115, 22, 0.8)',
          'rgba(239, 68, 68, 0.8)',
          'rgba(127, 29, 29, 0.8)',
        ],
        borderRadius: 4,
      },
    ],
  }
})

const chartOptions = computed(() => {
  const isCompare = compareMode.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: isCompare,
        position: 'top' as const,
        labels: {
          boxWidth: 12,
          padding: 15,
          font: { size: 11 },
        },
      },
      tooltip: {
        callbacks: {
          label: (ctx: any) => {
            const datasetIndex = ctx.datasetIndex
            if (isCompare && compareData.value) {
              const d = datasetIndex === 0 ? compareData.value.first : compareData.value.second
              const bucket = d?.buckets?.[ctx.dataIndex]
              if (!bucket) return ''
              return [
                `${ctx.dataset.label || ''}`,
                `数量: ${formatNumber(bucket.count)}`,
                `占比: ${(bucket.percentage || 0).toFixed(2)}%`,
              ]
            }
            const bucket = data.value?.buckets?.[ctx.dataIndex]
            if (!bucket) return ''
            return [
              `数量: ${formatNumber(bucket.count)}`,
              `占比: ${(bucket.percentage || 0).toFixed(2)}%`,
            ]
          },
        },
      },
    },
    scales: {
      y: {
        beginAtZero: true,
        ticks: {
          callback: (val: any) => {
            if (val >= 1000000) return (val / 1000000).toFixed(1) + 'M'
            if (val >= 1000) return (val / 1000).toFixed(1) + 'K'
            return val
          },
        },
      },
      x: {
        grid: { display: false },
      },
    },
  }
})

function formatDateRange(from: string, to: string): string {
  const f = new Date(from)
  const t = new Date(to)
  const format = (d: Date) => `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  return `${format(f)} - ${format(t)}`
}

function formatNumber(n?: number | string | null): string {
  if (n === null || n === undefined || n === '') return '-'
  const num = Number(n)
  if (isNaN(num)) return '-'
  return num.toLocaleString()
}

function getTimeRange() {
  const now = new Date()
  let from: Date
  switch (timeRange.value) {
    case '1h':
      from = new Date(now.getTime() - 3600 * 1000)
      break
    case '6h':
      from = new Date(now.getTime() - 6 * 3600 * 1000)
      break
    case '24h':
      from = new Date(now.getTime() - 24 * 3600 * 1000)
      break
    case '7d':
      from = new Date(now.getTime() - 7 * 24 * 3600 * 1000)
      break
    case 'custom':
      from = customFrom.value ? new Date(customFrom.value) : new Date(now.getTime() - 3600 * 1000)
      const to = customTo.value ? new Date(customTo.value) : now
      return { from, to }
    default:
      from = new Date(now.getTime() - 3600 * 1000)
  }
  return { from, to: now }
}

function getCompareTimeRange() {
  const now = new Date()
  const { from: firstFrom, to: firstTo } = getTimeRange()
  const durationMs = firstTo.getTime() - firstFrom.getTime()

  if (compareTimeRange.value === 'prev_period') {
    const prevTo = new Date(firstFrom.getTime())
    const prevFrom = new Date(firstFrom.getTime() - durationMs)
    return { from: prevFrom, to: prevTo }
  }

  let from: Date
  switch (compareTimeRange.value) {
    case '1h':
      from = new Date(now.getTime() - 3600 * 1000)
      break
    case '6h':
      from = new Date(now.getTime() - 6 * 3600 * 1000)
      break
    case '24h':
      from = new Date(now.getTime() - 24 * 3600 * 1000)
      break
    case '7d':
      from = new Date(now.getTime() - 7 * 24 * 3600 * 1000)
      break
    case 'custom':
      from = compareCustomFrom.value ? new Date(compareCustomFrom.value) : new Date(now.getTime() - 3600 * 1000)
      const to = compareCustomTo.value ? new Date(compareCustomTo.value) : now
      return { from, to }
    default:
      from = new Date(now.getTime() - 3600 * 1000)
  }
  return { from, to: now }
}

function resetFilters() {
  timeRange.value = '24h'
  customFrom.value = ''
  customTo.value = ''
  compareMode.value = false
  compareTimeRange.value = 'prev_period'
  compareCustomFrom.value = ''
  compareCustomTo.value = ''
  taskType.value = ''
  loadData()
}

function onCompareModeChange() {
  if (!compareMode.value) {
    loadData()
  }
}

async function loadData() {
  loading.value = true
  try {
    const { from, to } = getTimeRange()
    const params: Record<string, any> = {
      from: from.toISOString(),
      to: to.toISOString(),
    }
    if (taskType.value) {
      params.type = taskType.value
    }

    if (compareMode.value) {
      const { from: compareFrom, to: compareTo } = getCompareTimeRange()
      params.compare_from = compareFrom.toISOString()
      params.compare_to = compareTo.toISOString()

      const { data: result } = await useDurationHistogramCompare(params)
      if (result.value) {
        compareData.value = result.value
        data.value = null
      }
    } else {
      const { data: result } = await useDurationHistogram(params)
      if (result.value) {
        data.value = result.value
        compareData.value = null
      }
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>
