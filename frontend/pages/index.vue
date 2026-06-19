<template>
  <div class="space-y-8">
    <section class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      <div
        v-for="card in statCards"
        :key="card.label"
        class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm"
      >
        <div class="flex items-start justify-between">
          <div>
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
            <p class="text-3xl font-bold mt-2" :class="card.valueClass">{{ formatNumber(card.value) }}</p>
            <p v-if="card.hint" class="text-xs text-gray-400 mt-1">{{ card.hint }}</p>
          </div>
          <div
            class="w-12 h-12 rounded-lg flex items-center justify-center"
            :class="card.iconBg"
          >
            <Icon :name="card.icon" class="w-6 h-6" :class="card.iconClass" />
          </div>
        </div>
      </div>
    </section>

    <section class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center justify-between mb-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Queue Depths</h3>
          <UTooltip :content="`Last updated: ${formatDate(snapshot?.timestamp)}`">
            <Icon name="i-heroicons-information-circle-16-solid" class="w-5 h-5 text-gray-400" />
          </UTooltip>
        </div>
        <div class="space-y-4">
          <div
            v-for="level in priorityLevels"
            :key="level.key"
            class="space-y-1.5"
          >
            <div class="flex items-center justify-between text-sm">
              <span class="flex items-center gap-2">
                <span
                  class="w-2 h-2 rounded-full"
                  :style="{ backgroundColor: level.color }"
                ></span>
                <span class="font-medium text-gray-700 dark:text-gray-300">{{ level.label }}</span>
              </span>
              <span class="font-bold text-gray-900 dark:text-white">{{ formatNumber(snapshot?.queue_depths?.[level.key] || 0) }}</span>
            </div>
            <div class="h-3 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :style="{
                  width: `${depthPercent(level.key)}%`,
                  backgroundColor: level.color
                }"
              ></div>
            </div>
          </div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center justify-between mb-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Worker Cluster Status</h3>
          <NuxtLink to="/workers" class="text-sm text-blue-600 hover:underline">
            View all →
          </NuxtLink>
        </div>
        <div class="grid grid-cols-3 gap-4 mb-6">
          <div class="text-center p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
            <p class="text-3xl font-bold text-green-600 dark:text-green-400">{{ snapshot?.workers_online || 0 }}</p>
            <p class="text-xs text-green-700 dark:text-green-300 mt-1">Online</p>
          </div>
          <div class="text-center p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
            <p class="text-3xl font-bold text-red-600 dark:text-red-400">{{ snapshot?.workers_offline || 0 }}</p>
            <p class="text-xs text-red-700 dark:text-red-300 mt-1">Offline</p>
          </div>
          <div class="text-center p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
            <p class="text-3xl font-bold text-blue-600 dark:text-blue-400">{{ snapshot?.workers_total || 0 }}</p>
            <p class="text-xs text-blue-700 dark:text-blue-300 mt-1">Total</p>
          </div>
        </div>
        <div class="space-y-2">
          <div class="flex items-center justify-between text-sm">
            <span class="text-gray-600 dark:text-gray-400">Cluster Utilization</span>
            <span class="font-semibold">{{ (snapshot?.worker_utilization || 0).toFixed(1) }}%</span>
          </div>
          <div class="h-2 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden">
            <div
              class="h-full bg-gradient-to-r from-blue-500 to-purple-500 rounded-full transition-all"
              :style="{ width: `${Math.min(snapshot?.worker_utilization || 0, 100)}%` }"
            ></div>
          </div>
        </div>
      </div>
    </section>

    <section class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="lg:col-span-2 bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center justify-between mb-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Task Throughput (24h)</h3>
          <div class="flex items-center gap-2 text-sm">
            <span class="text-gray-500">Current Rate:</span>
            <span class="font-bold text-blue-600">{{ (snapshot?.throughput || 0).toFixed(2) }} tasks/s</span>
          </div>
        </div>
        <div class="h-64">
          <Line v-if="throughputChartData" :data="throughputChartData" :options="chartOptions" />
          <div v-else class="h-full flex items-center justify-center text-gray-400">
            Loading throughput data...
          </div>
        </div>
      </div>

      <div class="space-y-6">
        <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-6">Success / Failure Rates</h3>
          <div class="space-y-4">
            <div
              v-for="level in priorityLevels"
              :key="`rate-${level.key}`"
              class="p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
            >
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium" :style="{ color: level.color }">{{ level.label }}</span>
                <span class="text-xs text-gray-500">
                  {{ (snapshot?.success_rates?.[level.key] || 0).toFixed(1) }}%
                </span>
              </div>
              <div class="h-2 flex rounded-full overflow-hidden bg-gray-200 dark:bg-gray-600">
                <div
                  class="h-full bg-green-500"
                  :style="{ width: `${snapshot?.success_rates?.[level.key] || 0}%` }"
                ></div>
                <div
                  class="h-full bg-red-500"
                  :style="{ width: `${snapshot?.failure_rates?.[level.key] || 0}%` }"
                ></div>
              </div>
            </div>
          </div>

          <div class="mt-6 pt-6 border-t border-gray-200 dark:border-gray-700">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-lg bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
                  <Icon name="i-heroicons-no-symbol-16-solid" class="w-5 h-5 text-red-600" />
                </div>
                <div>
                  <p class="text-sm text-gray-500">Dead Letters</p>
                  <p class="text-xl font-bold text-red-600">{{ formatNumber(snapshot?.dead_letter_count || 0) }}</p>
                </div>
              </div>
              <NuxtLink to="/dead-letter">
                <UButton size="sm" variant="outline" color="red">Manage</UButton>
              </NuxtLink>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Rate Limit Triggers (1h)</h3>
            <NuxtLink to="/rate-limit" class="text-sm text-blue-600 hover:underline">
              Configure →
            </NuxtLink>
          </div>

          <div v-if="throttleEntries.length > 0" class="space-y-3">
            <div
              v-for="entry in throttleEntries"
              :key="entry.taskType"
              class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
            >
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-lg bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
                  <Icon name="i-heroicons-gauge-high-16-solid" class="w-4 h-4 text-red-600" />
                </div>
                <code class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ entry.taskType }}</code>
              </div>
              <span class="font-bold text-red-600 dark:text-red-400">{{ formatNumber(entry.count) }}</span>
            </div>
          </div>

          <div v-else class="py-6 text-center text-gray-500">
            <Icon name="i-heroicons-check-circle-16-solid" class="w-8 h-8 mx-auto mb-2 text-green-500" />
            <p class="text-sm">No rate limit triggers</p>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const priorityLevels = [
  { key: '4', label: 'Critical', color: '#ef4444' },
  { key: '3', label: 'High', color: '#f97316' },
  { key: '2', label: 'Normal', color: '#22c55e' },
  { key: '1', label: 'Low', color: '#3b82f6' },
  { key: '0', label: 'Bulk', color: '#6b7280' },
]

