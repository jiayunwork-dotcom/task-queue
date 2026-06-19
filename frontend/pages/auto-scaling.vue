<template>
  <div class="space-y-8">
    <div class="fixed top-4 right-4 z-50 space-y-2">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="px-4 py-3 rounded-lg shadow-lg border transition-all transform animate-fade-in"
        :class="[
          toast.type === 'success' ? 'bg-green-50 dark:bg-green-900/30 border-green-200 dark:border-green-800 text-green-800 dark:text-green-200' :
          toast.type === 'warning' ? 'bg-amber-50 dark:bg-amber-900/30 border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200' :
          'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-800 dark:text-blue-200'
        ]"
      >
        <div class="flex items-center gap-2">
          <Icon
            :name="toast.type === 'success' ? 'i-heroicons-check-circle-16-solid' : 'i-heroicons-exclamation-triangle-16-solid'"
            class="w-5 h-5"
          />
          <span class="text-sm font-medium">{{ toast.message }}</span>
          <button @click="removeToast(toast.id)" class="ml-2 opacity-70 hover:opacity-100">
            <Icon name="i-heroicons-x-mark-16-solid" class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Auto Scaling</h3>
          <p class="text-sm text-gray-500 mt-1">Configure auto-scaling policies for each task type</p>
        </div>
        <div class="flex items-center gap-3">
          <UButton
            icon="i-heroicons-plus-16-solid"
            @click="openCreateModal"
          >
            New Policy
          </UButton>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div
          v-for="policy in policies"
          :key="policy.id"
          class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden transition-all hover:border-blue-400 dark:hover:border-blue-500 cursor-pointer"
          @click="toggleExpand(policy.id)"
        >
          <div class="p-4 bg-gray-50 dark:bg-gray-700/50">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
                  <Icon name="i-heroicons-arrow-trending-up-16-solid" class="w-5 h-5 text-blue-600" />
                </div>
                <div>
                  <code class="text-sm font-medium text-gray-900 dark:text-white">{{ policy.task_type }}</code>
                  <p class="text-xs text-gray-500 mt-0.5">Target: {{ policy.target_utilization_pct }}% utilization</p>
                  <p class="text-xs text-gray-500 mt-0.5">
                    <span class="inline-flex items-center gap-1">
                      <Icon name="i-heroicons-clock-16-solid" class="w-3 h-3" />
                      {{ scheduleWindowSummary(policy.schedule_windows) }}
                    </span>
                  </p>
                </div>
              </div>
              <div class="flex items-center gap-3" @click.stop>
                <UToggle :model-value="policy.enabled" @update:model-value="handleToggle(policy)" />
                <Icon
                  :name="expandedPolicyId === policy.id ? 'i-heroicons-chevron-up-16-solid' : 'i-heroicons-chevron-down-16-solid'"
                  class="w-5 h-5 text-gray-400"
                />
              </div>
            </div>

            <div class="grid grid-cols-3 gap-4 mt-4">
              <div>
                <p class="text-xs text-gray-500">Min Workers</p>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ policy.min_workers }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500">Max Workers</p>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ policy.max_workers }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500">Cooldown</p>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ policy.cooldown_seconds }}s</p>
              </div>
            </div>
          </div>

          <div v-if="expandedPolicyId === policy.id" class="p-4 border-t border-gray-200 dark:border-gray-700">
            <div class="space-y-4">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Target Utilization (%)</label>
                  <UInput
                    v-model.number="editForm.target_utilization_pct"
                    type="number"
                    min="0"
                    max="100"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-In Protection (s)</label>
                  <UInput
                    v-model.number="editForm.scale_in_protection_secs"
                    type="number"
                    min="0"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-Out Threshold (queue)</label>
                  <UInput
                    v-model.number="editForm.scale_out_threshold"
                    type="number"
                    min="0"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-In Threshold (%)</label>
                  <UInput
                    v-model.number="editForm.scale_in_threshold_pct"
                    type="number"
                    min="0"
                    max="100"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Min Workers</label>
                  <UInput
                    v-model.number="editForm.min_workers"
                    type="number"
                    min="0"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Max Workers</label>
                  <UInput
                    v-model.number="editForm.max_workers"
                    type="number"
                    min="0"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Cooldown (seconds)</label>
                  <UInput
                    v-model.number="editForm.cooldown_seconds"
                    type="number"
                    min="0"
                  />
                </div>
              </div>

              <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
                <div class="flex items-center justify-between mb-3">
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Schedule Windows</label>
                  <UButton
                    size="sm"
                    variant="ghost"
                    icon="i-heroicons-plus-16-solid"
                    :disabled="!editForm.schedule_windows || editForm.schedule_windows.length >= 3"
                    @click.stop="addEditWindow"
                  >
                    Add Window
                  </UButton>
                </div>
                <p class="text-xs text-gray-500 mb-3">Optional: Configure time windows when this policy is active. Leave empty for always active.</p>

                <div v-if="editForm.schedule_windows && editForm.schedule_windows.length > 0" class="space-y-3">
                  <div
                    v-for="(window, idx) in editForm.schedule_windows"
                    :key="idx"
                    class="p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-600"
                  >
                    <div class="flex items-center justify-between mb-2">
                      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">Window {{ idx + 1 }}</span>
                      <UButton
                        size="xs"
                        variant="ghost"
                        color="red"
                        icon="i-heroicons-trash-16-solid"
                        @click.stop="removeEditWindow(idx)"
                      >
                        Remove
                      </UButton>
                    </div>
                    <div class="space-y-2">
                      <div>
                        <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">Days</label>
                        <div class="flex flex-wrap gap-2">
                          <label v-for="day in dayOptions" :key="day.value" class="flex items-center gap-1 cursor-pointer">
                            <input
                              type="checkbox"
                              :value="day.value"
                              v-model="window.days"
                              class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                            />
                            <span class="text-xs text-gray-700 dark:text-gray-300">{{ day.label }}</span>
                          </label>
                        </div>
                      </div>
                      <div class="grid grid-cols-2 gap-2">
                        <div>
                          <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">Start Time</label>
                          <UInput
                            v-model="window.start_time"
                            type="time"
                            size="sm"
                          />
                        </div>
                        <div>
                          <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">End Time</label>
                          <UInput
                            v-model="window.end_time"
                            type="time"
                            size="sm"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="text-center py-4 text-sm text-gray-500 dark:text-gray-400">
                  No schedule windows configured. Policy is always active.
                </div>
              </div>

              <div class="flex justify-end gap-2">
                <UButton
                  size="sm"
                  variant="ghost"
                  color="red"
                  icon="i-heroicons-trash-16-solid"
                  @click.stop="confirmDelete(policy.id)"
                >
                  Delete
                </UButton>
                <UButton
                  size="sm"
                  color="blue"
                  @click.stop="savePolicy(policy.id)"
                >
                  Save Changes
                </UButton>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="policies.length === 0"
          class="col-span-full py-12 text-center text-gray-500 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <Icon name="i-heroicons-inbox-stack-16-solid" class="w-12 h-12 mx-auto mb-3 text-gray-300" />
          <p>No scaling policies yet</p>
          <p class="text-sm mt-1">Click "New Policy" to create your first auto-scaling rule</p>
        </div>
      </div>
    </section>

    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Real-time Metrics</h3>
          <p class="text-sm text-gray-500 mt-1">Live worker utilization and queue status</p>
        </div>
        <UTooltip :content="`Last updated: ${formatDate(lastUpdate)}`">
          <Icon name="i-heroicons-information-circle-16-solid" class="w-5 h-5 text-gray-400" />
        </UTooltip>
      </div>

      <div class="space-y-4">
        <div
          v-for="metric in metrics"
          :key="metric.policy_id"
          class="p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
        >
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-3">
              <code class="text-sm font-medium bg-white dark:bg-gray-800 px-2 py-1 rounded border border-gray-200 dark:border-gray-600">
                {{ metric.task_type }}
              </code>
            </div>
            <div class="text-right">
              <span class="text-2xl font-bold text-gray-900 dark:text-white">
                {{ metric.current_workers }}
              </span>
              <span class="text-sm text-gray-500 ml-1">workers</span>
            </div>
          </div>

          <div class="mb-3">
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs text-gray-500">Utilization</span>
              <span class="text-xs font-medium" :class="getUtilizationColor(metric.utilization_pct)">
                {{ metric.utilization_pct.toFixed(1) }}%
              </span>
            </div>
            <div class="h-3 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="getUtilizationBarClass(metric.utilization_pct)"
                :style="{ width: `${Math.min(metric.utilization_pct, 100)}%` }"
              ></div>
            </div>
          </div>

          <div class="grid grid-cols-3 gap-4 text-center">
            <div>
              <p class="text-xs text-gray-500">Queue Waiting</p>
              <p class="text-lg font-semibold" :class="metric.queue_waiting > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-900 dark:text-white'">
                {{ metric.queue_waiting }}
              </p>
            </div>
            <div>
              <p class="text-xs text-gray-500">Cooldown</p>
              <p class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ formatCooldown(metric.seconds_since_op, getPolicyCooldown(metric.policy_id)) }}
              </p>
            </div>
            <div>
              <p class="text-xs text-gray-500">Last Op</p>
              <p class="text-sm font-medium text-gray-600 dark:text-gray-400">
                {{ metric.last_operation_at ? formatDate(metric.last_operation_at) : '-' }}
              </p>
            </div>
          </div>
        </div>

        <div v-if="metrics.length === 0" class="py-12 text-center text-gray-500">
          <Icon name="i-heroicons-chart-bar-16-solid" class="w-12 h-12 mx-auto mb-3 text-gray-300" />
          <p>No metrics data available</p>
          <p class="text-sm mt-1">Create a scaling policy to see real-time metrics</p>
        </div>
      </div>
    </section>

    <section class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Operation History</h3>
          <p class="text-sm text-gray-500 mt-1">Record of all scaling decisions and actions</p>
        </div>
        <div class="flex items-center gap-2">
          <USelect
            v-model="filterTaskType"
            :options="taskTypeOptions"
            placeholder="All task types"
            size="sm"
            class="w-48"
            @change="loadHistory"
          />
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700">
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Time</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Task Type</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Operation</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Suggested</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Workers</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Utilization</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Queue</th>
              <th class="text-left py-3 px-4 text-sm font-medium text-gray-500">Reason</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="record in history"
              :key="record.id"
              class="border-b border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-700/50"
            >
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400">
                {{ formatDate(record.created_at) }}
              </td>
              <td class="py-3 px-4">
                <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">{{ record.task_type }}</code>
              </td>
              <td class="py-3 px-4">
                <UBadge
                  :color="scalingOpColor(record.operation_type)"
                  size="sm"
                >
                  {{ scalingOpLabel(record.operation_type) }}
                </UBadge>
              </td>
              <td class="py-3 px-4 text-sm text-gray-900 dark:text-white font-medium">
                {{ record.suggested_count || '-' }}
              </td>
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400">
                {{ record.snapshot_workers }}
              </td>
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400">
                {{ record.snapshot_util_pct.toFixed(1) }}%
              </td>
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400">
                {{ record.snapshot_queue }}
              </td>
              <td class="py-3 px-4 text-sm text-gray-600 dark:text-gray-400 max-w-xs truncate">
                <UTooltip :content="record.reason">
                  {{ record.reason }}
                </UTooltip>
              </td>
            </tr>
            <tr v-if="history.length === 0">
              <td colspan="8" class="py-12 text-center text-gray-500">
                <Icon name="i-heroicons-clock-16-solid" class="w-12 h-12 mx-auto mb-3 text-gray-300" />
                <p>No operation history yet</p>
                <p class="text-sm mt-1">Scaling decisions will appear here once policies are active</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="totalPages > 1" class="flex items-center justify-between mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
        <p class="text-sm text-gray-500">
          Showing {{ (currentPage - 1) * pageSize + 1 }} - {{ Math.min(currentPage * pageSize, totalCount) }} of {{ totalCount }} records
        </p>
        <div class="flex items-center gap-2">
          <UButton
            size="sm"
            variant="ghost"
            icon="i-heroicons-chevron-left-16-solid"
            :disabled="currentPage <= 1"
            @click="prevPage"
          />
          <span class="text-sm text-gray-600 dark:text-gray-400 px-2">
            Page {{ currentPage }} / {{ totalPages }}
          </span>
          <UButton
            size="sm"
            variant="ghost"
            icon="i-heroicons-chevron-right-16-solid"
            :disabled="currentPage >= totalPages"
            @click="nextPage"
          />
        </div>
      </div>
    </section>
  </div>

  <UModal v-model="showCreateModal" title="Create Scaling Policy">
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Task Type</label>
        <UInput
          v-model="createForm.task_type"
          placeholder="e.g., send_email, process_payment"
        />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Target Utilization (%)</label>
          <UInput
            v-model.number="createForm.target_utilization_pct"
            type="number"
            min="0"
            max="100"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-In Protection (s)</label>
          <UInput
            v-model.number="createForm.scale_in_protection_secs"
            type="number"
            min="0"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-Out Threshold (queue)</label>
          <UInput
            v-model.number="createForm.scale_out_threshold"
            type="number"
            min="0"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Scale-In Threshold (%)</label>
          <UInput
            v-model.number="createForm.scale_in_threshold_pct"
            type="number"
            min="0"
            max="100"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Min Workers</label>
          <UInput
            v-model.number="createForm.min_workers"
            type="number"
            min="0"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Max Workers</label>
          <UInput
            v-model.number="createForm.max_workers"
            type="number"
            min="0"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Cooldown (seconds)</label>
          <UInput
            v-model.number="createForm.cooldown_seconds"
            type="number"
            min="0"
          />
        </div>
      </div>

      <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
        <div class="flex items-center justify-between mb-3">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Schedule Windows</label>
          <UButton
            size="sm"
            variant="ghost"
            icon="i-heroicons-plus-16-solid"
            :disabled="createForm.schedule_windows.length >= 3"
            @click="addCreateWindow"
          >
            Add Window
          </UButton>
        </div>
        <p class="text-xs text-gray-500 mb-3">Optional: Configure time windows when this policy is active. Leave empty for always active.</p>

        <div v-if="createForm.schedule_windows.length > 0" class="space-y-3">
          <div
            v-for="(window, idx) in createForm.schedule_windows"
            :key="idx"
            class="p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-600"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-medium text-gray-700 dark:text-gray-300">Window {{ idx + 1 }}</span>
              <UButton
                size="xs"
                variant="ghost"
                color="red"
                icon="i-heroicons-trash-16-solid"
                @click="removeCreateWindow(idx)"
              >
                Remove
              </UButton>
            </div>
            <div class="space-y-2">
              <div>
                <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">Days</label>
                <div class="flex flex-wrap gap-2">
                  <label v-for="day in dayOptions" :key="day.value" class="flex items-center gap-1 cursor-pointer">
                    <input
                      type="checkbox"
                      :value="day.value"
                      v-model="window.days"
                      class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span class="text-xs text-gray-700 dark:text-gray-300">{{ day.label }}</span>
                  </label>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">Start Time</label>
                  <UInput
                    v-model="window.start_time"
                    type="time"
                    size="sm"
                  />
                </div>
                <div>
                  <label class="block text-xs text-gray-600 dark:text-gray-400 mb-1">End Time</label>
                  <UInput
                    v-model="window.end_time"
                    type="time"
                    size="sm"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="text-center py-4 text-sm text-gray-500 dark:text-gray-400">
          No schedule windows configured. Policy is always active.
        </div>
      </div>

      <div class="flex items-center gap-2">
        <UCheckbox v-model="createForm.enabled" id="create-enabled" />
        <label for="create-enabled" class="text-sm text-gray-700 dark:text-gray-300">Enable policy immediately</label>
      </div>
    </div>

    <template #footer>
      <UButton variant="ghost" @click="showCreateModal = false">Cancel</UButton>
      <UButton color="blue" @click="createPolicy">Create Policy</UButton>
    </template>
  </UModal>

  <UConfirmDialog
    v-model="showDeleteConfirm"
    title="Delete Scaling Policy"
    description="Are you sure you want to delete this scaling policy? This action cannot be undone."
    confirm-label="Delete"
    confirm-color="red"
    @confirm="deletePolicy"
  />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  useScalingPolicies,
  useScalingMetrics,
  useScalingHistory,
  createScalingPolicy,
  updateScalingPolicy,
  toggleScalingPolicy,
  deleteScalingPolicy,
  scalingOpLabel,
  scalingOpColor,
  scheduleWindowSummary,
  formatDate as formatDateUtil,
  type ScalingPolicy,
  type ScalingPolicyMetrics,
  type ScalingHistory,
  type ScheduleWindow,
  type ScalingEvent,
} from '~/composables/useApi'

