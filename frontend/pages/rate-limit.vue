<template>
  <div class="space-y-8">
    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Rate Limit Configuration</h3>
          <p class="text-sm text-gray-500 mt-1">Configure execution rate limits for each task type</p>
        </div>
        <UButton
          icon="i-heroicons-plus-16-solid"
          @click="openCreateModal"
        >
          Add Config
        </UButton>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700">
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Task Type</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Max Rate (tasks/s)</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Window Size</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Status</th>
              <th class="text-right py-3 px-4 text-sm font-medium text-gray-500">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(config, taskType) in rateLimitConfigs"
              :key="taskType"
              class="border-b border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-700/50"
            >
              <td class="py-3 px-4">
                <code class="text-sm bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">{{ taskType }}</code>
              </td>
              <td class="py-3 px-4 text-sm text-gray-900 dark:text-white font-medium">
                {{ config.max_per_second }}
              </td>
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400">
                {{ config.window_size_ms }}ms
              </td>
              <td class="py-3 px-4">
                <UBadge
                  :color="config.enabled ? 'green' : 'gray'"
                  size="sm"
                >
                  {{ config.enabled ? 'Enabled' : 'Disabled' }}
                </UBadge>
              </td>
              <td class="py-3 px-4 text-right">
                <div class="flex items-center justify-end gap-2">
                  <UButton
                    size="sm"
                    variant="ghost"
                    icon="i-heroicons-pencil-16-solid"
                    @click="openEditModal(config)"
                  />
                  <UButton
                    size="sm"
                    variant="ghost"
                    color="red"
                    icon="i-heroicons-trash-16-solid"
                    @click="confirmDelete(taskType)"
                  />
                </div>
              </td>
            </tr>
            <tr v-if="Object.keys(rateLimitConfigs).length === 0">
              <td colspan="5" class="py-12 text-center text-gray-500">
                <Icon name="i-heroicons-inbox-stack-16-solid" class="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>No rate limit configurations yet</p>
                <p class="text-sm mt-1">Click "Add Config" to create your first rate limit rule</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Real-time Rate Monitor</h3>
          <p class="text-sm text-gray-500 mt-1">Current execution rate vs configured limits</p>
        </div>
        <UTooltip :content="`Last updated: ${formatDate(lastUpdate)}`">
          <Icon name="i-heroicons-information-circle-16-solid" class="w-5 h-5 text-gray-400" />
        </UTooltip>
      </div>

      <div class="space-y-6">
        <div
          v-for="status in rateLimitStatus"
          :key="status.task_type"
          class="p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
        >
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-3">
              <code class="text-sm font-medium bg-white dark:bg-gray-800 px-2 py-1 rounded border border-gray-200 dark:border-gray-600">
                {{ status.task_type }}
              </code>
              <UBadge
                v-if="status.wait_queue_size > 0"
                color="amber"
                size="sm"
              >
                {{ status.wait_queue_size }} waiting
              </UBadge>
            </div>
            <div class="text-right">
              <span class="text-2xl font-bold" :class="getRateColor(status)">
                {{ status.current_rate.toFixed(2) }}
              </span>
              <span class="text-sm text-gray-500 ml-1">/ {{ status.max_per_second }} tasks/s</span>
            </div>
          </div>

          <div class="h-4 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full transition-all duration-500"
              :class="getProgressBarClass(status)"
              :style="{ width: `${Math.min(status.usage_percent, 100)}%` }"
            ></div>
          </div>

          <div class="flex items-center justify-between mt-2 text-xs text-gray-500">
            <span>Window: {{ status.window_size_ms }}ms</span>
            <span>{{ status.usage_percent.toFixed(1) }}% of limit</span>
          </div>
        </div>

        <div v-if="rateLimitStatus.length === 0" class="py-12 text-center text-gray-500">
          <Icon name="i-heroicons-chart-bar-16-solid" class="w-12 h-12 mx-auto mb-3 text-gray-300" />
          <p>No rate limited task types</p>
          <p class="text-sm mt-1">Configure rate limits above to see monitoring data</p>
        </div>
      </div>
    </section>

    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Throttle Events (Last 1 hour)</h3>
          <p class="text-sm text-gray-500 mt-1">Number of times rate limits were triggered</p>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="stat in throttleStats"
          :key="stat.task_type"
          class="p-4 border border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-gray-500">{{ stat.task_type }}</p>
              <p class="text-2xl font-bold text-red-600 dark:text-red-400 mt-1">
                {{ formatNumber(stat.throttle_count) }}
              </p>
            </div>
            <div class="w-10 h-10 rounded-lg bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
              <Icon name="i-heroicons-no-symbol-16-solid" class="w-5 h-5 text-red-600" />
            </div>
          </div>
        </div>

        <div v-if="throttleStats.length === 0" class="col-span-full py-8 text-center text-gray-500">
          <p>No throttle events in the last hour</p>
        </div>
      </div>
    </section>
  </div>

  <UModal v-model="showConfigModal" :title="isEditing ? 'Edit Rate Limit' : 'Add Rate Limit'">
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Task Type</label>
        <UInput
          v-model="configForm.task_type"
          placeholder="e.g., send_email, process_payment"
          :disabled="isEditing"
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Max Executions Per Second</label>
        <UInput
          v-model.number="configForm.max_per_second"
          type="number"
          min="0"
          placeholder="100"
        />
        <p class="text-xs text-gray-500 mt-1">Set to 0 for unlimited</p>
      </div>
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Sliding Window Size (ms)</label>
        <USelect
          v-model="configForm.window_size_ms"
          :options="[
            { label: '100ms (Precision)', value: 100 },
            { label: '500ms', value: 500 },
            { label: '1 second (Recommended)', value: 1000 },
            { label: '5 seconds', value: 5000 },
            { label: '10 seconds', value: 10000 },
          ]"
        />
        <p class="text-xs text-gray-500 mt-1">Smaller windows offer more precise rate control</p>
      </div>
      <div class="flex items-center gap-2">
        <UCheckbox v-model="configForm.enabled" id="enabled" />
        <label for="enabled" class="text-sm text-gray-700 dark:text-gray-300">Enable rate limiting</label>
      </div>
    </div>

    <template #footer>
      <UButton variant="ghost" @click="showConfigModal = false">Cancel</UButton>
      <UButton color="blue" @click="saveConfig">Save</UButton>
    </template>
  </UModal>

  <UConfirmDialog
    v-model="showDeleteConfirm"
    title="Delete Rate Limit Configuration"
    description="Are you sure you want to delete this rate limit configuration? This action cannot be undone."
    confirm-label="Delete"
    confirm-color="red"
    @confirm="deleteConfig"
  />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { RateLimitConfig, RateLimitStatus, RateLimitThrottleStats } from '~/composables/useApi'

