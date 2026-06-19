<template>
  <div class="space-y-8">
    <section>
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold flex items-center gap-2">
          <Icon name="i-heroicons-fire-16-solid" class="w-5 h-5 text-orange-500" />
          耗时热力图
        </h3>
        <p class="text-sm text-gray-500">按小时粒度展示P95耗时分布</p>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <DurationHeatmap />
      </div>
    </section>

    <section>
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold flex items-center gap-2">
          <Icon name="i-heroicons-chart-bar-16-solid" class="w-5 h-5 text-blue-500" />
          耗时分布直方图
        </h3>
        <p class="text-sm text-gray-500">统计各耗时区间的任务数量与占比</p>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <DurationHistogram />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import DurationHeatmap from './DurationHeatmap.vue'
import DurationHistogram from './DurationHistogram.vue'
</script>
