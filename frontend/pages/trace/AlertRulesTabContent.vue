<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-xl font-bold text-gray-900 dark:text-white">告警规则</h2>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">配置任务监控的告警触发条件和通知方式</p>
      </div>
      <UButton @click="openCreateModal">
        <Icon name="i-heroicons-plus-16-solid" class="w-4 h-4 mr-1" />
        新建规则
      </UButton>
    </div>

    <div class="border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">规则名称</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">任务类型</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">条件类型</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">阈值</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">窗口</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">冷却时间</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">最近触发</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">状态</th>
              <th class="px-4 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <tr
              v-for="rule in rules"
              :key="rule.id"
              class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
              @click="openEditModal(rule)"
            >
              <td class="px-4 py-4">
                <span class="font-medium text-gray-900 dark:text-white">{{ rule.name }}</span>
              </td>
              <td class="px-4 py-4">
                <UBadge v-if="rule.task_type" color="blue" variant="subtle" size="sm">{{ rule.task_type }}</UBadge>
                <UBadge v-else color="gray" variant="subtle" size="sm">全部类型</UBadge>
              </td>
              <td class="px-4 py-4">
                <UBadge :color="alertConditionColor(rule.condition_type)" variant="subtle" size="sm">
                  {{ alertConditionLabel(rule.condition_type) }}
                </UBadge>
              </td>
              <td class="px-4 py-4 font-mono text-sm text-gray-900 dark:text-white">
                {{ formatAlertValue(rule.condition_type, rule.threshold) }}
              </td>
              <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-400">
                {{ rule.window_minutes }}分钟
              </td>
              <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-400">
                {{ Math.floor(rule.cooldown_seconds / 60) }}分钟
              </td>
              <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-400">
                {{ rule.last_triggered_at ? formatDate(rule.last_triggered_at) : '-' }}
              </td>
              <td class="px-4 py-4" @click.stop>
                <Switch
                  :checked="rule.enabled"
                  :on-change="() => handleToggle(rule)"
                  size="sm"
                />
              </td>
              <td class="px-4 py-4 text-right" @click.stop>
                <div class="flex justify-end gap-1">
                  <UButton size="xs" variant="outline" @click.stop="openEditModal(rule)">
                    <Icon name="i-heroicons-pencil-16-solid" class="w-3.5 h-3.5" />
                  </UButton>
                  <UButton size="xs" variant="outline" color="red" @click.stop="handleDelete(rule)">
                    <Icon name="i-heroicons-trash-16-solid" class="w-3.5 h-3.5" />
                  </UButton>
                </div>
              </td>
            </tr>
            <tr v-if="rules.length === 0">
              <td colspan="9" class="px-4 py-16 text-center text-gray-400">
                <Icon name="i-heroicons-bell-slash-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>暂无告警规则</p>
                <p class="text-xs mt-1">点击"新建规则"按钮创建第一个告警规则</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <UModal v-model:open="showModal" :ui="{ width: 'max-w-2xl' }">
      <div class="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
        <h3 class="text-lg font-semibold">
          {{ editingRule ? '编辑告警规则' : '创建告警规则' }}
        </h3>
        <UButton variant="ghost" size="xs" @click="showModal = false">
          <Icon name="i-heroicons-x-mark-16-solid" class="w-5 h-5" />
        </UButton>
      </div>

      <div class="p-6 space-y-5">
        <div>
          <UFormLabel>规则名称 <span class="text-red-500">*</span></UFormLabel>
          <UInput
            v-model="form.name"
            placeholder="例如：订单任务P95耗时告警"
            size="md"
          />
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>监控任务类型</UFormLabel>
            <USelect
              v-model="form.taskTypeMode"
              :options="[
                { value: 'all', label: '全部任务类型' },
                { value: 'specific', label: '指定任务类型' },
              ]"
              size="md"
            />
          </div>
          <div v-if="form.taskTypeMode === 'specific'">
            <UFormLabel>任务类型 <span class="text-red-500">*</span></UFormLabel>
            <UInput
              v-model="form.specificTaskType"
              placeholder="例如：order_process"
              size="md"
            />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>触发条件类型 <span class="text-red-500">*</span></UFormLabel>
            <USelect
              v-model="form.conditionType"
              :options="conditionOptions"
              size="md"
            />
          </div>
          <div>
            <UFormLabel>
              阈值 <span class="text-red-500">*</span>
              <span class="text-gray-400 text-xs ml-1">({{ thresholdUnitHint }})</span>
            </UFormLabel>
            <UInput
              v-model.number="form.threshold"
              type="number"
              :min="0"
              :step="thresholdStep"
              size="md"
            />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>统计窗口时长 (分钟)</UFormLabel>
            <UInput
              v-model.number="form.windowMinutes"
              type="number"
              :min="1"
              size="md"
            />
          </div>
          <div>
            <UFormLabel>冷却时间 (分钟)</UFormLabel>
            <UInput
              v-model.number="form.cooldownMinutes"
              type="number"
              :min="1"
              size="md"
            />
          </div>
        </div>

        <div>
          <UFormLabel>通知方式</UFormLabel>
          <USelect
            v-model="form.notifyType"
            :options="[
              { value: 'webhook', label: 'WebHook (HTTP POST)' },
            ]"
            size="md"
            disabled
          />
        </div>

        <div>
          <UFormLabel>WebHook URL <span class="text-red-500">*</span></UFormLabel>
          <UInput
            v-model="form.webhookUrl"
            placeholder="https://example.com/api/alerts/webhook"
            size="md"
          />
          <p class="text-xs text-gray-500 mt-1">
            POST payload 包含 rule_id, rule_name, condition_type, actual_value, threshold_value, description, triggered_at
          </p>
        </div>

        <div class="flex items-center gap-2">
          <Switch v-model="form.enabled" size="sm" />
          <span class="text-sm text-gray-700 dark:text-gray-300">创建后立即启用</span>
        </div>
      </div>

      <div class="flex justify-end gap-3 p-6 border-t border-gray-200 dark:border-gray-700">
        <UButton variant="outline" @click="showModal = false">取消</UButton>
        <UButton color="blue" @click="handleSubmit" :loading="submitting">
          {{ editingRule ? '保存修改' : '创建规则' }}
        </UButton>
      </div>
    </UModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import {
  useAlertRules, createAlertRule, updateAlertRule, toggleAlertRule, deleteAlertRule,
  alertConditionLabel, alertConditionColor, formatAlertValue, formatDate,
  type AlertRule, type AlertConditionType,
} from '~/composables/useApi'

