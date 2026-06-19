<template>
  <div class="overflow-x-auto">
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
              style="min-width: 140px"
            >
              <div class="font-semibold">{{ date }} {{ hour.toString().padStart(2, '0') }}:00</div>
              <div class="mt-1 space-y-0.5">
                <div>P50: {{ formatDuration(getCell(hour, di)?.p50_ms) }}</div>
                <div>P95: {{ formatDuration(getCell(hour, di)?.p95_ms) }}</div>
                <div>P99: {{ formatDuration(getCell(hour, di)?.p99_ms) }}</div>
                <div class="text-gray-400">样本数: {{ getCell(hour, di)?.sample_size || 0 }}</div>
              </div>
              <div v-if="showCompare" class="mt-2 pt-2 border-t border-gray-700">
                <div class="text-gray-400 mb-1">环比变化:</div>
                <div :class="getChangeClass(getCell(hour, di)?.p95_ms || 0, getCompareCell(hour, di)?.p95_ms)">
                  P95: {{ calcChangePercent(getCell(hour, di)?.p95_ms || 0, getCompareCell(hour, di)?.p95_ms) }}
                </div>
              </div>
              <div v-if="getCell(hour, di)?.is_anomaly" class="mt-2 pt-2 border-t border-gray-700">
                <div class="text-red-400 font-medium">⚠ 异常点</div>
              </div>
              <div class="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { formatDuration, type DurationHeatmapData, type DurationHeatmapCell } from '~/composables/useApi'

interface Props {
  data: DurationHeatmapData
  compareData?: DurationHeatmapData
  showCompare?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showCompare: false,
})

const hoverCell = ref<{ hour: number; di: number } | null>(null)

function getCell(hour: number, di: number): DurationHeatmapCell | null {
  const row = props.data.matrix[hour]
  if (!row) return null
  return row[di] || null
}

function getCompareCell(hour: number, di: number): DurationHeatmapCell | null {
  if (!props.compareData || !props.compareData.matrix) return null
  const row = props.compareData.matrix[hour]
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
  let anomalyClass = ''
  if (cell.is_anomaly) {
    anomalyClass = 'ring-2 ring-red-500 ring-offset-0 border-2 border-dashed border-red-500'
  }
  return `${color} ${anomalyClass} m-px rounded-sm transition-transform group-hover:scale-110`
}

function formatDateLabel(date: string): string {
  const d = new Date(date)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function calcChangePercent(current: number, previous: number | null | undefined): string {
  if (previous === null || previous === undefined || previous === 0) {
    return '无对比基准'
  }
  const change = ((current - previous) / previous) * 100
  const sign = change >= 0 ? '+' : ''
  return `${sign}${change.toFixed(1)}%`
}

function getChangeClass(current: number, previous: number | null | undefined): string {
  if (previous === null || previous === undefined || previous === 0) {
    return 'text-gray-400'
  }
  const change = ((current - previous) / previous) * 100
  if (change > 0) return 'text-red-400'
  if (change < 0) return 'text-green-400'
  return 'text-gray-400'
}
</script>
