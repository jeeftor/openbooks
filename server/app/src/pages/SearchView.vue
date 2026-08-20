<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { Search, Loader, Wifi, WifiOff, Download, RefreshCw, Clock } from "lucide-vue-next";
import { useAppStore } from "../stores/app";
import { useHistoryStore } from "../stores/history";
import { useTaskStore } from "../stores/tasks";
import { sendMessage, registerSearchTask, markSearchTimedOut } from "../composables/useWebSocket";
import { MessageType } from "../types/messages";
import type { HistoryItem } from "../types/messages";
import { useServers } from "../composables/useApi";
import BookTable from "../components/books/BookTable.vue";
import BookCards from "../components/books/BookCards.vue";
import ErrorTable from "../components/errors/ErrorTable.vue";
import EmptyState from "../components/search/EmptyState.vue";

const appStore = useAppStore();
const historyStore = useHistoryStore();
const taskStore = useTaskStore();
const { servers, isFresh, refresh: refreshServers } = useServers();
const isMobile = useMediaQuery("(max-width: 767px)");

const query = ref("");
const showErrors = ref(false);
const isSearching = ref(false);
// Autocomplete dropdown state
const showSuggestions = ref(false);
const selectedSuggestion = ref(-1);
// Tracks which query the active searchTimeout is guarding (cleared alongside the timeout).
let timedOutQuery: string | null = null;
let searchTimeout: ReturnType<typeof setTimeout> | null = null;

const SEARCH_TIMEOUT_MS = 60_000;

// Task for the query currently being IRC-searched.
const activeSearchTask = computed(() =>
  taskStore.tasks.find(t =>
    t.type === 'search' && t.status === 'active' &&
    t.meta?.query === appStore.activeItem?.query
  )
);
// Task for the query queued but not yet searching.
const queuedSearchTask = computed(() =>
  taskStore.tasks.find(t =>
    t.type === 'search' && t.status === 'queued' &&
    t.meta?.query === appStore.activeItem?.query
  )
);
// Queue position (1-based) for the current queued item.
const queuePosition = computed(() => {
  const q = appStore.activeItem?.query;
  if (!q) return 0;
  const queued = taskStore.tasks
    .filter(t => t.type === 'search' && (t.status === 'active' || t.status === 'queued'))
    .reverse(); // oldest-first
  return queued.findIndex(t => t.meta?.query === q) + 1;
});

// Reactive clock for the countdown (only ticks while an active search is running).
const now = ref(Date.now());
let _nowInterval: ReturnType<typeof setInterval> | null = null;
watch(
  () => !!activeSearchTask.value,
  (active) => {
    if (active && !_nowInterval) {
      _nowInterval = setInterval(() => { now.value = Date.now(); }, 1000);
    } else if (!active && _nowInterval) {
      clearInterval(_nowInterval);
      _nowInterval = null;
    }
  },
  { immediate: true }
);

// Progress 0→1 as 60s timeout drains.
const searchProgressPct = computed(() => {
  const task = activeSearchTask.value;
  if (!task?.activeAt) return 0;
  return Math.min((now.value - task.activeAt) / SEARCH_TIMEOUT_MS, 1) * 100;
});
const secondsRemaining = computed(() => {
  const task = activeSearchTask.value;
  if (!task?.activeAt) return SEARCH_TIMEOUT_MS / 1000;
  return Math.max(0, Math.ceil((SEARCH_TIMEOUT_MS - (now.value - task.activeAt)) / 1000));
});

const hasErrors = computed(
  () => (appStore.activeItem?.errors?.length ?? 0) > 0
);
const isTimedOut = computed(() => appStore.activeItem?.timedOut === true);
const resultCount = computed(() => appStore.activeItem?.results?.length ?? 0);
const onlineCount = computed(() => {
  const results = appStore.activeItem?.results;
  if (!results || !servers.value.length) return 0;
  return new Set(results.filter(b => servers.value.includes(b.server)).map(b => b.server)).size;
});
const rawResults = computed(() => {
  const timestamp = appStore.activeItem?.timestamp;
  return timestamp ? appStore.rawSearchResults[timestamp] : undefined;
});
const errorMode = computed(() => showErrors.value && !!appStore.activeItem);
const isShowingCachedResults = computed(() => {
  const ts = appStore.activeItem?.timestamp;
  return !isSearching.value
    && ts !== undefined
    && appStore.activeItem?.results !== undefined
    && (historyStore.getCachedResults(ts) !== undefined || !!appStore.activeItem?.cachedAt);
});