const rateLimitConfigs = ref<Record<string, RateLimitConfig>>({})
const rateLimitStatus = ref<RateLimitStatus[]>([])
const throttleStats = ref<RateLimitThrottleStats[]>([])
const lastUpdate = ref<string>('')

const showConfigModal = ref(false)
const isEditing = ref(false)
const editingTaskType = ref('')
const configForm = ref({
  task_type: '',
  max_per_second: 100,
  window_size_ms: 1000,
  enabled: true,
})

const showDeleteConfirm = ref(false)
const deletingTaskType = ref('')

const refreshInterval = ref<NodeJS.Timeout | null>(null)

function getRateColor(status: RateLimitStatus): string {
  if (status.usage_percent >= 100) return 'text-red-600 dark:text-red-400'
  if (status.usage_percent >= 80) return 'text-amber-600 dark:text-amber-400'
  return 'text-green-600 dark:text-green-400'
}

function getProgressBarClass(status: RateLimitStatus): string {
  if (status.usage_percent >= 100) return 'bg-red-500'
  if (status.usage_percent >= 80) return 'bg-amber-500'
  return 'bg-green-500'
}

function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return n.toString()
}

function formatDate(s?: string): string {
  if (!s) return '-'
  const d = new Date(s)
  return d.toLocaleTimeString()
}

function openCreateModal() {
  isEditing.value = false
  editingTaskType.value = ''
  configForm.value = {
    task_type: '',
    max_per_second: 100,
    window_size_ms: 1000,
    enabled: true,
  }
  showConfigModal.value = true
}

function openEditModal(config: RateLimitConfig) {
  isEditing.value = true
  editingTaskType.value = config.task_type
  configForm.value = {
    task_type: config.task_type,
    max_per_second: config.max_per_second,
    window_size_ms: config.window_size_ms,
    enabled: config.enabled,
  }
  showConfigModal.value = true
}

function confirmDelete(taskType: string) {
  deletingTaskType.value = taskType
  showDeleteConfirm.value = true
}

async function saveConfig() {
  try {
    const taskType = isEditing.value ? editingTaskType.value : configForm.value.task_type
    if (!taskType) {
      alert('Task type is required')
      return
    }

    await setRateLimitConfig(taskType, {
      max_per_second: Number(configForm.value.max_per_second),
      window_size_ms: Number(configForm.value.window_size_ms),
      enabled: configForm.value.enabled,
    })

    showConfigModal.value = false
    await loadData()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to save configuration')
  }
}

async function deleteConfig() {
  try {
    await deleteRateLimitConfig(deletingTaskType.value)
    showDeleteConfirm.value = false
    await loadData()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to delete configuration')
  }
}

async function loadData() {
  try {
    const { data: configs } = await useRateLimitConfigs()
    if (configs.value) rateLimitConfigs.value = configs.value

    const { data: status } = await useRateLimitStatus()
    if (status.value) rateLimitStatus.value = status.value

    const { data: stats } = await useRateLimitThrottleStats(1)
    if (stats.value) throttleStats.value = stats.value

    lastUpdate.value = new Date().toISOString()
  } catch {
    // ignore
  }
}

onMounted(() => {
  loadData()
  refreshInterval.value = setInterval(loadData, 3000)
})

onUnmounted(() => {
  if (refreshInterval.value) clearInterval(refreshInterval.value)
})
</script>