const policies = ref<ScalingPolicy[]>([])
const metrics = ref<ScalingPolicyMetrics[]>([])
const history = ref<ScalingHistory[]>([])
const lastUpdate = ref<string>('')

const expandedPolicyId = ref<string | null>(null)
const editForm = ref<any>({})

const showCreateModal = ref(false)
const createForm = ref({
  task_type: '',
  target_utilization_pct: 70,
  min_workers: 1,
  max_workers: 10,
  cooldown_seconds: 300,
  scale_in_protection_secs: 600,
  scale_out_threshold: 10,
  scale_in_threshold_pct: 30,
  enabled: true,
  schedule_windows: [] as ScheduleWindow[],
})

const showDeleteConfirm = ref(false)
const deletingPolicyId = ref('')

const dayOptions = [
  { label: 'Mon', value: 1 },
  { label: 'Tue', value: 2 },
  { label: 'Wed', value: 3 },
  { label: 'Thu', value: 4 },
  { label: 'Fri', value: 5 },
  { label: 'Sat', value: 6 },
  { label: 'Sun', value: 7 },
]

const config = useRuntimeConfig()

const wsConnected = useState<boolean>('ws-connected', () => true)
let ws: WebSocket | null = null
let wsReconnectTimer: number | null = null

const toasts = ref<Array<{ id: number; message: string; type: string }>>([])
let toastId = 0

