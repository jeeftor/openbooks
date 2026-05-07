<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { ScrollText, RefreshCw, Info } from "lucide-vue-next";
import { useLogs } from "../../composables/useApi";
import type { LogEntry } from "../../types/messages";

const { logs, loading, refresh } = useLogs();

// Click-to-toggle detail popup — works on both desktop and mobile.
const activeDetail = ref<{ text: string; x: number; y: number; w: number } | null>(null);

// Group hover — hovering any entry with a group highlights all entries in that group.
const hoveredGroup = ref<string | null>(null);

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
  return new Date(iso).toLocaleTimeString("en-US", { timeStyle: "short" });
}

function toggleDetail(e: MouseEvent | TouchEvent, text: string) {
  e.stopPropagation();
  if (activeDetail.value?.text === text) {
    activeDetail.value = null;
    return;
  }
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const popupW = Math.min(256, window.innerWidth - 16);
  // Clamp left so popup never overflows right edge.
  const x = Math.min(rect.left, window.innerWidth - popupW - 8);
  // Prefer opening below; flip above if not enough space.
  const spaceBelow = window.innerHeight - rect.bottom;
  const y = spaceBelow < 130 ? rect.top - 130 : rect.bottom + 6;
  activeDetail.value = { text, x, y, w: popupW };
}

function closeDetail() {
  activeDetail.value = null;
}

onMounted(() => document.addEventListener("click", closeDetail));
onUnmounted(() => document.removeEventListener("click", closeDetail));
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
        class="px-3 py-1.5 flex gap-1.5 items-start transition-colors"
        :class="entry.group && hoveredGroup === entry.group
          ? 'bg-brand-50 dark:bg-brand-900/20 border-l-2 border-l-brand-400'
          : 'hover:bg-slate-50 dark:hover:bg-slate-800/30 border-l-2 border-l-transparent'"
        @mouseenter="entry.group ? hoveredGroup = entry.group : null"
        @mouseleave="hoveredGroup = null">
        <span
          class="flex-shrink-0 text-[10px] text-slate-400 dark:text-slate-500 mt-px tabular-nums whitespace-nowrap">
          {{ formatTime(entry.time) }}
        </span>
        <span
          v-if="entry.level !== 'info'"
          class="flex-shrink-0 text-[10px] uppercase font-semibold mt-px"
          :class="levelClass(entry.level)">
          {{ entry.level }}
        </span>
        <span
          class="text-[11px] leading-relaxed min-w-0 truncate flex-1"
          :class="entry.level === 'info' ? 'text-slate-700 dark:text-slate-300' : levelClass(entry.level)"
          :title="entry.detail ? undefined : entry.message">
          {{ entry.message }}
        </span>
        <button
          v-if="entry.detail"
          class="flex-shrink-0 mt-px transition-colors"
          :class="activeDetail?.text === entry.detail
            ? 'text-brand-400'
            : 'text-slate-300 dark:text-slate-600 hover:text-brand-400 dark:hover:text-brand-400'"
          title="Show details"
          @click.stop="toggleDetail($event, entry.detail)"
          @touchend.stop.prevent="toggleDetail($event, entry.detail)">
          <Info :size="11" />
        </button>
      </li>
    </ul>

    <!-- Detail popup — click ℹ to open, click anywhere to close -->
    <Teleport to="body">
      <div
        v-if="activeDetail"
        class="fixed z-[9999] p-3 rounded-lg shadow-xl bg-slate-900 border border-slate-700"
        :style="{ left: activeDetail.x + 'px', top: activeDetail.y + 'px', width: activeDetail.w + 'px' }"
        @click.stop>
        <pre class="text-[10px] text-slate-200 whitespace-pre-wrap font-mono leading-relaxed">{{ activeDetail.text }}</pre>
      </div>
    </Teleport>
  </div>
</template>
