<script setup lang="ts">
import { ScrollText, RefreshCw } from "lucide-vue-next";
import { useLogs } from "../../composables/useApi";
import type { LogEntry } from "../../types/messages";

const { logs, loading, refresh } = useLogs();

function levelClass(level: LogEntry["level"]) {
  switch (level) {
    case "error":
      return "text-red-500 dark:text-red-400";
    case "warn":
      return "text-amber-500 dark:text-amber-400";
    default:
      return "text-slate-400 dark:text-slate-500";
  }
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString("en-US", { timeStyle: "medium" });
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <div
      class="flex-shrink-0 flex items-center justify-between px-3 py-2 border-b border-slate-100 dark:border-slate-800">
      <span class="text-xs font-medium text-slate-500 dark:text-slate-400">
        {{ logs.length }} entries
      </span>
      <button
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
        :class="{ 'animate-spin': loading }"
        title="Refresh"
        @click="refresh()">
        <RefreshCw :size="13" />
      </button>
    </div>

    <!-- Loading -->
    <div
      v-if="loading && !logs.length"
      class="flex-1 flex items-center justify-center">
      <RefreshCw :size="20" class="animate-spin text-slate-300 dark:text-slate-600" />
    </div>

    <!-- Empty -->
    <div
      v-else-if="!logs.length"
      class="flex-1 flex flex-col items-center justify-center gap-2 text-center px-4">
      <ScrollText :size="28" class="text-slate-300 dark:text-slate-600" />
      <p class="text-xs text-slate-400 dark:text-slate-500">No log entries.</p>
    </div>

    <!-- Entries -->
    <ul
      v-else
      class="flex-1 overflow-y-auto divide-y divide-slate-100/60 dark:divide-slate-800/40 font-mono">
      <li
        v-for="(entry, i) in logs"
        :key="i"
        class="px-3 py-1.5 flex gap-2 items-start hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
        <span
          class="flex-shrink-0 text-[10px] text-slate-400 dark:text-slate-500 mt-px tabular-nums">
          {{ formatTime(entry.time) }}
        </span>
        <span
          class="flex-shrink-0 text-[10px] uppercase font-semibold w-8 mt-px"
          :class="levelClass(entry.level)">
          {{ entry.level }}
        </span>
        <span
          class="text-[11px] text-slate-700 dark:text-slate-300 leading-relaxed break-words min-w-0">
          {{ entry.message }}
        </span>
      </li>
    </ul>
  </div>
</template>
