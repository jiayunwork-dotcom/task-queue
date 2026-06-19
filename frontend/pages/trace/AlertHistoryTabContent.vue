<template>
  <div class="space-y-6">
    <div>
      <h2 class="text-xl font-bold text-gray-900 dark:text-white">告警历史</h2>
      <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">查看最近的告警触发记录和推送状态</p>
    </div>

    <div class="bg-gray-50 dark:bg-gray-900/30 rounded-xl p-4 border border-gray-200 dark:border-gray-700">
      <div class="flex flex-col md:flex-row gap-4 md:items-end">
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 flex-1">
          <div>
            <UFormLabel class="text-xs">时间范围</UFormLabel>
            <USelect
              v-model="filters.timeRange"
              :options="timeRangeOptions"
              size="sm"
            />
          </div>
          <div v-if="filters.timeRange === 'custom'">
            <UFormLabel class="text-xs">开始时间</UFormLabel>
            <UInput
              v-model="filters.customFrom"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div v-if="filters.timeRange === 'custom'">
            <UFormLabel class="text-xs">结束时间</UFormLabel>
            <UInput
              v-model="filters.customTo"
              type="datetime-local"
              size="sm"
            />
          </div>
          <div>
            <UFormLabel class="text-xs">规则名称</UFormLabel>
            <UInput
              v-model="filters.ruleName"
              placeholder="全部规则"
              size="sm"
            />
          </div>
        </div>
        <div class="flex gap-2">
          <UButton size="sm" variant="outline" @click="resetFilters">重置</UButton>
          <UButton size="sm" @click="loadData">查询</UButton>
        </div>
      </div>
    </div>

    <div class="border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">触发时间</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">规则名称</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">任务类型</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">条件类型</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">实际值 vs 阈值</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">详情</th>
              <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">WebHook推送</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <template v-for="h in history" :key="h.id">
              <tr
                class="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors cursor-pointer"
                @click="toggleExpand(h.id)"
              >
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-400 whitespace-nowrap">
                  {{ formatDate(h.triggered_at) }}
                </td>
                <td class="px-4 py-4">
                  <span class="font-medium text-gray-900 dark:text-white">{{ h.rule_name }}</span>
                </td>
                <td class="px-4 py-4">
                  <UBadge v-if="h.task_type" color="blue" variant="subtle" size="sm">{{ h.task_type }}</UBadge>
                  <UBadge v-else color="gray" variant="subtle" size="sm">全部类型</UBadge>
                </td>
                <td class="px-4 py-4">
                  <UBadge :color="alertConditionColor(h.condition_type)" variant="subtle" size="sm">
                    {{ alertConditionLabel(h.condition_type) }}
                  </UBadge>
                </td>
                <td class="px-4 py-4">
                  <div class="flex items-center gap-2 text-sm">
                    <span class="font-mono font-semibold text-red-600 dark:text-red-400">
                      {{ formatAlertValue(h.condition_type, h.actual_value) }}
                    </span>
                    <span class="text-gray-400">/</span>
                    <span class="font-mono text-gray-600 dark:text-gray-400">
                      {{ formatAlertValue(h.condition_type, h.threshold_value) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-400 max-w-md truncate">
                  {{ h.comparison_description }}
                </td>
                <td class="px-4 py-4">
                  <div class="flex items-center gap-1.5">
                    <UBadge
                      :color="h.webhook_success ? 'green' : 'red'"
                      variant="subtle"
                      size="sm"
                    >
                      {{ h.webhook_success ? '推送成功' : '推送失败' }}
                    </UBadge>
                    <Icon
                      v-if="expanded.has(h.id)"
                      name="i-heroicons-chevron-up-16-solid"
                      class="w-4 h-4 text-gray-400"
                    />
                    <Icon
                      v-else
                      name="i-heroicons-chevron-down-16-solid"
                      class="w-4 h-4 text-gray-400"
                    />
                  </div>
                </td>
              </tr>
              <tr v-if="expanded.has(h.id)" class="bg-gray-50 dark:bg-gray-900/30">
                <td colspan="7" class="px-4 py-4">
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">告警ID</p>
                      <p class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ h.id }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">规则ID</p>
                      <p class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ h.rule_id }}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">对比详情</p>
                      <p class="text-gray-700 dark:text-gray-300">{{ h.comparison_description }}</p>
                    </div>
                    <div v-if="h.webhook_url">
                      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">WebHook URL</p>
                      <p class="font-mono text-xs text-gray-700 dark:text-gray-300 break-all">{{ h.webhook_url }}</p>
                    </div>
                    <div v-if="h.webhook_error" class="md:col-span-2">
                      <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">推送错误详情</p>
                      <div class="bg-red-50 dark:bg-red-900/20 rounded-md p-3">
                        <pre class="text-xs text-red-600 dark:text-red-400 whitespace-pre-wrap break-words">{{ h.webhook_error }}</pre>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="history.length === 0">
              <td colspan="7" class="px-4 py-16 text-center text-gray-400">
                <Icon name="i-heroicons-inbox-stack-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>暂无告警记录</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="px-4 py-4 bg-gray-50 dark:bg-gray-900/30 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <p class="text-sm text-gray-500">共 {{ total }} 条记录</p>
        <div class="flex gap-2">
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset === 0"
            @click="prevPage"
          >上一页</UButton>
          <UButton
            size="sm"
            variant="outline"
            :disabled="offset + limit >= total"
            @click="nextPage"
          >下一页</UButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import {
  useAlertHistory,
  alertConditionLabel, alertConditionColor, formatAlertValue, formatDate,
  type AlertHistory,
} from '~/composables/useApi'