const emit = defineEmits(['refresh'])

const rules = ref<AlertRule[]>([])
const showModal = ref(false)
const editingRule = ref<AlertRule | null>(null)
const submitting = ref(false)

const conditionOptions = [
  { value: 'duration_p95', label: 'P95耗时阈值' },
  { value: 'failure_rate', label: '失败率阈值 (百分比)' },
  { value: 'queue_backlog', label: '队列积压阈值 (Ready状态)' },
]

const form = reactive({
  name: '',
  taskTypeMode: 'all' as 'all' | 'specific',
  specificTaskType: '',
  conditionType: 'duration_p95' as AlertConditionType,
  threshold: 5000,
  windowMinutes: 5,
  cooldownMinutes: 5,
  notifyType: 'webhook' as const,
  webhookUrl: '',
  enabled: true,
})

const thresholdUnitHint = computed(() => {
  switch (form.conditionType) {
    case 'duration_p95': return '毫秒'
    case 'failure_rate': return '百分比 0-100'
    case 'queue_backlog': return '任务数量'
    default: return ''
  }
})

const thresholdStep = computed(() => {
  return form.conditionType === 'failure_rate' ? 0.1 : 1
})

async function loadRules() {
  const { data } = await useAlertRules()
  if (data.value) {
    rules.value = data.value as AlertRule[]
  }
}

function resetForm() {
  form.name = ''
  form.taskTypeMode = 'all'
  form.specificTaskType = ''
  form.conditionType = 'duration_p95'
  form.threshold = 5000
  form.windowMinutes = 5
  form.cooldownMinutes = 5
  form.notifyType = 'webhook'
  form.webhookUrl = ''
  form.enabled = true
}

function openCreateModal() {
  editingRule.value = null
  resetForm()
  showModal.value = true
}

function openEditModal(rule: AlertRule) {
  editingRule.value = rule
  form.name = rule.name
  form.taskTypeMode = rule.task_type ? 'specific' : 'all'
  form.specificTaskType = rule.task_type || ''
  form.conditionType = rule.condition_type
  form.threshold = rule.threshold
  form.windowMinutes = rule.window_minutes
  form.cooldownMinutes = Math.floor(rule.cooldown_seconds / 60)
  form.notifyType = rule.notify_type
  form.webhookUrl = rule.webhook_url || ''
  form.enabled = rule.enabled
  showModal.value = true
}

function validate(): string | null {
  if (!form.name.trim()) return '请输入规则名称'
  if (form.taskTypeMode === 'specific' && !form.specificTaskType.trim()) return '请输入任务类型'
  if (!form.threshold || form.threshold <= 0) return '阈值必须大于0'
  if (form.conditionType === 'failure_rate' && form.threshold > 100) return '失败率阈值不能超过100'
  if (!form.windowMinutes || form.windowMinutes < 1) return '统计窗口至少1分钟'
  if (!form.cooldownMinutes || form.cooldownMinutes < 1) return '冷却时间至少1分钟'
  if (form.notifyType === 'webhook' && !form.webhookUrl.trim()) return '请输入WebHook URL'
  return null
}

async function handleSubmit() {
  const err = validate()
  if (err) {
    return alert(err)
  }
  submitting.value = true
  try {
    const payload: any = {
      name: form.name,
      condition_type: form.conditionType,
      threshold: form.threshold,
      window_minutes: form.windowMinutes,
      cooldown_minutes: form.cooldownMinutes,
      notify_type: form.notifyType,
      webhook_url: form.webhookUrl,
      enabled: form.enabled,
      task_type: form.taskTypeMode === 'specific' ? form.specificTaskType : null,
    }
    if (editingRule.value) {
      await updateAlertRule(editingRule.value.id, payload)
    } else {
      await createAlertRule(payload)
    }
    await loadRules()
    emit('refresh')
    showModal.value = false
  } catch (e: any) {
    alert(e?.data?.error || e?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

async function handleToggle(rule: AlertRule) {
  try {
    await toggleAlertRule(rule.id)
    await loadRules()
    emit('refresh')
  } catch (e: any) {
    alert(e?.data?.error || e?.message || '操作失败')
    await loadRules()
  }
}

async function handleDelete(rule: AlertRule) {
  if (!confirm(`确定要删除规则"${rule.name}"吗？此操作不可撤销。`)) return
  try {
    await deleteAlertRule(rule.id)
    await loadRules()
    emit('refresh')
  } catch (e: any) {
    alert(e?.data?.error || e?.message || '删除失败')
  }
}

onMounted(loadRules)
</script>
