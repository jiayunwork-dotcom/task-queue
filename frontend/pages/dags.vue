<template>
  <div class="space-y-6">
    <div class="flex gap-3">
      <div class="flex-1 bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold">DAG Templates</h3>
          <UButton size="sm" color="green" @click="showCreateModal = true">
            <Icon name="i-heroicons-plus-16-solid" class="w-4 h-4 mr-1" />
            New Template
          </UButton>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div
            v-for="tpl in templates"
            :key="tpl.id"
            class="p-4 rounded-lg border-2 transition-all cursor-pointer"
            :class="selectedTemplate?.id === tpl.id
              ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
              : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'"
            @click="selectTemplate(tpl)"
          >
            <div class="flex items-start justify-between mb-2">
              <div>
                <h4 class="font-semibold">{{ tpl.name }}</h4>
                <p v-if="tpl.description" class="text-xs text-gray-500 mt-0.5">{{ tpl.description }}</p>
              </div>
              <div class="flex gap-1">
                <UTooltip content="Run DAG">
                  <UButton size="2xs" color="green" icon="i-heroicons-play-16-solid" @click.stop="runTemplate(tpl.id)" />
                </UTooltip>
              </div>
            </div>
            <div class="flex items-center gap-4 text-xs text-gray-500 mt-3">
              <span>{{ tpl.nodes?.length || 0 }} nodes</span>
              <span>{{ tpl.edges?.length || 0 }} edges</span>
              <span>{{ formatDate(tpl.created_at) }}</span>
            </div>
          </div>

          <div
            v-if="templates.length === 0"
            class="md:col-span-2 p-8 text-center text-gray-400 border-2 border-dashed rounded-lg"
          >
            <Icon name="i-heroicons-swatch-20-solid" class="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>No DAG templates yet. Create one to get started.</p>
          </div>
        </div>
      </div>

      <div class="w-96 bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <h3 class="text-lg font-semibold mb-4">Recent Runs</h3>
        <div class="space-y-2 max-h-96 overflow-auto">
          <div
            v-for="run in runs"
            :key="run.id"
            class="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/30 border border-gray-200 dark:border-gray-700 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700"
            @click="loadRun(run.id)"
          >
            <div class="flex items-center justify-between mb-1">
              <span class="font-mono text-xs">{{ run.id.slice(0, 8) }}...</span>
              <UBadge :color="dagStatusColor(run.status)" size="xs">{{ run.status }}</UBadge>
            </div>
            <p class="text-xs text-gray-500">{{ formatDate(run.created_at) }}</p>
          </div>
          <div v-if="runs.length === 0" class="text-center text-sm text-gray-400 py-8">No runs yet</div>
        </div>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
      <div class="flex items-center justify-between mb-6">
        <h3 class="text-lg font-semibold">
          {{ selectedTemplate ? `Topology: ${selectedTemplate.name}` : runDetail ? 'Run Topology' : 'Select a template to view' }}
        </h3>
        <div v-if="selectedTemplate" class="flex gap-2">
          <UButton size="sm" variant="outline" @click="showCreateModal = true; editMode = true">
            <Icon name="i-heroicons-pencil-16-solid" class="w-4 h-4 mr-1" />
            Edit
          </UButton>
          <UButton size="sm" color="green" @click="runTemplate(selectedTemplate.id)">
            <Icon name="i-heroicons-play-16-solid" class="w-4 h-4 mr-1" />
            Run
          </UButton>
        </div>
      </div>

      <div class="relative bg-gray-50 dark:bg-gray-900/30 rounded-xl border-2 border-dashed border-gray-200 dark:border-gray-700 min-h-[400px] overflow-auto p-8">
        <svg class="absolute inset-0 w-full h-full pointer-events-none" style="min-height: 400px">
          <defs>
            <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#94a3b8" />
            </marker>
          </defs>
          <g v-for="edge in displayEdges" :key="`${edge.from}-${edge.to}`">
            <line
              :x1="getNodePos(edge.from).x"
              :y1="getNodePos(edge.from).y"
              :x2="getNodePos(edge.to).x"
              :y2="getNodePos(edge.to).y"
              stroke="#94a3b8"
              stroke-width="2"
              marker-end="url(#arrow)"
            />
          </g>
        </svg>

        <div
          v-for="node in displayNodes"
          :key="node.id"
          class="absolute w-40 transform -translate-x-1/2 -translate-y-1/2"
          :style="{ left: getNodePos(node.id).x + 'px', top: getNodePos(node.id).y + 'px' }"
        >
          <div
            class="rounded-lg shadow-md border-2 p-3 transition-all"
            :class="getNodeClass(node.id)"
          >
            <p class="font-semibold text-sm truncate">{{ node.name || node.task_type }}</p>
            <p class="text-xs opacity-75 mt-0.5 truncate">{{ node.task_type }}</p>
            <div class="flex items-center justify-between mt-2 text-xs">
              <UBadge :color="priorityColor(node.priority)" size="xs">{{ priorityLabel(node.priority) }}</UBadge>
              <UBadge v-if="getNodeStatus(node.id)" :color="dagStatusColor(getNodeStatus(node.id))" size="xs">
                {{ getNodeStatus(node.id) }}
              </UBadge>
            </div>
          </div>
        </div>

        <div v-if="displayNodes.length === 0" class="absolute inset-0 flex items-center justify-center text-gray-400">
          Select a template or create a new one to visualize the DAG
        </div>
      </div>
    </div>

    <UModal v-model="showCreateModal" class="w-[750px] max-w-[95vw]">
      <template #header>
        <h3 class="text-lg font-bold">{{ editMode ? 'Edit DAG Template' : 'Create DAG Template' }}</h3>
      </template>

      <div class="p-6 space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <UFormLabel>Name *</UFormLabel>
            <UInput v-model="tplForm.name" size="sm" placeholder="My Pipeline" />
          </div>
          <div>
            <UFormLabel>Strategy (failure)</UFormLabel>
            <USelect
              v-model="tplForm.strategy"
              size="sm"
              :options="[
                { value: 'abort', label: 'Abort DAG' },
                { value: 'skip', label: 'Skip & Continue' },
                { value: 'retry', label: 'Retry Node' },
              ]"
            />
          </div>
        </div>
        <div>
          <UFormLabel>Description</UFormLabel>
          <UInput v-model="tplForm.description" size="sm" />
        </div>

        <div class="border rounded-lg p-4 space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="font-semibold text-sm">Nodes</h4>
            <UButton size="2xs" color="blue" variant="outline" @click="addNode">
              <Icon name="i-heroicons-plus-16-solid" class="w-3 h-3 mr-1" />
              Add Node
            </UButton>
          </div>

          <div
            v-for="(node, idx) in tplForm.nodes"
            :key="idx"
            class="p-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg space-y-2"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-mono text-gray-500">ID: {{ node.id }}</span>
              <UButton size="2xs" color="red" variant="ghost" icon="i-heroicons-trash-16-solid" @click="removeNode(idx)" />
            </div>
            <div class="grid grid-cols-2 gap-2">
              <UInput v-model="node.name" placeholder="Node name" size="xs" />
              <UInput v-model="node.task_type" placeholder="Task type *" size="xs" />
            </div>
            <div class="grid grid-cols-2 gap-2">
              <USelect
                v-model="node.priority"
                size="xs"
                :options="[
                  { value: 4, label: 'Critical' },
                  { value: 3, label: 'High' },
                  { value: 2, label: 'Normal' },
                  { value: 1, label: 'Low' },
                  { value: 0, label: 'Bulk' },
                ]"
              />
              <USelect
                v-model="node.strategy"
                size="xs"
                :options="[
                  { value: 'abort', label: 'Abort' },
                  { value: 'skip', label: 'Skip' },
                  { value: 'retry', label: 'Retry' },
                ]"
              />
            </div>
            <div>
              <p class="text-xs text-gray-500 mb-1">Dependencies (hold Cmd/Ctrl for multi-select)</p>
              <select
                v-model="node.dependencies"
                multiple
                class="w-full rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-xs"
              >
                <option v-for="(n, i) in tplForm.nodes" v-if="i !== idx" :key="i" :value="n.id">
                  {{ n.name || n.id }}
                </option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex gap-2 justify-end">
          <UButton variant="ghost" @click="showCreateModal = false">Cancel</UButton>
          <UButton color="green" @click="saveTemplate">{{ editMode ? 'Save Changes' : 'Create Template' }}</UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const templates = ref<any[]>([])