// Age of cached results in minutes.
// Prefers server-supplied cachedAt; falls back to item.timestamp (when search completed).
// Returns null only when there's nothing to show age for.
const cachedResultsAgeMinutes = computed(() => {
  const ca = appStore.activeItem?.cachedAt;
  if (ca) {
    const ms = new Date(ca).getTime();
    if (!isNaN(ms)) return (Date.now() - ms) / 60000;
  }
  // Session-local: derive from timestamp, but only show if > 2 minutes old
  const ts = appStore.activeItem?.timestamp;
  if (ts && isShowingCachedResults.value) {
    const mins = (Date.now() - ts) / 60000;
    return mins > 2 ? mins : null;
  }
  return null;
});

function formatCacheAge(minutes: number): string {
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${Math.round(minutes)}m ago`;
  if (minutes < 1440) return `${Math.round(minutes / 60)}h ago`;
  return `${Math.round(minutes / 1440)}d ago`;
}

// ── Search input autocomplete ──────────────────────────────────────────────────
// Filters history items by the typed query, showing relative age and allowing
// quick-load of cached results (or re-search if not cached).

const suggestions = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q || q.length < 2) return [];
  return historyStore.items
    .filter(item => item.query.includes(q))
    .slice(0, 8);
});

function formatAge(timestamp: number): string {
  const mins = (Date.now() - timestamp) / 60000;
  if (mins < 1) return "just now";
  if (mins < 60) return `${Math.round(mins)}m ago`;
  if (mins < 1440) return `${Math.round(mins / 60)}h ago`;
  return `${Math.round(mins / 1440)}d ago`;
}

function selectSuggestion(item: { query: string; timestamp: number; timedOut?: boolean }) {
  const cached = historyStore.getCachedResults(item.timestamp);
  if (cached) {
    historyStore.restoreItem(item as HistoryItem);
  } else {
    appStore.pendingQuery = item.query;
  }
  query.value = "";
  showSuggestions.value = false;
  selectedSuggestion.value = -1;
}

function onSearchKeydown(e: KeyboardEvent) {
  if (!showSuggestions.value || suggestions.value.length === 0) return;
  if (e.key === "ArrowDown") {
    e.preventDefault();
    selectedSuggestion.value = Math.min(selectedSuggestion.value + 1, suggestions.value.length - 1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    selectedSuggestion.value = Math.max(selectedSuggestion.value - 1, 0);
  } else if (e.key === "Enter" && selectedSuggestion.value >= 0) {
    e.preventDefault();
    selectSuggestion(suggestions.value[selectedSuggestion.value]);
  } else if (e.key === "Escape") {
    showSuggestions.value = false;
    selectedSuggestion.value = -1;
  }
}

function onSearchFocus() {
  if (suggestions.value.length > 0) {
    showSuggestions.value = true;
  }
}

function onSearchBlur() {
  // Delay to allow suggestion click to register before the dropdown closes.
  setTimeout(() => {
    showSuggestions.value = false;
    selectedSuggestion.value = -1;
  }, 150);
}

// Show/hide suggestions as the user types.
watch(
  () => query.value,
  (q) => {
    showSuggestions.value = q.trim().length >= 2 && suggestions.value.length > 0;
    selectedSuggestion.value = -1;
  }
);

const validInput = computed(() => {
  if (!appStore.isConnected) return false;
  return errorMode.value ? query.value.startsWith("!") : query.value.trim() !== "";
});

const searchPlaceholder = computed(() => {
  if (errorMode.value) return "Enter download command (starts with !)";
  const active = appStore.activeItem?.query;
  return active ? `Showing: \u201c${active}\u201d \u2014 type to search again` : "Search for a book\u2026";
});

watch(
  () => appStore.activeItem?.timestamp,
  () => {
    showErrors.value = false;
    // Refresh server list when switching searches (e.g., from history)
    refreshServers();
  }
);

// Re-issue a search triggered from outside (e.g. clicking a timed-out history item)
watch(
  () => appStore.pendingQuery,
  (q) => {
    if (q && appStore.isConnected) {
      appStore.pendingQuery = null;
      // Already searching for this exact query — just switch the view, don't re-queue.
      if (isSearching.value && appStore.activeItem?.query === q) {
        // nothing to do, already in progress
      } else {
        issueSearch(q);
      }
    }
  }
);

// Watch for results arriving to clear the searching state.
// Only clear the timeout when the results are for the query THIS timer is guarding —
// switching to a completed tab must not cancel the timeout for an unrelated in-flight search.
watch(
  () => appStore.activeItem,
  (item) => {
    if (item?.results !== undefined && item.query === timedOutQuery) {
      isSearching.value = false;
      if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = null;
      }
      timedOutQuery = null;
    }
  },
  { deep: false }
);

onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout);
  if (_nowInterval) clearInterval(_nowInterval);
  timedOutQuery = null;
});

function issueSearch(q: string) {
  const normalized = q.trim().toLowerCase();

  // Reuse existing results or in-flight search for the same (normalized) query.
  const existing = historyStore.items.find(i => i.query === normalized);
  if (existing) {
    const cached = historyStore.getCachedResults(existing.timestamp);
    if (cached) {
      historyStore.restoreItem(existing);
      return;
    }
    const inFlight = taskStore.tasks.some(t =>
      t.type === 'search' && (t.status === 'active' || t.status === 'queued') &&
      t.meta?.query === normalized
    );
    if (inFlight) {
      appStore.setActiveItem(existing);
      return;
    }
  }

  const timestamp = Date.now();
  appStore.setActiveItem({ query: normalized, timestamp });
  historyStore.addItem({ query: normalized, timestamp });
  registerSearchTask(normalized);
  sendMessage({ type: MessageType.SEARCH, payload: { query: normalized } });
  isSearching.value = true;
  timedOutQuery = normalized;

  // Set a 60s timeout — if no results arrive, mark as failed.
  // Passes the query so markSearchTimedOut can verify it's still the right entry.
  if (searchTimeout) clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    const active = appStore.activeItem;
    if (active && active.query === normalized && active.results === undefined) {
      const timedOut = { ...active, results: [], errors: [], timedOut: true };
      appStore.setActiveItem(timedOut);
      historyStore.updateItem(timedOut);
    }
    markSearchTimedOut(normalized);
    isSearching.value = false;
    timedOutQuery = null;
    searchTimeout = null;
  }, 60000);
}

function retrySearch() {
  const active = appStore.activeItem;
  if (active?.query) {
    historyStore.clearCachedResults(active.timestamp);
    issueSearch(active.query);
  }
}

function downloadRawResults() {
  const active = appStore.activeItem;
  if (!active || !rawResults.value) return;

  const safeQuery = active.query
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 60) || "search";
  const blob = new Blob([rawResults.value], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `openbooks-abs-${safeQuery}-raw-results.txt`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function handleSearch(e: Event) {
  e.preventDefault();
  if (!validInput.value) return;

  if (errorMode.value) {
    sendMessage({ type: MessageType.DOWNLOAD, payload: { book: query.value } });
  } else {
    const q = query.value.trim();
    issueSearch(q);
  }
  query.value = "";
}
</script>

<template>
  <div class="flex-1 flex flex-col overflow-hidden">
    <!-- Connection status banner (hidden when connected) -->
    <div
      v-if="!appStore.isConnected"
      class="flex-shrink-0 flex items-center gap-2 px-4 py-2 text-xs border-b"
      :class="appStore.isConnecting
        ? 'bg-amber-50 dark:bg-amber-950/30 text-amber-700 dark:text-amber-400 border-amber-200 dark:border-amber-800/40'
        : 'bg-red-50 dark:bg-red-950/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800/40'">
      <Loader v-if="appStore.isConnecting" :size="12" class="animate-spin flex-shrink-0" />
      <WifiOff v-else :size="12" class="flex-shrink-0" />
      <span v-if="appStore.isConnecting">Connecting to server…</span>
      <span v-else>Connection lost — retrying automatically</span>
    </div>

    <!-- Search bar row -->
    <div class="flex-shrink-0 px-4 pt-4 pb-3 bg-slate-100 dark:bg-slate-950 z-10">
      <div class="flex items-center gap-2">
        <!-- Search form -->
        <form class="flex-1 flex gap-2" @submit="handleSearch">
          <div class="relative flex-1">
            <Search
              :size="16"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
            <input
              v-model="query"
              type="search"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="none"
              spellcheck="false"
              :placeholder="searchPlaceholder"
              :disabled="!appStore.isConnected"
              @focus="onSearchFocus"
              @blur="onSearchBlur"
              @keydown="onSearchKeydown"
              class="w-full pl-9 pr-4 py-2.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-50 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-brand-400 focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed transition" />
            <!-- Autocomplete dropdown -->
            <Transition name="suggest">
              <ul
                v-if="showSuggestions && suggestions.length > 0"
                class="absolute left-0 right-0 top-full mt-1 z-50 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl shadow-lg overflow-hidden max-h-72 overflow-y-auto">
                <li
                  v-for="(item, idx) in suggestions"
                  :key="item.timestamp"
                  @mousedown.prevent="selectSuggestion(item)"
                  @mouseenter="selectedSuggestion = idx"
                  class="flex items-center gap-2.5 px-3 py-2 cursor-pointer transition-colors"
                  :class="idx === selectedSuggestion
                    ? 'bg-brand-50 dark:bg-brand-900/20'
                    : 'hover:bg-slate-50 dark:hover:bg-slate-700/50'">
                  <Search :size="13" class="flex-shrink-0 text-slate-400" />
                  <span class="truncate text-sm text-slate-700 dark:text-slate-200">
                    {{ item.query }}
                  </span>
                  <span class="flex-shrink-0 text-[11px] tabular-nums text-slate-400 dark:text-slate-500">
                    {{ formatAge(item.timestamp) }}
                  </span>
                </li>
              </ul>
            </Transition>
          </div>
          <button
            type="submit"
            :disabled="!validInput"
            class="flex-shrink-0 px-4 py-2.5 rounded-xl text-sm font-semibold transition disabled:opacity-40 disabled:cursor-not-allowed"
            :class="
              validInput
                ? 'bg-brand-400 hover:bg-brand-500 text-white'
                : 'bg-slate-200 dark:bg-slate-700 text-slate-500 dark:text-slate-400'
            ">
            {{ errorMode ? "Get" : "Search" }}
          </button>
        </form>
      </div>

      <!-- Result stats bar / connection indicator -->
      <div class="mt-1.5 flex items-center gap-3 text-xs text-slate-400 dark:text-slate-500">
        <!-- Non-blocking searching indicator -->
        <template v-if="isSearching && !appStore.activeItem?.results">
          <Loader :size="11" class="animate-spin text-brand-400 flex-shrink-0" />
          <span class="text-brand-400">Searching for &ldquo;{{ appStore.activeItem?.query }}&rdquo;&hellip;</span>
        </template>
        <template v-else-if="appStore.activeItem?.results">
          <span class="tabular-nums">{{ resultCount.toLocaleString() }} results</span>
          <span
            v-if="onlineCount > 0"
            class="flex items-center gap-1"
            :class="isFresh ? 'text-green-600 dark:text-green-500' : 'text-amber-600 dark:text-amber-500'"
            :title="isFresh ? 'Server list is current' : 'Server list may be stale — refresh to update'">
            <Wifi :size="11" />
            {{ onlineCount }} online
            <span v-if="!isFresh" class="text-[10px] opacity-70">(stale)</span>
          </span>
          <span v-if="hasErrors" class="text-amber-500">
            · {{ appStore.activeItem.errors?.length }} parse errors
          </span>
          <button
            v-if="rawResults"
            class="ml-1 flex items-center gap-1 rounded border border-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-500 transition hover:border-brand-300 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            title="Download the raw IRC search results text file"
            @click="downloadRawResults">
            <Download :size="11" />
            Raw results
          </button>
          <button
            v-if="appStore.isConnected && !isSearching"
            class="ml-1 flex items-center gap-1 rounded border border-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-500 transition hover:border-brand-300 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            title="Re-run this search against IRC"
            @click="retrySearch">
            <RefreshCw :size="11" />
            Refresh
          </button>
        </template>
        <!-- Connected indicator (always visible when connected, right-aligned) -->
        <span
          v-if="appStore.isConnected && appStore.username"
          class="ml-auto flex items-center gap-1 text-slate-400 dark:text-slate-500">
          <span class="w-1.5 h-1.5 rounded-full bg-green-500 inline-block" />
          {{ appStore.username }}
        </span>
      </div>

      <!-- Errors toggle -->
      <div v-if="hasErrors" class="mt-2">
        <button
          class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full transition"
          :class="
            showErrors
              ? 'bg-amber-500 text-white'
              : 'bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400 hover:bg-amber-100 dark:hover:bg-amber-900/50'
          "
          @click="showErrors = !showErrors">
          <span>⚠</span>
          {{ appStore.activeItem?.errors?.length }} parsing
          {{ appStore.activeItem?.errors?.length === 1 ? "error" : "errors" }}
        </button>
      </div>
    </div>

    <!-- Content area -->
    <div class="flex-1 overflow-hidden">
      <!-- Not yet connected: show connecting/failed state instead of search prompt -->
      <div
        v-if="!appStore.activeItem && !appStore.isConnected"
        class="h-full flex items-center justify-center">
        <div class="flex flex-col items-center gap-3 text-center">
          <div
            class="w-12 h-12 rounded-full flex items-center justify-center"
            :class="appStore.isConnecting
              ? 'bg-amber-100 dark:bg-amber-900/30'
              : 'bg-red-100 dark:bg-red-900/30'">
            <Loader v-if="appStore.isConnecting" :size="22" class="animate-spin text-amber-500" />
            <WifiOff v-else :size="22" class="text-red-400" />
          </div>
          <p class="text-sm font-medium text-slate-600 dark:text-slate-300">
            {{ appStore.isConnecting ? 'Connecting to server…' : 'Connection failed' }}
          </p>
          <p class="text-xs text-slate-400">
            {{ appStore.isConnecting ? 'Search will be available once connected' : 'Check that the backend is running on :5228' }}
          </p>
        </div>
      </div>

      <EmptyState v-else-if="!appStore.activeItem" />

      <!-- Timeout state -->
      <div
        v-else-if="isTimedOut"
        class="h-full flex items-center justify-center">
        <div class="flex flex-col items-center gap-3 text-center max-w-sm px-4">
          <div class="w-12 h-12 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
            <WifiOff :size="22" class="text-red-400" />
          </div>
          <p class="text-sm font-medium text-slate-600 dark:text-slate-300">Search timed out</p>
          <p class="text-xs text-slate-400 dark:text-slate-500">
            No response from the IRC server after 60 seconds. The bot may be offline or overloaded.
          </p>
          <button
            v-if="appStore.isConnected"
            class="mt-1 px-4 py-2 rounded-lg text-sm font-semibold bg-brand-400 hover:bg-brand-500 text-white transition"
            @click="retrySearch">
            Search again
          </button>
        </div>
      </div>

      <ErrorTable
        v-else-if="errorMode"
        :errors="appStore.activeItem.errors ?? []"
        @download="
          (cmd: string) => {
            query = cmd;
          }
        " />
      <!-- Searching / queued waiting state -->
      <div
        v-else-if="appStore.activeItem.results === undefined"
        class="h-full flex items-center justify-center">
        <div class="flex flex-col items-center gap-4 text-center max-w-xs px-4">
          <!-- Circular SVG countdown ring (searching) or clock icon (queued) -->
          <div class="relative">
            <svg
              v-if="activeSearchTask"
              class="-rotate-90"
              width="56" height="56" viewBox="0 0 56 56">
              <circle cx="28" cy="28" r="22" fill="none" stroke-width="4"
                class="stroke-slate-200 dark:stroke-slate-700" />
              <circle cx="28" cy="28" r="22" fill="none" stroke-width="4"
                class="stroke-brand-400"
                :stroke-dasharray="138.23"
                :stroke-dashoffset="138.23 * searchProgressPct / 100"
                style="transition: stroke-dashoffset 1s linear" />
            </svg>
            <!-- Centered icon inside the ring -->
            <div class="absolute inset-0 flex items-center justify-center" v-if="activeSearchTask">
              <Search :size="18" class="text-brand-400" />
            </div>
            <!-- Queued: just a clock icon, no ring -->
            <div v-else class="w-14 h-14 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
              <Clock :size="24" class="text-slate-400" />
            </div>
          </div>

          <!-- Label -->
          <div>
            <p class="text-sm font-medium text-slate-700 dark:text-slate-200">
              <template v-if="activeSearchTask">
                Searching for &ldquo;{{ appStore.activeItem.query }}&rdquo;&hellip;
              </template>
              <template v-else>
                &ldquo;{{ appStore.activeItem.query }}&rdquo; is queued
              </template>
            </p>
            <p class="text-xs text-slate-400 dark:text-slate-500 mt-1">
              <template v-if="activeSearchTask">
                {{ secondsRemaining }}s remaining before timeout
              </template>
              <template v-else-if="queuePosition > 0">
                Position {{ queuePosition }} in queue
              </template>
            </p>
          </div>

          <!-- Progress bar -->
          <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
            <div
              v-if="activeSearchTask"
              class="h-full bg-brand-400 rounded-full"
              style="transition: width 1s linear"
              :style="{ width: `${100 - searchProgressPct}%` }" />
            <div
              v-else
              class="h-full w-8 bg-slate-400 dark:bg-slate-500 rounded-full animate-pulse" />
          </div>
        </div>
      </div>

      <!-- Cached results banner + table.
           Must be a real element (not <template>) so the flex column
           correctly constrains BookTable to remaining height. -->
      <div
        v-if="appStore.activeItem && appStore.activeItem.results !== undefined"
        class="flex flex-col h-full overflow-hidden">
        <!-- Age-aware cached results banner -->
        <div
          v-if="isShowingCachedResults && appStore.isConnected"
          class="flex-shrink-0 flex items-center justify-between gap-3 px-4 py-2 border-b text-xs"
          :class="cachedResultsAgeMinutes !== null && cachedResultsAgeMinutes > 30
            ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-400'
            : 'bg-slate-50 dark:bg-slate-800/40 border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400'">
          <span v-if="cachedResultsAgeMinutes !== null && cachedResultsAgeMinutes > 30">
            Results from {{ formatCacheAge(cachedResultsAgeMinutes) }} — may be outdated.
          </span>
          <span v-else-if="cachedResultsAgeMinutes !== null">
            Results from {{ formatCacheAge(cachedResultsAgeMinutes) }}.
          </span>
          <span v-else>
            Showing saved results from a previous search.
          </span>
          <button
            class="flex-shrink-0 flex items-center gap-1 font-medium px-2.5 py-1 rounded border transition-colors"
            :class="cachedResultsAgeMinutes !== null && cachedResultsAgeMinutes > 30
              ? 'border-amber-300 dark:border-amber-700 hover:bg-amber-100 dark:hover:bg-amber-900/40'
              : 'border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-700'"
            @click="retrySearch">
            <RefreshCw :size="11" />
            {{ cachedResultsAgeMinutes !== null && cachedResultsAgeMinutes > 30 ? 'Refresh' : 'Search again' }}
          </button>
        </div>
        <!-- flex-1 min-h-0: fill remaining height after the banner without overflowing -->
        <BookCards v-if="isMobile" class="flex-1 overflow-auto" :books="appStore.activeItem.results" />
        <BookTable v-else class="flex-1 min-h-0" :books="appStore.activeItem.results" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.suggest-enter-active,
.suggest-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.suggest-enter-from,
.suggest-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