const filterTaskType = ref('')
const currentPage = ref(1)
const pageSize = 20
const totalCount = ref(0)

const taskTypeOptions = computed(() => {
  const types = [...new Set(policies.value.map(p => p.task_type))]
  return [
    { label: 'All task types', value: '' },
    ...types.map(t => ({ label: t, value: t })),
  ]
})

const totalPages = computed(() => Math.ceil(totalCount.value / pageSize) || 1)

const refreshInterval = ref<number | null>(null)

function formatDate(s?: string | null): string {
  if (!s) return '-'
  return formatDateUtil(s)
}

function getUtilizationColor(util: number): string {
  if (util >= 90) return 'text-red-600 dark:text-red-400'
  if (util >= 70) return 'text-amber-600 dark:text-amber-400'
  return 'text-green-600 dark:text-green-400'
}

function getUtilizationBarClass(util: number): string {
  if (util >= 90) return 'bg-red-500'
  if (util >= 70) return 'bg-amber-500'
  return 'bg-green-500'
}

function formatCooldown(seconds: number, cooldown: number): string {
  if (seconds < 0) return 'Never'
  if (seconds >= cooldown) return 'Ready'
  return `${cooldown - seconds}s`
}

function getPolicyCooldown(policyId: string): number {
  const policy = policies.value.find(p => p.id === policyId)
  return policy?.cooldown_seconds || 300
}

