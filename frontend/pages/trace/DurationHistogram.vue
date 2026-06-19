<template>
  <div class="space-y-4">
    <div class="flex flex-col md:flex-row gap-4 md:items-end">
      <div class="flex-1 grid grid-cols-1 md:grid-cols-3 gap-4">
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
      <div class="flex gap-2">
        <UButton size="sm" variant="outline" @click="resetFilters">重置</UButton>
        <UButton size="sm" @click="loadData">查询</UButton>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-20">
      <Icon name="i-heroicons-arrow-path-16-solid" class="w-8 h-8 animate-spin text-blue-500" />
      <span class="ml-3 text-gray-500">加载中...</span>
    </div>

    <div v-else-if="!data || data.total_count === 0" class="py-16 text-center text-gray-400">
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
        <div class="space-y-3">
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">样本总数</span>
            <span class="font-semibold">{{ data.total_count.toLocaleString() }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">平均值</span>
            <span class="font-semibold font-mono">{{ formatDuration(data.avg_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P50 (中位数)</span>
            <span class="font-semibold font-mono text-blue-600 dark:text-blue-400">{{ formatDuration(data.p50_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P90</span>
            <span class="font-semibold font-mono text-amber-600 dark:text-amber-400">{{ formatDuration(data.p90_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2 border-b border-gray-200 dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">P95</span>
            <span class="font-semibold font-mono text-orange-600 dark:text-orange-400">{{ formatDuration(data.p95_ms) }}</span>
          </div>
          <div class="flex justify-between items-center py-2">
            <span class="text-sm text-gray-600 dark:text-gray-400">P99</span>
            <span class="font-semibold font-mono text-red-600 dark:text-red-400">{{ formatDuration(data.p99_ms) }}</span>
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
import { useDurationHistogram, formatDuration, type DurationHistogramData } from '~/composables/useApi'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const timeRangeOptions = [
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: '7d', label: '最近7天' },
  { value: 'custom', label: '自定义' },
]

const timeRange = ref('24h')
const customFrom = ref('')
const customTo = ref('')
const taskType = ref('')
const data = ref<DurationHistogramData | null>(null)
const loading = ref(false)

const chartData = computed(() => {
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

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (ctx: any) => {
          const bucket = data.value?.buckets[ctx.dataIndex]
          if (!bucket) return ''
          return [
            `数量: ${bucket.count.toLocaleString()}`,
            `占比: ${bucket.percentage.toFixed(2)}%`,
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

function resetFilters() {
  timeRange.value = '24h'
  customFrom.value = ''
  customTo.value = ''
  taskType.value = ''
  loadData()
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
    const { data: result } = await useDurationHistogram(params)
    if (result.value) {
      data.value = result.value
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>
