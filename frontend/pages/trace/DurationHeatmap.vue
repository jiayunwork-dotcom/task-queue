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
      <div class="flex gap-2 items-end flex-wrap">
        <div class="flex items-center gap-2 pb-1">
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              v-model="compareMode"
              class="sr-only peer"
              @change="loadData"
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
      <p>暂无热力图数据</p>
    </div>

    <div v-else class="space-y-8">
      <div v-if="compareMode && compareData" class="space-y-2">
        <div class="flex items-center gap-2">
          <span class="inline-block w-3 h-3 bg-blue-500 rounded-sm"></span>
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">当前周期</span>
          <span class="text-xs text-gray-500">(最近{{ days }}天)</span>
        </div>
        <HeatmapGrid
          :data="compareData.current"
          :compare-data="compareData.previous"
          :show-compare="true"
        />
      </div>

      <div v-if="compareMode && compareData" class="space-y-2">
        <div class="flex items-center gap-2">
          <span class="inline-block w-3 h-3 bg-gray-400 rounded-sm"></span>
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">上一周期</span>
          <span class="text-xs text-gray-500">(前{{ days }}天)</span>
        </div>
        <HeatmapGrid
          :data="compareData.previous"
          :show-compare="false"
        />
      </div>

      <div v-if="!compareMode && data">
        <HeatmapGrid
          :data="data"
          :show-compare="false"
        />
      </div>

      <div class="flex items-center justify-between flex-wrap gap-4">
        <div class="flex items-center gap-4">
          <div class="flex items-center gap-2">
            <div class="w-4 h-4 border-2 border-red-500 border-dashed rounded-sm"></div>
            <span class="text-xs text-gray-500">异常点</span>
          </div>
        </div>
        <div class="flex items-center gap-3 flex-wrap">
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
          <span class="text-xs text-gray-400">| 总样本数: {{ totalSamples.toLocaleString() }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  useDurationHeatmap,
  useDurationHeatmapCompare,
  formatDuration,
  type DurationHeatmapData,
  type DurationHeatmapCompareData,
  type DurationHeatmapCell,
} from '~/composables/useApi'
import HeatmapGrid from './HeatmapGrid.vue'

const dayOptions = [
  { value: 7, label: '最近7天' },
  { value: 14, label: '最近14天' },
  { value: 30, label: '最近30天' },
]

const taskType = ref('')
const days = ref(7)
const compareMode = ref(false)
const data = ref<DurationHeatmapData | null>(null)
const compareData = ref<DurationHeatmapCompareData | null>(null)
const loading = ref(false)

const legendColors = [
  '#22c55e',
  '#84cc16',
  '#eab308',
  '#f97316',
  '#ef4444',
]

const hasData = computed(() => {
  if (compareMode.value) {
    if (!compareData.value) return false
    return hasSamples(compareData.value.current) || hasSamples(compareData.value.previous)
  }
  return data.value && hasSamples(data.value)
})

function hasSamples(d: DurationHeatmapData | undefined | null): boolean {
  if (!d || !d.matrix) return false
  for (const row of d.matrix) {
    if (!row) continue
    for (const cell of row) {
      if (cell && cell.sample_size > 0) return true
    }
  }
  return false
}

const totalSamples = computed(() => {
  const d = compareMode.value ? compareData.value?.current : data.value
  if (!d || !d.matrix) return 0
  let total = 0
  for (const row of d.matrix) {
    if (!row) continue
    for (const cell of row) {
      if (cell) {
        total += cell.sample_size || 0
      }
    }
  }
  return total
})

function resetFilters() {
  taskType.value = ''
  days.value = 7
  compareMode.value = false
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

    if (compareMode.value) {
      const { data: result } = await useDurationHeatmapCompare(params)
      if (result.value) {
        compareData.value = result.value
        data.value = null
      }
    } else {
      const { data: result } = await useDurationHeatmap(params)
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
