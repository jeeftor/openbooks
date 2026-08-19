<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from "vue";
import { Plus, X, Library, Loader, WifiOff, Search, Clock } from "lucide-vue-next";
import { useHistoryStore } from "../../stores/history";
import { useAppStore } from "../../stores/app";
import { useTaskStore } from "../../stores/tasks";
import type { HistoryItem } from "../../types/messages";

const props = defineProps<{ libraryOpen: boolean }>();
const emit = defineEmits<{ toggleLibrary: [] }>();

const historyStore = useHistoryStore();
const appStore = useAppStore();
const taskStore = useTaskStore();
const scrollEl = ref<HTMLElement | null>(null);

// The query currently being IRC-searched (status === 'active' in task store).
const activeSearchQuery = computed<string | undefined>(() => {
  const t = taskStore.tasks.find(t => t.type === 'search' && t.status === 'active');
  return t?.meta?.query as string | undefined;
});

// Reactive clock for SVG countdown ring — ticks every second.
const now = ref(Date.now());
let _clockInterval: ReturnType<typeof setInterval> | null = null;
onMounted(() => { _clockInterval = setInterval(() => { now.value = Date.now(); }, 1000); });
onUnmounted(() => { if (_clockInterval) clearInterval(_clockInterval); });

const SEARCH_TIMEOUT_MS = 60_000;
const SVG_CIRCUMFERENCE = 25.13; // 2π * r (r=4, 12×12 viewBox)

function activeTaskForQuery(query: string) {
  return taskStore.tasks.find(t => t.type === 'search' && t.status === 'active' && t.meta?.query === query);
}

/** Progress 0→1 as the 60s timeout drains. */
function searchProgress(item: HistoryItem): number {
  const task = activeTaskForQuery(item.query);
  if (!task?.activeAt) return 0;
  return Math.min((now.value - task.activeAt) / SEARCH_TIMEOUT_MS, 1);
}

/** Seconds remaining for the countdown tooltip. */
function secondsRemaining(item: HistoryItem): number {
  return Math.max(0, Math.ceil(SEARCH_TIMEOUT_MS / 1000 - (now.value - (activeTaskForQuery(item.query)?.activeAt ?? now.value)) / 1000));
}

type TabStatus = "loading" | "done" | "timeout";
type TabAge = "fresh" | "aging" | "stale";

function tabStatus(item: HistoryItem): TabStatus {
  if (item.timedOut) return "timeout";
  // results are stored in resultsCache, not on items[] directly
  if (historyStore.getCachedResults(item.timestamp) !== undefined) return "done";
  return "loading";
}

function tabResultCount(item: HistoryItem): number {
  return historyStore.getCachedResults(item.timestamp)?.results.length ?? 0;
}

/** Age bucket based on how long ago the search completed. */
function tabAge(item: HistoryItem): TabAge {
  const ageMs = Date.now() - item.timestamp;
  if (ageMs < 30 * 60 * 1000) return "fresh";
  if (ageMs < 2 * 60 * 60 * 1000) return "aging";
  return "stale";
}