function toggleExpand(policyId: string) {
  if (expandedPolicyId.value === policyId) {
    expandedPolicyId.value = null
  } else {
    const policy = policies.value.find(p => p.id === policyId)
    if (policy) {
      editForm.value = {
        ...policy,
        schedule_windows: (policy.schedule_windows || []).map(w => ({
          days: [...w.days],
          start_time: w.start_time,
          end_time: w.end_time,
        })),
      }
    }
    expandedPolicyId.value = policyId
  }
}

function openCreateModal() {
  createForm.value = {
    task_type: '',
    target_utilization_pct: 70,
    min_workers: 1,
    max_workers: 10,
    cooldown_seconds: 300,
    scale_in_protection_secs: 600,
    scale_out_threshold: 10,
    scale_in_threshold_pct: 30,
    enabled: true,
    schedule_windows: [] as ScheduleWindow[],
  }
  showCreateModal.value = true
}

function addEditWindow() {
  if (!editForm.value.schedule_windows) {
    editForm.value.schedule_windows = []
  }
  if (editForm.value.schedule_windows.length >= 3) {
    return
  }
  editForm.value.schedule_windows = [
    ...editForm.value.schedule_windows,
    {
      days: [],
      start_time: '09:00',
      end_time: '18:00',
    },
  ]
}