const timeRangeOptions = [
  { value: '1h', label: '最近1小时' },
  { value: '6h', label: '最近6小时' },
  { value: '24h', label: '最近24小时' },
  { value: '7d', label: '最近7天' },
  { value: 'custom', label: '自定义' },
]

const filters = reactive({
  timeRange: '24h',
  customFrom: '',
  customTo: '',
  ruleName: '',
})

const limit = 20
const offset = ref(0)
const history = ref<AlertHistory[]>([])
const total = ref(0)
const expanded = ref<Set<string>>(new Set())

function toggleExpand(id: string) {
  if (expanded.value.has(id)) {
    expanded.value.delete(id)
  } else {
    expanded.value.add(id)
  }
}

function resetFilters() {
  filters.timeRange = '24h'
  filters.customFrom = ''
  filters.customTo = ''
  filters.ruleName = ''
  offset.value = 0
  loadData()
}

function getTimeRange() {
  const now = new Date()
  let from: Date
  switch (filters.timeRange) {
    case '1h':
      from = new Date(now.getTime() - 3600 * 1000)
      break
    case '6h':
      from = new Date(now.getTime() - 6 * 3600 * 1000)
      break
    case '24h':
      from = new Date(now.getTime() - 24 * 3600 * 1000)
      break
    case '7d':
      from = new Date(now.getTime() - 7 * 24 * 3600 * 1000)
      break
    case 'custom':
      from = filters.customFrom ? new Date(filters.customFrom) : new Date(now.getTime() - 24 * 3600 * 1000)
      const to = filters.customTo ? new Date(filters.customTo) : now
      return { from, to }
    default:
      from = new Date(now.getTime() - 24 * 3600 * 1000)
  }
  return { from, to: now }
}

async function loadData() {
  const { from, to } = getTimeRange()
  const params: Record<string, any> = {
    limit,
    offset: offset.value,
    from: from.toISOString(),
    to: to.toISOString(),
  }
  if (filters.ruleName) params.rule_name = filters.ruleName

  const { data } = await useAlertHistory(params)
  if (data.value) {
    history.value = (data.value as any).items || []
    total.value = (data.value as any).total || 0
  }
}

function prevPage() {
  if (offset.value >= limit) {
    offset.value -= limit
    loadData()
  }
}

function nextPage() {
  offset.value += limit
  loadData()
}

onMounted(loadData)
</script>
