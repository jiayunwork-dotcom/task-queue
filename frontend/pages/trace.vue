<template>
  <div class="space-y-6">
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
      <div class="border-b border-gray-200 dark:border-gray-700 px-6">
        <nav class="flex gap-1 -mb-px">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            @click="activeTab = tab.value"
            class="px-4 py-3 text-sm font-medium border-b-2 transition-colors"
            :class="activeTab === tab.value
              ? 'border-blue-500 text-blue-600 dark:text-blue-400'
              : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'"
          >
            <div class="flex items-center gap-2">
              <Icon :name="tab.icon" class="w-4 h-4" />
              <span>{{ tab.label }}</span>
              <UBadge
                v-if="tab.value === 'history' && unreadCount > 0"
                size="2xs"
                color="red"
              >{{ unreadCount > 99 ? '99+' : unreadCount }}</UBadge>
            </div>
          </button>
        </nav>
      </div>

      <div v-show="activeTab === 'trace'" class="p-6">
        <TraceTabContent />
      </div>

      <div v-show="activeTab === 'rules'" class="p-6">
        <AlertRulesTabContent @refresh="loadRules" />
      </div>

      <div v-show="activeTab === 'history'" class="p-6">
        <AlertHistoryTabContent />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAlertHistory } from '~/composables/useApi'
import TraceTabContent from './trace/TraceTabContent.vue'
import AlertRulesTabContent from './trace/AlertRulesTabContent.vue'
import AlertHistoryTabContent from './trace/AlertHistoryTabContent.vue'

const tabs = [
  { value: 'trace', label: '链路追踪', icon: 'i-heroicons-eye-16-solid' },
  { value: 'rules', label: '告警规则', icon: 'i-heroicons-bell-16-solid' },
  { value: 'history', label: '告警历史', icon: 'i-heroicons-clock-16-solid' },
]

const activeTab = ref('trace')
const unreadCount = ref(0)

async function loadRules() {}

async function refreshUnread() {
  const since = new Date(Date.now() - 24 * 3600 * 1000).toISOString()
  const { data } = await useAlertHistory({ from: since, limit: 1 })
  if (data.value) {
    unreadCount.value = (data.value as any).total || 0
  }
}

onMounted(refreshUnread)
</script>