const runs = ref<any[]>([])
const selectedTemplate = ref<any>(null)
const runDetail = ref<any>(null)
const showCreateModal = ref(false)
const editMode = ref(false)

const tplForm = ref({
  name: '',
  description: '',
  strategy: 'abort' as any,
  nodes: [] as any[],
})

const displayNodes = computed<any[]>(() => {
  if (runDetail.value) {
    const tpl = templates.value.find(t => t.id === runDetail.value.template_id)
    return tpl?.nodes || []
  }
  return selectedTemplate.value?.nodes || []
})

const displayEdges = computed<any[]>(() => {
  if (runDetail.value) {
    const tpl = templates.value.find(t => t.id === runDetail.value.template_id)
    return tpl?.edges || []
  }
  return selectedTemplate.value?.edges || []
})

const nodePositions = ref<Record<string, { x: number; y: number }>>({})

function getNodePos(id: string) {
  if (!nodePositions.value[id]) {
    const nodes = displayNodes.value
    const cols = Math.ceil(Math.sqrt(nodes.length))
    const idx = nodes.findIndex(n => n.id === id)
    const col = idx % cols
    const row = Math.floor(idx / cols)
    nodePositions.value[id] = {
      x: 150 + col * 220,
      y: 100 + row * 120,
    }
  }
  return nodePositions.value[id]
}

