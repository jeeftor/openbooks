<script setup lang="ts">
import { ref, computed } from "vue";
import {
  Activity,
  ChevronDown,
  Clock,
  CircleCheck,
  CircleAlert,
  TriangleAlert,
  Loader,
  Terminal
} from "lucide-vue-next";
import { useTaskStore } from "../../stores/tasks";
import type { Task } from "../../stores/tasks";
import LogsPanel from "./LogsPanel.vue";
import EmptyPanel from "../shared/EmptyPanel.vue";
import PanelHeader from "../shared/PanelHeader.vue";

const taskStore = useTaskStore();

const expandedIds = ref(new Set<string>());
const showRawLogs = ref(false);

const completedTasks = computed(() =>
  taskStore.tasks.filter(t => t.status !== 'active' && t.status !== 'queued')
);

function taskTypeLabel(type: Task['type']): string {
  switch (type) {
    case 'search': return 'Search';
    case 'download': return 'Download';
    case 'save': return 'Save';
    default: return type;
  }
}

function latestEvent(task: Task): string {
  if (task.events.length === 0) {
    return task.phase ?? task.status;
  }
  return task.events[task.events.length - 1].message;
}

function formatAge(ms: number): string {
  const diffSec = Math.floor((Date.now() - ms) / 1000);
  if (diffSec < 60) return 'just now';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  return `${Math.floor(diffMin / 60)}h ago`;
}

function formatTime(ms: number): string {
  return new Date(ms).toLocaleTimeString('en-US', { timeStyle: 'short' });
}

function toggleExpand(id: string) {
  if (expandedIds.value.has(id)) {
    expandedIds.value.delete(id);
  } else {
    expandedIds.value.add(id);
  }
  expandedIds.value = new Set(expandedIds.value);
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <PanelHeader>
      Activity
      <template #actions>
        <div class="flex items-center gap-2">
          <span
            v-if="taskStore.activeCount > 0"
            class="text-xs bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 px-1.5 py-0.5 rounded-full">
            {{ taskStore.activeCount }} live
          </span>
          <button
            v-if="completedTasks.length > 0"
            class="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
            @click="taskStore.clearCompleted()">
            Clear
          </button>
        </div>
      </template>
    </PanelHeader>

    <!-- Task list -->
    <div class="flex-1 overflow-y-auto">
      <!-- Empty state -->
      <EmptyPanel v-if="taskStore.tasks.length === 0" :icon="Activity">
        No activity yet
      </EmptyPanel>

      <!-- Task items -->
      <div
        v-for="task in taskStore.recentTasks"
        :key="task.id"
        class="border-b border-slate-50 dark:border-slate-800/50 last:border-0">
        <button
          class="w-full text-left px-3 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
          @click="toggleExpand(task.id)">
          <div class="flex items-start gap-2">
            <!-- Status icon -->
            <div class="mt-0.5 flex-shrink-0">
              <Loader v-if="task.status === 'active'" :size="13" class="text-blue-500 animate-spin" />
              <Clock v-else-if="task.status === 'queued'" :size="13" class="text-slate-400" />
              <CircleCheck v-else-if="task.status === 'done'" :size="13" class="text-green-500" />
              <CircleAlert v-else-if="task.status === 'failed'" :size="13" class="text-red-500" />
              <TriangleAlert v-else-if="task.status === 'timed-out'" :size="13" class="text-amber-500" />
            </div>
            <!-- Content -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-1">
                <span class="text-xs font-medium text-slate-700 dark:text-slate-200 truncate">
                  {{ taskTypeLabel(task.type) }}: {{ task.label }}
                </span>
                <span class="text-[10px] text-slate-400 flex-shrink-0">{{ formatAge(task.updatedAt) }}</span>
              </div>
              <div class="text-[11px] text-slate-400 dark:text-slate-500 truncate mt-0.5">
                {{ latestEvent(task) }}
              </div>
            </div>
            <ChevronDown
              :size="11"
              class="text-slate-300 mt-0.5 transition-transform flex-shrink-0"
              :class="expandedIds.has(task.id) ? 'rotate-180' : ''" />
          </div>
        </button>
        <!-- Expanded events -->
        <div v-if="expandedIds.has(task.id)" class="px-3 pb-2 space-y-1">
          <div
            v-for="event in task.events"
            :key="event.time"
            class="flex items-start gap-2 pl-5">
            <span class="text-[10px] text-slate-400 flex-shrink-0 mt-0.5">{{ formatTime(event.time) }}</span>
            <span
              class="text-[11px]"
              :class="{
                'text-slate-500 dark:text-slate-400': event.level === 'info',
                'text-amber-600 dark:text-amber-400': event.level === 'warn',
                'text-red-600 dark:text-red-400': event.level === 'error',
              }">{{ event.message }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Raw logs link -->
    <div class="flex-shrink-0 border-t border-slate-100 dark:border-slate-800">
      <button
        class="w-full text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 py-2 flex items-center justify-center gap-1 transition-colors"
        @click="showRawLogs = !showRawLogs">
        <Terminal :size="11" />
        {{ showRawLogs ? 'Hide server logs' : 'Show server logs' }}
      </button>
      <LogsPanel
        v-if="showRawLogs"
        class="border-t border-slate-100 dark:border-slate-800 max-h-48" />
    </div>
  </div>
</template>
