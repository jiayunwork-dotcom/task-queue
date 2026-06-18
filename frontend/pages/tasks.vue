<template>
  <div class="space-y-6">
    <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex flex-col md:flex-row gap-4 md:items-end">
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 flex-1">
          <div>
            <UFormLabel class="text-xs">Status</UFormLabel>
            <USelect
              v-model="filters.status"
              :options="statusOptions"
              placeholder="All statuses"
              size="sm"
              :clearable="true"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">Priority</UFormLabel>
            <USelect
              v-model="filters.priority"
              :options="priorityOptions"
              placeholder="All priorities"
              size="sm"
              :clearable="true"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">Task Type</UFormLabel>
            <UInput
              v-model="filters.type"
              placeholder="e.g. email.send"
              size="sm"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">Created From</UFormLabel>
            <UInput
              v-model="filters.from"
              type="datetime-local"
              size="sm"
            />
          </div>
        </div>
        <div class="flex gap-2">
          <UButton size="sm" variant="outline" @click="resetFilters">Reset</UButton>
          <UButton size="sm" @click="loadTasks">Apply</UButton>
          <UButton size="sm" color="green" @click="showCreateModal = true">
            <Icon name="i-heroicons-plus-16-solid" class="w-4 h-4 mr-1" />
            Submit Task
          </UButton>
        </div>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider w-10">
                <input type="checkbox" class="rounded" @change="toggleAll" :checked="allSelected" />
              </th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Task ID</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Type</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Priority</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Retries</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Created</th>
              <th class="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <tr
              v-for="task in tasks"
              :key="task.id"
              class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
              @click="openTask(task.id)"
            >
              <td class="px-6 py-4" @click.stop>
                <input
                  type="checkbox"
                  class="rounded"
                  :checked="selected.has(task.id)"
                  @change="toggleOne(task.id)"
                />
              </td>
              <td class="px-6 py-4">
                <span class="font-mono text-xs text-gray-600 dark:text-gray-400">{{ task.id.slice(0, 8) }}...</span>
              </td>
              <td class="px-6 py-4">
                <span class="font-medium text-gray-900 dark:text-white">{{ task.type }}</span>
              </td>
              <td class="px-6 py-4">
                <UBadge :color="priorityColor(task.priority)" variant="subtle" size="sm">
                  {{ priorityLabel(task.priority) }}
                </UBadge>
              </td>
              <td class="px-6 py-4">
                <UBadge :color="statusColor(task.status)" variant="subtle" size="sm">
                  {{ task.status }}
                </UBadge>
              </td>
              <td class="px-6 py-4">
                <span class="text-sm">
                  <span :class="task.retry_count > 0 ? 'text-amber-600 font-bold' : 'text-gray-500'">
                    {{ task.retry_count }}
                  </span>
                  <span class="text-gray-400">/{{ task.max_retries }}</span>
                </span>
              </td>
              <td class="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">
                {{ formatDate(task.created_at) }}
              </td>
              <td class="px-6 py-4" @click.stop>
                <div class="flex gap-1">
                  <UTooltip content="View details">
                    <UButton size="2xs" variant="ghost" icon="i-heroicons-eye-16-solid" @click="openTask(task.id)" />
                  </UTooltip>
                  <UTooltip content="Cancel" v-if="!['success','failed','dead_letter','cancelled'].includes(task.status)">
                    <UButton size="2xs" variant="ghost" color="red" icon="i-heroicons-x-mark-16-solid" @click="handleCancel(task.id)" />
                  </UTooltip>
                </div>
              </td>
            </tr>
            <tr v-if="tasks.length === 0">
              <td colspan="8" class="px-6 py-16 text-center text-gray-400">
                <Icon name="i-heroicons-inbox-stack-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>No tasks found</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="px-6 py-4 bg-gray-50 dark:bg-gray-900/30 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <p class="text-sm text-gray-500">
          Showing {{ tasks.length }} of {{ total }} tasks
        </p>
        <div class="flex gap-2">
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset === 0"
            @click="prevPage"
          >Previous</UButton>
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset + limit >= total"
            @click="nextPage"
          >Next</UButton>
        </div>
      </div>
    </div>

    <UModal v-model="showDetailModal" class="w-[800px] max-w-[95vw]">
      <template #header>
        <div class="flex items-center gap-3">
          <h3 class="text-lg font-bold">Task Details</h3>
          <UBadge v-if="selectedTask" :color="statusColor(selectedTask.status)">{{ selectedTask.status }}</UBadge>
        </div>
      </template>

      <div v-if="selectedTask" class="space-y-6 p-6">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-gray-500 mb-1">Task ID</p>
            <p class="font-mono text-sm">{{ selectedTask.id }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Type</p>
            <p class="font-medium">{{ selectedTask.type }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Priority</p>
            <UBadge :color="priorityColor(selectedTask.priority)" size="sm">{{ priorityLabel(selectedTask.priority) }}</UBadge>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Retries</p>
            <p>{{ selectedTask.retry_count }} / {{ selectedTask.max_retries }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Created</p>
            <p class="text-sm">{{ formatDate(selectedTask.created_at) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 mb-1">Completed</p>
            <p class="text-sm">{{ formatDate(selectedTask.completed_at) }}</p>
          </div>
        </div>

        <div>
          <p class="text-xs text-gray-500 mb-2">Payload</p>
          <pre class="bg-gray-900 text-green-400 p-4 rounded-lg text-xs overflow-auto max-h-40">{{ JSON.stringify(selectedTask.payload, null, 2) }}</pre>
        </div>

        <div v-if="selectedTask.last_error">
          <p class="text-xs text-gray-500 mb-2">Last Error</p>
          <pre class="bg-red-50 dark:bg-red-900/20 text-red-600 p-4 rounded-lg text-xs overflow-auto max-h-32">{{ selectedTask.last_error }}</pre>
        </div>

        <div>
          <p class="text-sm font-semibold mb-3">Execution History Timeline</p>
          <div class="space-y-0">
            <div
              v-for="(exec, idx) in executions"
              :key="exec.id"
              class="relative pl-8 pb-6 last:pb-0"
            >
              <div
                class="absolute left-0 top-1 w-4 h-4 rounded-full border-2 border-white dark:border-gray-800 flex items-center justify-center"
                :class="exec.status === 'success' ? 'bg-green-500' : exec.status === 'failed' ? 'bg-red-500' : 'bg-amber-500'"
              ></div>
              <div
                v-if="idx < executions.length - 1"
                class="absolute left-[7px] top-5 w-0.5 h-[calc(100%-16px)] bg-gray-200 dark:bg-gray-700"
              ></div>
              <div class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-4">
                <div class="flex items-center justify-between mb-2">
                  <div class="flex items-center gap-2">
                    <span class="font-semibold">Attempt {{ exec.attempt }}</span>
                    <UBadge :color="statusColor(exec.status)" size="xs">{{ exec.status }}</UBadge>
                  </div>
                  <span class="text-xs text-gray-500">{{ formatDuration(exec.duration_ms) }}</span>
                </div>
                <p class="text-xs text-gray-500 mb-1">
                  {{ formatDate(exec.started_at) }} → {{ formatDate(exec.ended_at) }}
                </p>
                <p class="text-xs text-gray-500">
                  Worker: <span class="font-mono">{{ exec.worker_id?.slice(0, 8) }}</span> |
                  Handler: {{ exec.handler_id }}
                </p>
                <pre v-if="exec.error" class="mt-2 bg-red-50 dark:bg-red-900/20 text-red-600 p-2 rounded text-xs overflow-auto">{{ exec.error }}</pre>
              </div>
            </div>
            <div v-if="executions.length === 0" class="text-gray-400 text-sm py-4">No executions yet</div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex gap-2 justify-end">
          <UButton
            v-if="selectedTask && selectedTask.status !== 'cancelled'"
            variant="outline"
            color="red"
            @click="handleCancel(selectedTask.id)"
          >Cancel Task</UButton>
          <UButton
            v-if="selectedTask && ['failed','dead_letter','cancelled'].includes(selectedTask.status)"
            color="green"
            @click="handleRetry(selectedTask.id)"
          >Retry Manually</UButton>
          <UButton variant="ghost" @click="showDetailModal = false">Close</UButton>
        </div>
      </template>
    </UModal>

    <UModal v-model="showCreateModal" class="w-[600px] max-w-[95vw]">
      <template #header>
        <h3 class="text-lg font-bold">Submit New Task</h3>
      </template>

      <div class="p-6 space-y-4">
        <div>
          <UFormLabel>Task Type *</UFormLabel>
          <UInput v-model="createForm.type" placeholder="e.g. email.send, user.sync" size="sm" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>Priority</UFormLabel>
            <USelect
              v-model="createForm.priority"
              :options="priorityOptions"
              size="sm"
            />
          </div>
          <div>
            <UFormLabel>Max Retries</UFormLabel>
            <UInput v-model.number="createForm.max_retries" type="number" min="0" size="sm" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>Delay (seconds)</UFormLabel>
            <UInput v-model.number="createForm.delay_seconds" type="number" min="0" size="sm" />
          </div>
          <div>
            <UFormLabel>Timeout (seconds)</UFormLabel>
            <UInput v-model.number="createForm.timeout_seconds" type="number" min="1" size="sm" />
          </div>
        </div>
        <div>
          <UFormLabel>Callback URL</UFormLabel>
          <UInput v-model="createForm.callback_url" placeholder="https://..." size="sm" />
        </div>
        <div>
          <UFormLabel>Payload (JSON)</UFormLabel>
          <UTextarea
            v-model="createForm.payload_text"
            rows="6"
            placeholder='{"key": "value"}'
            class="font-mono text-xs"
          />
        </div>
      </div>

      <template #footer>
        <div class="flex gap-2 justify-end">
          <UButton variant="ghost" @click="showCreateModal = false">Cancel</UButton>
          <UButton color="green" @click="handleSubmit">Submit Task</UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const filters = ref({
  status: '' as any,
  priority: '' as any,
  type: '',
  from: '',
})

const statusOptions = [
  { value: 'pending', label: 'Pending' },
  { value: 'delayed', label: 'Delayed' },
  { value: 'ready', label: 'Ready' },
  { value: 'running', label: 'Running' },
  { value: 'success', label: 'Success' },
  { value: 'failed', label: 'Failed' },
  { value: 'dead_letter', label: 'Dead Letter' },
  { value: 'cancelled', label: 'Cancelled' },
]

const priorityOptions = [
  { value: '4', label: 'Critical' },
  { value: '3', label: 'High' },
  { value: '2', label: 'Normal' },
  { value: '1', label: 'Low' },
  { value: '0', label: 'Bulk' },
]

const limit = 50
const offset = ref(0)
const tasks = ref<any[]>([])
const total = ref(0)
const selected = ref<Set<string>>(new Set())
const showDetailModal = ref(false)
const showCreateModal = ref(false)
const selectedTask = ref<any>(null)
const executions = ref<any[]>([])

const createForm = ref({
  type: '',
  priority: '2',
  max_retries: 3,
  delay_seconds: 0,
  timeout_seconds: 60,
  callback_url: '',
  payload_text: '{}',
})

const allSelected = computed(() => tasks.value.length > 0 && tasks.value.every(t => selected.value.has(t.id)))

function toggleAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) {
    tasks.value.forEach(t => selected.value.add(t.id))
  } else {
    selected.value.clear()
  }
}

function toggleOne(id: string) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
}

function resetFilters() {
  filters.value = { status: '', priority: '', type: '', from: '' }
  offset.value = 0
  loadTasks()
}

async function loadTasks() {
  const params: Record<string, any> = { limit, offset: offset.value }
  if (filters.value.status) params.status = filters.value.status
  if (filters.value.priority) params.priority = filters.value.priority
  if (filters.value.type) params.type = filters.value.type
  if (filters.value.from) params.from = new Date(filters.value.from).toISOString()

  const { data } = await useTasks(params)
  if (data.value) {
    tasks.value = data.value.items || []
    total.value = data.value.total || 0
  }
}

function prevPage() {
  if (offset.value >= limit) {
    offset.value -= limit
    loadTasks()
  }
}

function nextPage() {
  offset.value += limit
  loadTasks()
}

async function openTask(id: string) {
  const { data: task } = await useTask(id)
  selectedTask.value = task.value
  const { data: ex } = await useTaskExecutions(id)
  executions.value = (ex.value || []).sort((a: any, b: any) => a.attempt - b.attempt)
  showDetailModal.value = true
}

async function handleCancel(id: string) {
  try {
    await cancelTask(id)
    loadTasks()
    if (showDetailModal.value) openTask(id)
  } catch (e) {
    alert('Failed to cancel task')
  }
}

async function handleRetry(id: string) {
  try {
    await retryTask(id)
    loadTasks()
    if (showDetailModal.value) openTask(id)
  } catch (e) {
    alert('Failed to retry task')
  }
}

async function handleSubmit() {
  try {
    let payload: any
    try {
      payload = JSON.parse(createForm.value.payload_text)
    } catch (e) {
      alert('Invalid JSON payload')
      return
    }
    await createTask({
      ...createForm.value,
      priority: parseInt(createForm.value.priority),
      payload,
    })
    showCreateModal.value = false
    createForm.value = {
      type: '', priority: '2', max_retries: 3,
      delay_seconds: 0, timeout_seconds: 60,
      callback_url: '', payload_text: '{}',
    }
    loadTasks()
  } catch (e: any) {
    alert('Failed to submit task: ' + (e.data?.error || e.message))
  }
}

onMounted(() => loadTasks())
</script>
