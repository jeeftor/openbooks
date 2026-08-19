<script setup lang="ts">
import { ref, computed } from "vue";
import {
  ChevronUp, ChevronDown,
  Loader, CircleCheck, CircleAlert, TriangleAlert, Clock
} from "lucide-vue-next";
import { useTaskStore, type Task } from "../../stores/tasks";
import LogsPanel from "../sidebar/LogsPanel.vue";

const taskStore = useTaskStore();
const expanded = ref(false);
const showLogs = ref(false);

const hasAny = computed(() => taskStore.tasks.length > 0);

const primaryTask = computed(() =>
  taskStore.activeTasks[0] ?? taskStore.recentTasks[0]
);

const completedCount = computed(() =>
  taskStore.tasks.filter(t => t.status !== "active" && t.status !== "queued").length
);

function typeEmoji(type: Task["type"]) {
  return type === "search" ? "🔍" : type === "download" ? "⬇" : "💾";
}

function latestEvent(task: Task): string {
  return task.events.at(-1)?.message ?? task.phase ?? task.status;
}

function phaseLabel(task: Task): string {
  if (task.status === "queued") return "Queued";
  if (task.status === "active") {
    if (task.phase === "waiting") return `Waiting for ${task.meta?.bot ?? "bot"}`;
    if (task.phase === "transferring") return "Transferring…";
    if (task.phase === "cleaning") return "Processing…";
    if (task.phase === "rename") return "Awaiting rename";
    if (task.phase === "staged") return "Saved for later";
    return latestEvent(task);
  }
  return latestEvent(task);
}
</script>

<template>
  <!-- Only render when there are tasks -->
  <Transition name="task-panel-fade">
    <div
      v-if="hasAny"
      data-testid="task-panel"
      class="fixed z-40"
      :class="[
        'bottom-4 right-4',
        'max-sm:right-2 max-sm:left-2',
        expanded ? '' : ''
      ]">

      <!-- ── Collapsed pill ── -->
      <Transition name="task-flip" mode="out-in">
        <button
          v-if="!expanded"
          key="pill"
          class="flex items-center gap-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-full px-3 py-1.5 shadow-lg hover:shadow-xl text-xs font-medium transition-shadow max-w-[240px] max-sm:max-w-full"
          @click="expanded = true">
          <Loader
            v-if="taskStore.activeCount > 0"
            :size="11"
            class="text-brand-400 animate-spin flex-shrink-0" />
          <CircleCheck
            v-else
            :size="11"
            class="text-slate-400 flex-shrink-0" />
          <span class="truncate text-slate-600 dark:text-slate-300">
            <template v-if="primaryTask">
              {{ typeEmoji(primaryTask.type) }} {{ primaryTask.label }}
            </template>
          </span>
          <span
            v-if="taskStore.activeCount > 0"
            class="flex-shrink-0 text-[9px] font-bold px-1.5 py-px rounded-full bg-brand-100 dark:bg-brand-900/40 text-brand-600 dark:text-brand-400">
            {{ taskStore.activeCount }} live
          </span>
          <ChevronUp :size="11" class="text-slate-400 flex-shrink-0" />
        </button>

        <!-- ── Expanded panel ── -->
        <div
          v-else
          key="panel"
          class="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-2xl shadow-2xl overflow-hidden w-80 max-sm:w-full">

          <!-- Panel header -->
          <div class="flex items-center justify-between px-3 py-2 border-b border-slate-100 dark:border-slate-700/60">
            <span class="text-xs font-semibold text-slate-700 dark:text-slate-200">Activity</span>
            <div class="flex items-center gap-1.5">
              <span
                v-if="taskStore.activeCount > 0"
                class="text-[10px] px-1.5 py-px rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 font-bold">
                {{ taskStore.activeCount }} live
              </span>
              <button
                v-if="completedCount > 0"
                class="text-[10px] text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors px-1"
                @click="taskStore.clearCompleted()">
                Clear
              </button>
              <button
                class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-400 transition-colors"
                @click="expanded = false">
                <ChevronDown :size="12" />
              </button>
            </div>
          </div>

          <!-- Task list -->
          <div class="max-h-64 overflow-y-auto">
            <div
              v-for="task in taskStore.recentTasks"
              :key="task.id"
              class="flex items-start gap-2.5 px-3 py-2 border-b border-slate-50 dark:border-slate-700/40 last:border-0">

              <div class="mt-0.5 flex-shrink-0">
                <Loader v-if="task.status === 'active'" :size="12" class="text-brand-400 animate-spin" />
                <Clock v-else-if="task.status === 'queued'" :size="12" class="text-slate-400" />
                <CircleCheck v-else-if="task.status === 'done'" :size="12" class="text-green-500" />
                <CircleAlert v-else-if="task.status === 'failed'" :size="12" class="text-red-500" />
                <TriangleAlert v-else-if="task.status === 'timed-out'" :size="12" class="text-amber-500" />
              </div>

              <div class="flex-1 min-w-0">
                <div class="flex items-baseline justify-between gap-1">
                  <p class="text-xs font-medium text-slate-700 dark:text-slate-200 truncate">
                    {{ typeEmoji(task.type) }} {{ task.label }}
                  </p>
                </div>
                <p class="text-[10px] text-slate-400 dark:text-slate-500 truncate mt-px">
                  {{ phaseLabel(task) }}
                </p>
              </div>
            </div>

            <div v-if="taskStore.tasks.length === 0" class="px-3 py-4 text-center text-xs text-slate-400">
              No activity yet
            </div>
          </div>

          <!-- Server logs toggle -->
          <div class="border-t border-slate-100 dark:border-slate-700/60">
            <button
              class="w-full text-[10px] text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 py-1.5 flex items-center justify-center gap-1 transition-colors"
              @click="showLogs = !showLogs">
              {{ showLogs ? "▲ Hide server logs" : "▼ Server logs" }}
            </button>
            <LogsPanel
              v-if="showLogs"
              class="border-t border-slate-100 dark:border-slate-700/60 max-h-36" />
          </div>
        </div>
      </Transition>
    </div>
  </Transition>
</template>

<style scoped>
.task-panel-fade-enter-active,
.task-panel-fade-leave-active { transition: opacity 0.2s ease; }
.task-panel-fade-enter-from,
.task-panel-fade-leave-to { opacity: 0; }

.task-flip-enter-active,
.task-flip-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.task-flip-enter-from { opacity: 0; transform: translateY(4px); }
.task-flip-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
