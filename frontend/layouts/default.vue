<template>
  <div class="flex h-screen bg-gray-50 dark:bg-gray-900">
    <aside class="w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col">
      <div class="p-6 border-b border-gray-200 dark:border-gray-700">
        <h1 class="text-xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Icon name="i-heroicons-cpu-chip-20-solid" class="w-6 h-6 text-blue-600" />
          Task Queue
        </h1>
        <p class="text-xs text-gray-500 mt-1">Admin Panel v1.0</p>
      </div>

      <nav class="flex-1 p-4 space-y-1">
        <NuxtLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors"
          :class="[
            route.path === item.to
              ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
          ]"
        >
          <Icon :name="item.icon" class="w-5 h-5" />
          {{ item.label }}
          <UBadge
            v-if="item.badge"
            :color="item.badgeColor || 'red'"
            size="xs"
            class="ml-auto"
          >
            {{ item.badge }}
          </UBadge>
        </NuxtLink>
      </nav>

      <div class="p-4 border-t border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-full bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center text-white font-bold text-sm">
            A
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-900 dark:text-white truncate">Admin</p>
            <p class="text-xs text-gray-500 truncate">System Administrator</p>
          </div>
        </div>
      </div>
    </aside>

    <main class="flex-1 flex flex-col overflow-hidden">
      <header class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-8 py-4">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-2xl font-bold text-gray-900 dark:text-white">{{ pageTitle }}</h2>
            <p class="text-sm text-gray-500 mt-0.5">{{ pageSubtitle }}</p>
          </div>
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium border"
              :class="wsConnected
                ? 'bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400 border-green-200 dark:border-green-800'
                : 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800'"
            >
              <span class="w-2 h-2 rounded-full" :class="wsConnected ? 'bg-green-500' : 'bg-red-500 animate-pulse'"></span>
              {{ wsConnected ? 'Connected' : 'Disconnected' }}
            </div>
            <UButton
              variant="ghost"
              icon="i-heroicons-arrow-path-16-solid"
              @click="refreshPage"
            >
              Refresh
            </UButton>
          </div>
        </div>
      </header>

      <div class="flex-1 overflow-auto p-8">
        <slot />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import type { RouteLocationNormalizedLoaded } from 'vue-router'

const route = useRoute()

const wsConnected = useState<boolean>('ws-connected', () => true)

interface NavItem {
  to: string
  label: string
  icon: string
  badge?: number | string
  badgeColor?: string
}

const deadLetterCount = ref(0)

const navItems = computed<NavItem[]>(() => [
  { to: '/', label: 'Dashboard', icon: 'i-heroicons-squares-2x2-20-solid' },
  { to: '/tasks', label: 'Tasks', icon: 'i-heroicons-queue-list-20-solid' },
  { to: '/trace', label: 'Trace Analytics', icon: 'i-heroicons-chart-bar-20-solid' },
  { to: '/dead-letter', label: 'Dead Letter', icon: 'i-heroicons-no-symbol-20-solid', badge: deadLetterCount.value || undefined, badgeColor: 'red' },
  { to: '/dags', label: 'DAG Editor', icon: 'i-heroicons-swatch-20-solid' },
  { to: '/workers', label: 'Workers', icon: 'i-heroicons-server-stack-20-solid' },
  { to: '/rate-limit', label: 'Rate Limiting', icon: 'i-heroicons-gauge-20-solid' },
  { to: '/auto-scaling', label: 'Auto Scaling', icon: 'i-heroicons-arrow-trending-up-20-solid' },
])

const pageTitles: Record<string, { title: string; subtitle: string }> = {
  '/': { title: 'Dashboard', subtitle: 'Real-time overview of your task queue cluster' },
  '/tasks': { title: 'Task Management', subtitle: 'Browse and manage all tasks in the system' },
  '/trace': { title: 'Trace Analytics', subtitle: 'Track task execution lifecycle and analyze performance bottlenecks' },
  '/dead-letter': { title: 'Dead Letter Queue', subtitle: 'Inspect and recover failed tasks' },
  '/dags': { title: 'DAG Orchestration', subtitle: 'Design and run task dependency graphs' },
  '/workers': { title: 'Worker Cluster', subtitle: 'Monitor and manage worker nodes' },
  '/rate-limit': { title: 'Rate Limiting', subtitle: 'Configure and monitor task execution rate limits' },
  '/auto-scaling': { title: 'Auto Scaling', subtitle: 'Configure and monitor automatic worker scaling policies' },
}

const pageTitle = computed(() => pageTitles[route.path]?.title || 'Task Queue')
const pageSubtitle = computed(() => pageTitles[route.path]?.subtitle || '')

async function refreshPage() {
  await refreshNuxtData()
  await loadDeadLetterCount()
}

async function loadDeadLetterCount() {
  try {
    const { data } = await useMetricsSnapshot()
    if (data.value) {
      deadLetterCount.value = data.value.dead_letter_count
    }
  } catch {
    // ignore
  }
}

onMounted(() => {
  loadDeadLetterCount()
})
</script>