function tabTimestamp(item: HistoryItem): string {
  return new Date(item.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function isActive(item: HistoryItem) {
  return appStore.activeItem?.timestamp === item.timestamp;
}

function selectTab(item: HistoryItem) {
  const cached = historyStore.getCachedResults(item.timestamp);
  if (cached) {
    historyStore.restoreItem(item);
    return;
  }
  // Already queued or searching — just switch the view, don't re-send.
  const inFlight = taskStore.tasks.some(t =>
    t.type === 'search' &&
    (t.status === 'active' || t.status === 'queued') &&
    t.meta?.query === item.query
  );
  if (inFlight) {
    appStore.setActiveItem(item);
  } else {
    appStore.pendingQuery = item.query;
  }
}

function closeTab(e: Event, item: HistoryItem) {
  e.stopPropagation();
  historyStore.deleteItem(item.timestamp);
}

function focusSearch() {
  const input = document.querySelector<HTMLInputElement>('input[type="search"]');
  if (input) { input.focus(); input.select(); }
}

// Auto-scroll newly added tab into view
watch(
  () => historyStore.items[0]?.timestamp,
  async () => {
    await nextTick();
    scrollEl.value?.scrollTo({ left: 0, behavior: "smooth" });
  }
);
</script>

<template>
  <div class="flex-shrink-0 flex items-stretch border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/70 min-h-[36px]">

    <!-- Scrollable tab area -->
    <div
      ref="scrollEl"
      class="flex-1 flex items-end overflow-x-auto tab-scroll gap-px px-2 pt-1"
      style="-webkit-overflow-scrolling: touch">

      <!-- Empty state nudge -->
      <div
        v-if="!historyStore.items.length"
        class="flex items-center gap-1.5 px-3 text-xs text-slate-400 dark:text-slate-600 italic pb-1.5 select-none">
        <Search :size="11" />
        Search to start
      </div>

      <!-- One tab per search session -->
      <button
        v-for="item in historyStore.items"
        :key="item.timestamp"
        data-testid="search-tab"
        :data-query="item.query"
        class="group flex-shrink-0 flex items-center gap-1.5 px-3 pb-1.5 pt-1 rounded-t-md text-[11px] font-medium transition-all max-w-[180px] border-x border-t"
        :class="isActive(item)
          ? 'bg-white dark:bg-slate-950 border-slate-200 dark:border-slate-700 text-slate-900 dark:text-slate-50 shadow-sm relative z-10'
          : 'bg-transparent border-transparent text-slate-500 dark:text-slate-400 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-slate-700 dark:hover:text-slate-200'"
        :title="tabStatus(item) === 'loading'
          ? (activeSearchQuery === item.query
              ? `Searching for &quot;${item.query}&quot; — ${secondsRemaining(item)}s remaining`
              : `&quot;${item.query}&quot; — queued, waiting its turn`)
          : `&quot;${item.query}&quot; · searched at ${tabTimestamp(item)}`"
        @click="selectTab(item)">

        <!-- SVG countdown ring: depletes over 60s while actively searching -->
        <svg
          v-if="tabStatus(item) === 'loading' && activeSearchQuery === item.query"
          class="-rotate-90 flex-shrink-0"
          width="12" height="12" viewBox="0 0 12 12">
          <circle cx="6" cy="6" r="4" fill="none" stroke-width="2"
            class="stroke-slate-300 dark:stroke-slate-600" />
          <circle cx="6" cy="6" r="4" fill="none" stroke-width="2"
            class="stroke-brand-400"
            :stroke-dasharray="SVG_CIRCUMFERENCE"
            :stroke-dashoffset="SVG_CIRCUMFERENCE * searchProgress(item)"
            style="transition: stroke-dashoffset 1s linear" />
        </svg>
        <!-- Clock = queued (waiting its turn) -->
        <Clock
          v-else-if="tabStatus(item) === 'loading'"
          :size="10"
          class="flex-shrink-0 text-slate-400" />
        <WifiOff
          v-else-if="tabStatus(item) === 'timeout'"
          :size="10"
          class="flex-shrink-0 text-red-400" />
        <!-- Done: result count badge, colored by age (fresh=brand, aging=amber, stale=red) -->
        <span
          v-else
          data-testid="tab-badge"
          class="flex-shrink-0 text-[9px] font-bold tabular-nums px-1 py-px rounded"
          :class="tabAge(item) === 'stale'
            ? 'bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400'
            : tabAge(item) === 'aging'
              ? 'bg-amber-100 dark:bg-amber-900/40 text-amber-600 dark:text-amber-400'
              : isActive(item)
                ? 'bg-brand-100 dark:bg-brand-900/40 text-brand-600 dark:text-brand-400'
                : 'bg-slate-200 dark:bg-slate-700 text-slate-500 dark:text-slate-400 group-hover:bg-slate-300 dark:group-hover:bg-slate-600'">
          {{ tabResultCount(item) }}
        </span>

        <!-- Query label -->
        <span data-testid="tab-query" class="truncate">{{ item.query }}</span>

        <!-- Age clock: appears on aging/stale results -->
        <Clock
          v-if="tabStatus(item) === 'done' && tabAge(item) !== 'fresh'"
          :size="8"
          class="flex-shrink-0 opacity-60"
          :class="tabAge(item) === 'stale' ? 'text-red-400' : 'text-amber-400'" />

        <!-- Close -->
        <span
          class="flex-shrink-0 p-px rounded opacity-0 group-hover:opacity-70 hover:!opacity-100 hover:bg-slate-200 dark:hover:bg-slate-700 transition-opacity -mr-1"
          @click.stop="closeTab($event, item)">
          <X :size="9" />
        </span>
      </button>

      <!-- New search button -->
      <button
        class="flex-shrink-0 flex items-center gap-1 px-2 pb-1.5 pt-1 rounded-t-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-white/70 dark:hover:bg-slate-800/60 transition-colors"
        title="New search (or just start typing)"
        @click="focusSearch">
        <Plus :size="11" />
      </button>
    </div>

    <!-- Library toggle (right-anchored) -->
    <button
      class="flex-shrink-0 flex items-center gap-1.5 px-3 text-[11px] font-medium border-l border-slate-200 dark:border-slate-800 transition-colors"
      :class="libraryOpen
        ? 'text-brand-600 dark:text-brand-400 bg-brand-50 dark:bg-brand-950/40'
        : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-white/60 dark:hover:bg-slate-800/50'"
      title="Downloaded books"
      @click="emit('toggleLibrary')">
      <Library :size="13" />
      <span class="hidden sm:block">Library</span>
    </button>
  </div>
</template>

<style scoped>
.tab-scroll::-webkit-scrollbar { display: none; }
.tab-scroll { scrollbar-width: none; }
</style>