function removeEditWindow(index: number) {
  if (!editForm.value.schedule_windows) {
    return
  }
  editForm.value.schedule_windows = editForm.value.schedule_windows.filter(
    (_: any, i: number) => i !== index,
  )
}

function addCreateWindow() {
  if (!createForm.value.schedule_windows) {
    createForm.value.schedule_windows = []
  }
  if (createForm.value.schedule_windows.length >= 3) {
    return
  }
  createForm.value.schedule_windows = [
    ...createForm.value.schedule_windows,
    {
      days: [],
      start_time: '09:00',
      end_time: '18:00',
    },
  ]
}

function removeCreateWindow(index: number) {
  if (!createForm.value.schedule_windows) {
    return
  }
  createForm.value.schedule_windows = createForm.value.schedule_windows.filter(
    (_: any, i: number) => i !== index,
  )
}

function addToast(message: string, type: string = 'info') {
  const id = ++toastId
  toasts.value.push({ id, message, type })
  setTimeout(() => {
    removeToast(id)
  }, 5000)
}

function removeToast(id: number) {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index !== -1) {
    toasts.value.splice(index, 1)
  }
}

function connectWebSocket() {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsHost = window.location.host
  const wsUrl = `${wsProtocol}//${wsHost}/api/v1/auto-scaling/ws`

  try {
    ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      wsConnected.value = true
      if (wsReconnectTimer) {
        clearTimeout(wsReconnectTimer)
        wsReconnectTimer = null
      }
    }

    ws.onmessage = (event) => {
      try {
        const data: ScalingEvent = JSON.parse(event.data)
        console.log('Received scaling event:', data)

        const opLabel = data.operation_type === 'scale_out' ? '扩容' : '缩容'
        addToast(
          `[${data.task_type}] 触发了${opLabel},建议调整 ${data.suggested_count} 个Worker`,
          data.operation_type === 'scale_out' ? 'success' : 'warning'
        )

        loadMetrics()
        loadHistory()
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
      wsConnected.value = false
      ws = null

      if (wsReconnectTimer) {
        clearTimeout(wsReconnectTimer)
      }
      wsReconnectTimer = window.setTimeout(() => {
        console.log('Attempting to reconnect WebSocket...')
        connectWebSocket()
      }, 10000)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }
  } catch (e) {
    console.error('Failed to create WebSocket:', e)
    wsConnected.value = false
  }
}

