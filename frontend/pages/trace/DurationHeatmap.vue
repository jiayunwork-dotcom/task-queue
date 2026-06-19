<template>
  <div class="space-y-4">
    <div class="flex flex-col md:flex-row gap-4 md:items-end">
      <div class="flex-1 grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <UFormLabel class="text-xs">任务类型</UFormLabel>
          <UInput
            v-model="taskType"
            placeholder="所有类型"
            size="sm"
            @keyup.enter="loadData"
          />
        </div>
        <div>
          <UFormLabel class="text-xs">时间范围</UFormLabel>
          <USelect
            v-model="days"
            :options="dayOptions"
            size="sm"
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

    <div v-else-if="!data || totalSamples === 0" class="py-16 text-center text-gray-400">
      <Icon name="i-heroicons-chart-bar-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
      <p>暂无热力图数据</p>
    </div>

    <div v-else class="overflow-x-auto">
      <div class="inline-block min-w-full">
        <div class="flex">
          <div class="w-16 flex-shrink-0"></div>
          <div class="flex-1 flex">
            <div
              v-for="date in data.dates"
              :key="date"
              class="flex-1 text-center text-xs text-gray-500 pb-2"
            >
              {{ formatDateLabel(date) }}
            </div>
          </div>
        </div>

        <div
          v-for="hour in data.hours"
          :key="hour"
          class="flex items-center"
        >
          <div class="w-16 flex-shrink-0 text-xs text-gray-500 text-right pr-2">
            {{ hour.toString().padStart(2, '0') }}:00
          </div>
          <div class="flex-1 flex">
            <div
              v-for="(date, di) in data.dates"
              :key="`${hour}-${date}`"
              class="flex-1 aspect-square relative cursor-pointer group"
              :class="getCellClass(hour, di)"
              @mouseenter="hoverCell = { hour, di }"
              @mouseleave="hoverCell = null"
            >
              <div
                v-if="hoverCell?.hour === hour && hoverCell?.di === di"
                class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-10 bg-gray-900 text-white text-xs rounded px-3 py-2 whitespace-nowrap shadow-lg"
              >
                <div class="font-semibold">{{ date }} {{ hour.toString().padStart(2, '0') }}:00</div>
                <div class="mt-1 space-y-0.5">
                  <div>P50: {{ formatDuration(getCell(hour, di)?.p50_ms) }}</div>
                  <div>P95: {{ formatDuration(getCell(hour, di)?.p95_ms) }}</div>
                  <div>P99: {{ formatDuration(getCell(hour, di)?.p99_ms) }}</div>
                  <div class="text-gray-400">样本数: {{ getCell(hour, di)?.sample_size || 0 }}</div>
                </div>
                <div class="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
              </div>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-end mt-4 gap-3">
          <span class="text-xs text-gray-500">P95耗时:</span>
          <div class="flex items-center gap-1">
            <span class="text-xs text-gray-400">0ms</span>
            <div class="flex">
              <div
                v-for="(color, idx) in legendColors"
                :key="idx"
                class="w-6 h-4"
                :style="{ backgroundColor: color }"
              ></div>
            </div>
            <span class="text-xs text-gray-400">10s+</span>
          </div>
          <span class="text-xs text-gray-400">| 总样本数: {{ totalSamples }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useDurationHeatmap, formatDuration, type DurationHeatmapData } from '~/composables/useApi'

const dayOptions = [
  { value: 7, label: '最近7天' },
  { value: 14, label: '最近14天' },
  { value: 30, label: '最近30天' },
]

const taskType = ref('')
const days = ref(7)
const data = ref<DurationHeatmapData | null>(null)
const loading = ref(false)
const hoverCell = ref<{ hour: number; di: number } | null>(null)

const legendColors = [
  '#22c55e',
  '#84cc16',
  '#eab308',
  '#f97316',
  '#ef4444',
]

function getCell(hour: number, di: number) {
  if (!data.value || !data.value.matrix) return null
  const row = data.value.matrix[hour]
  if (!row) return null
  return row[di] || null
}

function getCellClass(hour: number, di: number): string {
  const cell = getCell(hour, di)
  if (!cell) {
    return 'bg-gray-200 dark:bg-gray-700 m-px rounded-sm'
  }
  const p95 = cell.p95_ms
  let color = 'bg-gray-300 dark:bg-gray-600'
  if (p95 < 200) {
    color = 'bg-green-500'
  } else if (p95 < 1000) {
    color = 'bg-yellow-400'
  } else if (p95 < 5000) {
    color = 'bg-orange-500'
  } else {
    color = 'bg-red-500'
  }
  return `${color} m-px rounded-sm transition-transform group-hover:scale-110`
}

const totalSamples = computed(() => {
  if (!data.value || !data.value.matrix) return 0
  let total = 0
  for (const row of data.value.matrix) {
    if (!row) continue
    for (const cell of row) {
      if (cell) {
        total += cell.sample_size || 0
      }
    }
  }
  return total
})

function formatDateLabel(date: string): string {
  const d = new Date(date)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function resetFilters() {
  taskType.value = ''
  days.value = 7
  loadData()
}

async function loadData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      days: days.value,
    }
    if (taskType.value) {
      params.type = taskType.value
    }
    const { data: result } = await useDurationHeatmap(params)
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