function getNodeClass(id: string) {
  const status = getNodeStatus(id)
  const base = 'bg-white dark:bg-gray-800'
  if (status === 'success') return `${base} border-green-500`
  if (status === 'running') return `${base} border-amber-500 animate-pulse`
  if (status === 'failed') return `${base} border-red-500`
  if (status === 'skipped') return `${base} border-gray-400 opacity-60`
  return `${base} border-gray-200 dark:border-gray-600`
}

function getNodeStatus(id: string) {
  if (!runDetail.value?.nodes_state) return null
  return runDetail.value.nodes_state[id]?.status || null
}

function selectTemplate(tpl: any) {
  selectedTemplate.value = tpl
  runDetail.value = null
  nodePositions.value = {}
}

async function loadRuns() {
  const { data } = await useDAGRuns({ limit: 50 })
  if (data.value) runs.value = data.value.items || []
}

async function loadTemplates() {
  const { data } = await useDAGTemplates()
  if (data.value) templates.value = data.value || []
  if (templates.value.length > 0 && !selectedTemplate.value) {
    selectTemplate(templates.value[0])
  }
}

async function loadRun(id: string) {
  const { data } = await useDAGRun(id)
  if (data.value) {
    runDetail.value = data.value
    selectedTemplate.value = null
    nodePositions.value = {}
  }
}

function addNode() {
  tplForm.value.nodes.push({
    id: 'node_' + Math.random().toString(36).slice(2, 8),
    name: '',
    task_type: '',
    priority: 2,
    strategy: 'abort',
    dependencies: [],
  })
}

function removeNode(idx: number) {
  tplForm.value.nodes.splice(idx, 1)
}

async function saveTemplate() {
  if (!tplForm.value.name || tplForm.value.nodes.length === 0) {
    alert('Name and at least one node required')
    return
  }
  const nodes = tplForm.value.nodes
  const edges: any[] = []
  nodes.forEach(n => {
    (n.dependencies || []).forEach(depId => {
      if (!edges.find(e => e.from === depId && e.to === n.id)) {
        edges.push({ from: depId, to: n.id })
      }
    })
  })
  try {
    await createDAGTemplate({
      name: tplForm.value.name,
      description: tplForm.value.description,
      nodes: nodes.map(({ id, name, task_type, priority, strategy }) => ({
        id, name, task_type, priority, strategy, dependencies: []
      })),
      edges,
    })
    showCreateModal.value = false
    resetForm()
    loadTemplates()
  } catch (e: any) {
    alert('Save failed: ' + (e.data?.error || e.message))
  }
}

function resetForm() {
  editMode.value = false
  tplForm.value = {
    name: '',
    description: '',
    strategy: 'abort',
    nodes: [],
  }
}

async function runTemplate(id: string) {
  if (!confirm('Run this DAG template?')) return
  try {
    const res = await runDAG(id, { strategy: 'abort', max_retries: 3 })
    alert(`DAG started! Run ID: ${res.run_id}`)
    loadRuns()
  } catch (e: any) {
    alert('Failed to run: ' + (e.data?.error || e.message))
  }
}

onMounted(() => {
  loadTemplates()
  loadRuns()
})
</script>