function disconnectWebSocket() {
  if (wsReconnectTimer) {
    clearTimeout(wsReconnectTimer)
    wsReconnectTimer = null
  }
  if (ws) {
    ws.close()
    ws = null
  }
  wsConnected.value = false
}

async function createPolicy() {
  try {
    if (!createForm.value.task_type) {
      alert('Task type is required')
      return
    }
    await createScalingPolicy(createForm.value)
    showCreateModal.value = false
    await loadPolicies()
    await loadMetrics()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to create policy')
  }
}

async function savePolicy(policyId: string) {
  try {
    await updateScalingPolicy(policyId, editForm.value)
    expandedPolicyId.value = null
    await loadPolicies()
    await loadMetrics()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to update policy')
  }
}

async function handleToggle(policy: ScalingPolicy) {
  try {
    await toggleScalingPolicy(policy.id)
    await loadPolicies()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to toggle policy')
  }
}

function confirmDelete(policyId: string) {
  deletingPolicyId.value = policyId
  showDeleteConfirm.value = true
}

async function deletePolicy() {
  try {
    await deleteScalingPolicy(deletingPolicyId.value)
    showDeleteConfirm.value = false
    expandedPolicyId.value = null
    await loadPolicies()
    await loadMetrics()
    await loadHistory()
  } catch (e: any) {
    alert(e.data?.error || e.message || 'Failed to delete policy')
  }
}

function prevPage() {
  if (currentPage.value > 1) {
    currentPage.value--
    loadHistory()
  }
}

function nextPage() {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
    loadHistory()
  }
}

async function loadPolicies() {
  try {
    const { data } = await useScalingPolicies()
    if (data.value) policies.value = data.value
  } catch {
    // ignore
  }
}

async function loadMetrics() {
  try {
    const { data } = await useScalingMetrics()
    if (data.value) metrics.value = data.value
    lastUpdate.value = new Date().toISOString()
  } catch {
    // ignore
  }
}

async function loadHistory() {
  try {
    const params: Record<string, any> = {
      limit: pageSize,
      offset: (currentPage.value - 1) * pageSize,
    }
    if (filterTaskType.value) {
      params.task_type = filterTaskType.value
    }
    const { data } = await useScalingHistory(params)
    if (data.value) {
      history.value = data.value.items || data.value || []
      totalCount.value = data.value.total || (data.value || []).length
    }
  } catch {
    // ignore
  }
}

async function loadAll() {
  await Promise.all([
    loadPolicies(),
    loadMetrics(),
    loadHistory(),
  ])
}

onMounted(() => {
  loadAll()
  refreshInterval.value = window.setInterval(() => {
    loadMetrics()
    loadHistory()
  }, 5000)
  connectWebSocket()
})

onUnmounted(() => {
  if (refreshInterval.value) clearInterval(refreshInterval.value)
  disconnectWebSocket()
})
</script>

<style>
@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.animate-fade-in {
  animation: fadeIn 0.3s ease-out;
}
</style>
