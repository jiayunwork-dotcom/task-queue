<template>
  <div class="space-y-6">
    <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
      <div class="lg:col-span-3 bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold">Dead Letter Queue</h3>
          <div class="flex gap-2">
            <UButton
              size="sm"
              variant="outline"
              color="green"
              :disabled="selected.size === 0"
              @click="handleBatchRetry"
            >
              <Icon name="i-heroicons-arrow-path-16-solid" class="w-4 h-4 mr-1" />
              Batch Retry ({{ selected.size }})
            </UButton>
            <UButton
              size="sm"
              variant="outline"
              color="red"
              :disabled="selected.size === 0"
              @click="handleBatchDiscard"
            >
              <Icon name="i-heroicons-trash-16-solid" class="w-4 h-4 mr-1" />
              Batch Discard ({{ selected.size }})
            </UButton>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase w-10">
                  <input type="checkbox" class="rounded" @change="toggleAll" :checked="allSelected" />
                </th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Task ID</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Type</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Priority</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Retries</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Reason</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Failed At</th>
                <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <tr
                v-for="task in tasks"
                :key="task.id"
                class="hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer"
                @click="openDetail(task.id)"
              >
                <td class="px-4 py-3" @click.stop>
                  <input
                    type="checkbox"
                    class="rounded"
                    :checked="selected.has(task.id)"
                    @change="toggleOne(task.id)"
                  />
                </td>
                <td class="px-4 py-3">
                  <span class="font-mono text-xs text-gray-600">{{ task.id.slice(0, 8) }}...</span>
                </td>
                <td class="px-4 py-3 font-medium">{{ task.type }}</td>
                <td class="px-4 py-3">
                  <UBadge :color="priorityColor(task.priority)" size="xs">{{ priorityLabel(task.priority) }}</UBadge>
                </td>
                <td class="px-4 py-3">
                  <span class="text-sm font-bold text-red-600">{{ task.retry_count }}</span>
                </td>
                <td class="px-4 py-3 max-w-[200px]">
                  <span class="text-xs text-red-600 truncate block" :title="task.last_error">{{ task.last_error || '-' }}</span>
                </td>
                <td class="px-4 py-3 text-sm text-gray-500">{{ formatDate(task.completed_at || task.updated_at) }}</td>
                <td class="px-4 py-3" @click.stop>
                  <div class="flex gap-1">
                    <UTooltip content="View details">
                      <UButton size="2xs" variant="ghost" icon="i-heroicons-eye-16-solid" @click="openDetail(task.id)" />
                    </UTooltip>
                    <UTooltip content="Retry">
                      <UButton size="2xs" variant="ghost" color="green" icon="i-heroicons-arrow-path-16-solid" @click="handleRetry(task.id)" />
                    </UTooltip>
                    <UTooltip content="Discard">
                      <UButton size="2xs" variant="ghost" color="red" icon="i-heroicons-trash-16-solid" @click="handleDiscard(task.id)" />
                    </UTooltip>
                  </div>
                </td>
              </tr>
              <tr v-if="tasks.length === 0">
                <td colspan="8" class="px-6 py-16 text-center text-gray-400">
                  <Icon name="i-heroicons-check-circle-20-solid" class="w-12 h-12 mx-auto mb-3 text-green-500 opacity-50" />
                  <p>No dead letters. All tasks are healthy.</p>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="mt-4 flex items-center justify-between">
          <p class="text-sm text-gray-500">Showing {{ tasks.length }} of {{ total }}</p>
          <div class="flex gap-2">
            <UButton size="sm" variant="outline" :disabled="offset === 0" @click="prevPage">Prev</UButton>
            <UButton size="sm" variant="outline" :disabled="offset + limit >= total" @click="nextPage">Next</UButton>
          </div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <h3 class="text-lg font-semibold mb-4">By Error Type</h3>
        <div class="h-48">
          <Doughnut v-if="pieChartData" :data="pieChartData" :options="pieOptions" />
          <div v-else class="h-full flex items-center justify-center text-gray-400 text-sm">No data</div>
        </div>
        <div class="mt-4 space-y-2 max-h-48 overflow-auto">
          <div
            v-for="(count, reason) in errorStats"
            :key="reason"
            class="flex items-center justify-between text-xs p-2 bg-gray-50 dark:bg-gray-700/30 rounded"
          >
            <span class="text-gray-600 dark:text-gray-300 truncate mr-2" :title="reason as string">
              {{ (reason as string).length > 30 ? (reason as string).slice(0, 30) + '...' : reason }}
            </span>
            <span class="font-bold text-gray-900 dark:text-white">{{ count }}</span>
          </div>
        </div>
      </div>
    </div>

    <UModal v-model="showDetail" class="w-[700px] max-w-[95vw]">
      <template #header>
        <h3 class="text-lg font-bold text-red-600">Dead Letter Details</h3>
      </template>

      <div v-if="detail" class="space-y-6 p-6">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-gray-500 mb-1">Task ID</p>
            <p class="font-mono text-sm break-all">{{ detail.task?.id }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Type</p>
            <p class="font-medium">{{ detail.task?.type }}</p>
          </div>
        </div>

        <div>
          <p class="text-xs text-gray-500 mb-2">Payload</p>
          <pre class="bg-gray-900 text-green-400 p-4 rounded-lg text-xs overflow-auto max-h-32">{{ JSON.stringify(detail.task?.payload, null, 2) }}</pre>
        </div>

        <div>
          <p class="text-sm font-semibold mb-2">Error History</p>
          <div class="space-y-2 max-h-48 overflow-auto">
            <div
              v-for="(err, idx) in (detail.error_history || [])"
              :key="idx"
              class="p-3 bg-red-50 dark:bg-red-900/20 rounded-lg border border-red-200 dark:border-red-800"
            >
              <pre class="text-xs text-red-700 dark:text-red-300 whitespace-pre-wrap">{{ err }}</pre>
            </div>
          </div>
        </div>

        <div>
          <p class="text-sm font-semibold mb-2">Execution Timeline</p>
          <div class="space-y-2 max-h-48 overflow-auto">
            <div
              v-for="exec in (detail.executions || [])"
              :key="exec.id"
              class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
            >
              <div>
                <span class="text-sm font-semibold">Attempt {{ exec.attempt }}</span>
                <UBadge :color="statusColor(exec.status)" size="xs" class="ml-2">{{ exec.status }}</UBadge>
              </div>
              <div class="text-right text-xs text-gray-500">
                {{ formatDate(exec.started_at) }}
                <span v-if="exec.duration_ms"> ({{ formatDuration(exec.duration_ms) }})</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex gap-2 justify-end">
          <UButton variant="outline" color="red" @click="handleDiscard(detail.task.id)">Discard</UButton>
          <UButton color="green" @click="handleRetry(detail.task.id)">Retry Now</UButton>
          <UButton variant="ghost" @click="showDetail = false">Close</UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  Chart as ChartJS,
  ArcElement,
  Tooltip,
  Legend,
} from 'chart.js'
import { Doughnut } from 'vue-chartjs'

