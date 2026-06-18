<template>
  <div class="space-y-6">
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center gap-4">
          <div class="w-14 h-14 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
            <Icon name="i-heroicons-server-stack-16-solid" class="w-7 h-7 text-green-600" />
          </div>
          <div>
            <p class="text-sm text-gray-500">Total Workers</p>
            <p class="text-3xl font-bold">{{ workers.length }}</p>
          </div>
        </div>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center gap-4">
          <div class="w-14 h-14 rounded-lg bg-emerald-100 dark:bg-emerald-900/30 flex items-center justify-center">
            <Icon name="i-heroicons-check-circle-16-solid" class="w-7 h-7 text-emerald-600" />
          </div>
          <div>
            <p class="text-sm text-gray-500">Online</p>
            <p class="text-3xl font-bold text-emerald-600">{{ onlineCount }}</p>
          </div>
        </div>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center gap-4">
          <div class="w-14 h-14 rounded-lg bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
            <Icon name="i-heroicons-x-circle-16-solid" class="w-7 h-7 text-red-600" />
          </div>
          <div>
            <p class="text-sm text-gray-500">Offline</p>
            <p class="text-3xl font-bold text-red-600">{{ offlineCount }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
      <div class="p-6 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h3 class="text-lg font-semibold">Worker Nodes</h3>
        <UButton size="sm" @click="loadWorkers">
          <Icon name="i-heroicons-arrow-path-16-solid" class="w-4 h-4 mr-1" />
          Refresh
        </UButton>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 p-6">
        <div
          v-for="worker in workers"
          :key="worker.id"
          class="rounded-xl border-2 p-5 transition-all hover:shadow-md cursor-pointer"
          :class="[
            worker.status === 'online' ? 'border-green-200 dark:border-green-800 bg-green-50/30 dark:bg-green-900/10' :
            worker.status === 'draining' ? 'border-amber-200 dark:border-amber-800 bg-amber-50/30 dark:bg-amber-900/10' :
            'border-gray-200 dark:border-gray-700 bg-gray-50/30 dark:bg-gray-800/50'
          ]"
          @click="openWorker(worker.id)"
        >
          <div class="flex items-start justify-between mb-4">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center text-white font-bold">
                {{ worker.name?.charAt(0).toUpperCase() || 'W' }}
              </div>
              <div>
                <h4 class="font-semibold">{{ worker.name }}</h4>
                <p class="text-xs text-gray-500 font-mono">{{ worker.id?.slice(0, 8) }}...</p>
              </div>
            </div>
            <UBadge :color="workerStatusColor(worker.status)" size="sm">
              <span class="flex items-center gap-1">
                <span class="w-1.5 h-1.5 rounded-full" :class="worker.status === 'online' ? 'bg-green-500 animate-pulse' : ''"></span>
                {{ worker.status }}
              </span>
            </UBadge>
          </div>

          <div class="space-y-3">
            <div>
              <div class="flex items-center justify-between text-xs mb-1">
                <span class="text-gray-500">Slot Utilization</span>
                <span class="font-semibold">{{ worker.used_slots }} / {{ worker.total_slots }}</span>
              </div>
              <div class="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  class="h-full transition-all rounded-full"
                  :class="utilColor(worker)"
                  :style="{ width: `${(worker.used_slots / Math.max(worker.total_slots, 1)) * 100}%` }"
                ></div>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="p-2 rounded-lg bg-white dark:bg-gray-800 text-center">
                <p class="text-xs text-gray-500">Completed</p>
                <p class="font-bold text-green-600">{{ formatNumber(worker.tasks_completed) }}</p>
              </div>
              <div class="p-2 rounded-lg bg-white dark:bg-gray-800 text-center">
                <p class="text-xs text-gray-500">Failed</p>
                <p class="font-bold text-red-600">{{ formatNumber(worker.tasks_failed) }}</p>
              </div>
            </div>

            <div class="pt-2 border-t border-gray-200 dark:border-gray-700 space-y-1 text-xs text-gray-500">
              <div class="flex items-center gap-1.5">
                <Icon name="i-heroicons-computer-desktop-16-solid" class="w-3.5 h-3.5" />
                <span>{{ worker.hostname }}</span>
              </div>
              <div class="flex items-center gap-1.5">
                <Icon name="i-heroicons-heart-16-solid" class="w-3.5 h-3.5" :class="worker.status === 'online' ? 'text-green-500' : 'text-gray-400'" />
                <span>Last: {{ formatDate(worker.last_heartbeat_at) }}</span>
              </div>
              <div class="flex items-center gap-1.5">
                <Icon name="i-heroicons-calendar-16-solid" class="w-3.5 h-3.5" />
                <span>Since: {{ formatDate(worker.registered_at) }}</span>
              </div>
            </div>

            <div v-if="worker.running_tasks && worker.running_tasks.length > 0" class="pt-2 border-t border-gray-200 dark:border-gray-700">
              <p class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2">
                Running Tasks ({{ worker.running_tasks.length }})
              </p>
              <div class="space-y-1 max-h-20 overflow-auto">
                <div
                  v-for="tid in worker.running_tasks.slice(0, 5)"
                  :key="tid"
                  class="text-xs font-mono px-2 py-1 rounded bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 truncate"
                >
                  {{ tid.slice(0, 8) }}...
                </div>
                <p v-if="worker.running_tasks.length > 5" class="text-xs text-gray-400 pl-2">
                  +{{ worker.running_tasks.length - 5 }} more
                </p>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="workers.length === 0"
          class="md:col-span-2 xl:col-span-3 p-12 text-center text-gray-400 border-2 border-dashed rounded-xl"
        >
          <Icon name="i-heroicons-server-stack-20-solid" class="w-16 h-16 mx-auto mb-4 opacity-50" />
          <p class="text-lg font-medium mb-1">No workers connected</p>
          <p class="text-sm">Register workers through the HTTP API to process tasks</p>
        </div>
      </div>
    </div>

    <UModal v-model="showDetail" class="w-[700px] max-w-[95vw]">
      <template #header>
        <div class="flex items-center gap-3">
          <h3 class="text-lg font-bold">Worker Details</h3>
          <UBadge v-if="selectedWorker" :color="workerStatusColor(selectedWorker.status)">
            {{ selectedWorker.status }}
          </UBadge>
        </div>
      </template>

      <div v-if="selectedWorker" class="space-y-6 p-6">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-gray-500 mb-1">Worker ID</p>
            <p class="font-mono text-sm break-all">{{ selectedWorker.id }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Name</p>
            <p class="font-medium">{{ selectedWorker.name }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Hostname</p>
            <p>{{ selectedWorker.hostname }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Total Slots</p>
            <p class="font-bold">{{ selectedWorker.total_slots }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Registered</p>
            <p>{{ formatDate(selectedWorker.registered_at) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Last Heartbeat</p>
            <p>{{ formatDate(selectedWorker.last_heartbeat_at) }}</p>
          </div>
        </div>

        <div class="p-4 bg-gray-50 dark:bg-gray-700/30 rounded-lg space-y-3">
          <h4 class="font-semibold text-sm">Performance Statistics</h4>
          <div class="grid grid-cols-4 gap-4">
            <div class="text-center p-3 bg-white dark:bg-gray-800 rounded-lg">
              <p class="text-2xl font-bold text-green-600">{{ formatNumber(selectedWorker.tasks_completed) }}</p>
              <p class="text-xs text-gray-500 mt-1">Completed</p>
            </div>
            <div class="text-center p-3 bg-white dark:bg-gray-800 rounded-lg">
              <p class="text-2xl font-bold text-red-600">{{ formatNumber(selectedWorker.tasks_failed) }}</p>
              <p class="text-xs text-gray-500 mt-1">Failed</p>
            </div>
            <div class="text-center p-3 bg-white dark:bg-gray-800 rounded-lg">
              <p class="text-2xl font-bold text-blue-600">{{ selectedWorker.used_slots }}</p>
              <p class="text-xs text-gray-500 mt-1">Active Slots</p>
            </div>
            <div class="text-center p-3 bg-white dark:bg-gray-800 rounded-lg">
              <p class="text-2xl font-bold text-purple-600">{{ successRate }}%</p>
              <p class="text-xs text-gray-500 mt-1">Success Rate</p>
            </div>
          </div>
        </div>

        <div>
          <h4 class="font-semibold text-sm mb-3">Running Tasks</h4>
          <div v-if="selectedWorker.running_tasks && selectedWorker.running_tasks.length > 0" class="space-y-2">
            <div
              v-for="tid in selectedWorker.running_tasks"
              :key="tid"
              class="flex items-center justify-between p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800"
            >
              <span class="font-mono text-sm">{{ tid }}</span>
              <UBadge color="amber" size="xs">running</UBadge>
            </div>
          </div>
          <p v-else class="text-sm text-gray-400 py-4 text-center border-2 border-dashed rounded-lg">
            No tasks currently running
          </p>
        </div>
      </div>

      <template #footer>
        <div class="flex gap-2 justify-end">
          <UButton
            v-if="selectedWorker && selectedWorker.status === 'online'"
            variant="outline"
            color="amber"
            @click="initiateShutdown(selectedWorker.id)"
          >Initiate Shutdown</UButton>
          <UButton variant="ghost" @click="showDetail = false">Close</UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

const workers = ref<any[]>([])
const showDetail = ref(false)
const selectedWorker = ref<any>(null)
const refreshTimer = ref<any>(null)

const onlineCount = computed(() => workers.value.filter(w => w.status === 'online' || w.status === 'draining').length)
const offlineCount = computed(() => workers.value.filter(w => w.status === 'offline').length)

const successRate = computed(() => {
  if (!selectedWorker.value) return 0
  const total = (selectedWorker.value.tasks_completed || 0) + (selectedWorker.value.tasks_failed || 0)
  if (total === 0) return 100
  return ((selectedWorker.value.tasks_completed / total) * 100).toFixed(1)
})

function utilColor(w: any) {
  const pct = (w.used_slots / Math.max(w.total_slots, 1)) * 100
  if (pct < 60) return 'bg-green-500'
  if (pct < 85) return 'bg-amber-500'
  return 'bg-red-500'
}

function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return n?.toString() || '0'
}

async function loadWorkers() {
  const { data } = await useWorkers()
  if (data.value) workers.value = data.value || []
}

async function openWorker(id: string) {
  const { data } = await useWorker(id)
  selectedWorker.value = data.value
  showDetail.value = true
}

async function initiateShutdown(id: string) {
  if (!confirm('Initiate graceful shutdown for this worker?')) return
  try {
    const config = useRuntimeConfig()
    await $fetch(`/workers/${id}/shutdown`, {
      baseURL: config.public.apiBase || '/api/v1',
      method: 'POST',
    })
    loadWorkers()
    showDetail.value = false
  } catch (e) {
    alert('Shutdown request failed')
  }
}

onMounted(() => {
  loadWorkers()
  refreshTimer.value = setInterval(loadWorkers, 5000)
})
onUnmounted(() => {
  if (refreshTimer.value) clearInterval(refreshTimer.value)
})
</script>