const snapshot = ref<any>(null)
const throughputHistory = ref<Record<string, number>>({})
const refreshInterval = ref<NodeJS.Timeout | null>(null)

const throughputChartData = computed(() => {
  if (!throughputHistory.value || Object.keys(throughputHistory.value).length === 0) {
    return null
  }
  const entries = Object.entries(throughputHistory.value)
    .map(([ts, val]) => ({ ts: parseInt(ts), val }))
    .sort((a, b) => a.ts - b.ts)
    .slice(-96)

  return {
    labels: entries.map(e => {
      const d = new Date(e.ts * 1000)
      return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
    }),
    datasets: [{
      label: 'Tasks/sec',
      data: entries.map(e => e.val),
      borderColor: '#3b82f6',
      backgroundColor: 'rgba(59, 130, 246, 0.1)',
      fill: true,
      tension: 0.4,
      pointRadius: 0,
      borderWidth: 2,
    }]
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { maxTicksLimit: 12 },
    },
    y: {
      beginAtZero: true,
      grid: { color: 'rgba(0,0,0,0.05)' },
    },
  },
}

const statCards = computed(() => [
  {
    label: 'Total Ready Tasks',
    value: priorityLevels.reduce((sum, l) => sum + (snapshot.value?.queue_depths?.[l.key] || 0), 0),
    icon: 'i-heroicons-inbox-stack-16-solid',
    iconBg: 'bg-blue-100 dark:bg-blue-900/30',
    iconClass: 'text-blue-600',
    valueClass: 'text-gray-900 dark:text-white',
    hint: 'Across all priority queues',
  },
  {
    label: 'Throughput',
    value: parseFloat((snapshot.value?.throughput || 0).toFixed(2)),
    icon: 'i-heroicons-bolt-16-solid',
    iconBg: 'bg-amber-100 dark:bg-amber-900/30',
    iconClass: 'text-amber-600',
    valueClass: 'text-amber-600',
    hint: 'Tasks processed per second',
  },
  {
    label: 'Avg Latency',
    value: parseFloat((snapshot.value?.avg_latency_ms || 0).toFixed(0)),
    icon: 'i-heroicons-clock-16-solid',
    iconBg: 'bg-purple-100 dark:bg-purple-900/30',
    iconClass: 'text-purple-600',
    valueClass: 'text-purple-600',
    hint: 'Milliseconds per task',
  },
  {
    label: 'Worker Utilization',
    value: parseFloat((snapshot.value?.worker_utilization || 0).toFixed(1)),
    icon: 'i-heroicons-server-stack-16-solid',
    iconBg: 'bg-green-100 dark:bg-green-900/30',
    iconClass: 'text-green-600',
    valueClass: 'text-green-600',
    hint: '% of total slots in use',
  },
])

const throttleEntries = computed(() => {
  const counts = snapshot.value?.throttle_counts
  if (!counts || typeof counts !== 'object') return []
  return Object.entries(counts).map(([taskType, count]) => ({
    taskType,
    count: Number(count),
  }))
})

function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return n.toString()
}

function formatDate(ts?: string | number): string {
  if (!ts) return 'N/A'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? 'N/A' : d.toLocaleString()
}

function depthPercent(key: string): number {
  const max = Math.max(...priorityLevels.map(l => snapshot.value?.queue_depths?.[l.key] || 0), 1)
  return ((snapshot.value?.queue_depths?.[key] || 0) / max) * 100
}

async function loadData() {
  try {
    const { data: snap } = await useMetricsSnapshot()
    if (snap.value) snapshot.value = snap.value

    const { data: tp } = await useThroughputHistory(24)
    if (tp.value) throughputHistory.value = tp.value as Record<string, number>
  } catch (e) {
    // ignore
  }
}

onMounted(() => {
  loadData()
  refreshInterval.value = setInterval(loadData, 5000)
})

onUnmounted(() => {
  if (refreshInterval.value) clearInterval(refreshInterval.value)
})
</script>