ChartJS.register(ArcElement, Tooltip, Legend)

const limit = 50
const offset = ref(0)
const tasks = ref<any[]>([])
const total = ref(0)
const selected = ref<Set<string>>(new Set())
const errorStats = ref<Record<string, number>>({})
const showDetail = ref(false)
const detail = ref<any>(null)
const refreshTimer = ref<any>(null)

const colors = ['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#8b5cf6', '#ec4899', '#14b8a6', '#6366f1', '#f43f5e']

const pieChartData = computed(() => {
  const entries = Object.entries(errorStats.value)
  if (entries.length === 0) return null
  return {
    labels: entries.map(([k]) => k.length > 20 ? k.slice(0, 20) + '...' : k),
    datasets: [{
      data: entries.map(([, v]) => v),
      backgroundColor: entries.map((_, i) => colors[i % colors.length]),
    }]
  }
})

const pieOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
}

const allSelected = computed(() => tasks.value.length > 0 && tasks.value.every(t => selected.value.has(t.id)))

function toggleAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) tasks.value.forEach(t => selected.value.add(t.id))
  else selected.value.clear()
}
function toggleOne(id: string) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
}

async function loadDeadLetters() {
  const { data } = await useDeadLetters({ limit, offset: offset.value })
  if (data.value) {
    tasks.value = data.value.items || []
    total.value = data.value.total || 0
  }
  const { data: stats } = await useDeadLetterByError()
  if (stats.value) errorStats.value = stats.value as Record<string, number>
}

function prevPage() {
  if (offset.value >= limit) { offset.value -= limit; loadDeadLetters() }
}
function nextPage() {
  offset.value += limit; loadDeadLetters()
}

async function openDetail(id: string) {
  const { data } = await useDeadLetterDetail(id)
  detail.value = data.value
  showDetail.value = true
}

async function handleRetry(id: string) {
  try {
    await retryDeadLetter(id)
    selected.value.delete(id)
    if (showDetail.value) showDetail.value = false
    loadDeadLetters()
  } catch (e) { alert('Retry failed') }
}

async function handleDiscard(id: string) {
  if (!confirm('Permanently discard this task?')) return
  try {
    await discardDeadLetter(id)
    selected.value.delete(id)
    if (showDetail.value) showDetail.value = false
    loadDeadLetters()
  } catch (e) { alert('Discard failed') }
}

async function handleBatchRetry() {
  if (!confirm(`Retry ${selected.value.size} tasks?`)) return
  try {
    await batchRetryDeadLetters(Array.from(selected.value))
    selected.value.clear()
    loadDeadLetters()
  } catch (e) { alert('Batch retry failed') }
}

async function handleBatchDiscard() {
  if (!confirm(`Permanently discard ${selected.value.size} tasks?`)) return
  try {
    await batchDiscardDeadLetters(Array.from(selected.value))
    selected.value.clear()
    loadDeadLetters()
  } catch (e) { alert('Batch discard failed') }
}

onMounted(() => {
  loadDeadLetters()
  refreshTimer.value = setInterval(loadDeadLetters, 10000)
})
onUnmounted(() => {
  if (refreshTimer.value) clearInterval(refreshTimer.value)
})
</script>
