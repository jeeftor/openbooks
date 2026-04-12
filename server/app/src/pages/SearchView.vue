<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { Search, PanelLeftOpen, Loader, Wifi, WifiOff } from "lucide-vue-next";
import { useAppStore } from "../stores/app";
import { useHistoryStore } from "../stores/history";
import { sendMessage } from "../composables/useWebSocket";
import { MessageType } from "../types/messages";
import { useServers } from "../composables/useApi";
import BookTable from "../components/books/BookTable.vue";
import BookCards from "../components/books/BookCards.vue";
import ErrorTable from "../components/errors/ErrorTable.vue";
import EmptyState from "../components/search/EmptyState.vue";

const appStore = useAppStore();
const historyStore = useHistoryStore();
const servers = useServers();
const isMobile = useMediaQuery("(max-width: 767px)");

const query = ref("");
const showErrors = ref(false);
let searchTimeout: ReturnType<typeof setTimeout> | null = null;
let searchProgressInterval: ReturnType<typeof setInterval> | null = null;
const searchStartTime = ref<number | null>(null);
const searchProgress = ref(0); // 0-100

const hasErrors = computed(
  () => (appStore.activeItem?.errors?.length ?? 0) > 0
);
const resultCount = computed(() => appStore.activeItem?.results?.length ?? 0);
const onlineCount = computed(() => {
  const results = appStore.activeItem?.results;
  if (!results || !servers.value.length) return 0;
  return new Set(results.filter(b => servers.value.includes(b.server)).map(b => b.server)).size;
});
const errorMode = computed(() => showErrors.value && !!appStore.activeItem);
const validInput = computed(() => {
  if (!appStore.isConnected) return false;
  return errorMode.value ? query.value.startsWith("!") : query.value.trim() !== "";
});

watch(
  () => appStore.activeItem?.timestamp,
  () => {
    showErrors.value = false;
  }
);

// Watch for results arriving to clear the timeout
watch(
  () => appStore.activeItem?.results,
  (results) => {
    if (results !== undefined && searchTimeout) {
      clearTimeout(searchTimeout);
      searchTimeout = null;
    }
    if (results !== undefined && searchProgressInterval) {
      clearInterval(searchProgressInterval);
      searchProgressInterval = null;
      searchStartTime.value = null;
      searchProgress.value = 0;
    }
  }
);

onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout);
  if (searchProgressInterval) clearInterval(searchProgressInterval);
});

function handleSearch(e: Event) {
  e.preventDefault();
  if (!validInput.value) return;

  if (errorMode.value) {
    sendMessage({ type: MessageType.DOWNLOAD, payload: { book: query.value } });
  } else {
    const q = query.value.trim();
    const timestamp = Date.now();
    appStore.setActiveItem({ query: q, timestamp });
    historyStore.addItem({ query: q, timestamp });
    sendMessage({ type: MessageType.SEARCH, payload: { query: q } });

    // Start countdown progress bar
    searchStartTime.value = Date.now();
    searchProgress.value = 0;
    if (searchProgressInterval) clearInterval(searchProgressInterval);
    searchProgressInterval = setInterval(() => {
      if (searchStartTime.value) {
        const elapsed = Date.now() - searchStartTime.value;
        searchProgress.value = Math.min((elapsed / 60000) * 100, 100);
      }
    }, 100);

    // Set a 60s timeout — if no results arrive, mark as failed
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      const active = appStore.activeItem;
      if (active && active.results === undefined) {
        const timedOut = {
          ...active,
          results: [],
          errors: [{
            error: "Search timeout",
            line: "No response from IRC server after 60 seconds. The bot may be offline or overloaded. Try again later."
          }]
        };
        appStore.setActiveItem(timedOut);
        historyStore.updateItem(timedOut);
      }
      searchTimeout = null;
      if (searchProgressInterval) {
        clearInterval(searchProgressInterval);
        searchProgressInterval = null;
      }
      searchStartTime.value = null;
      searchProgress.value = 0;
    }, 60000);
  }
  query.value = "";
}
</script>

<template>
  <div
    class="flex-1 flex flex-col overflow-hidden"
    :class="isMobile ? 'pb-14' : ''">
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
    <div class="flex-shrink-0 px-4 pt-4 pb-3">
      <div class="flex items-center gap-2">
        <!-- Sidebar toggle (desktop only, when sidebar is closed) -->
        <button
          v-if="!isMobile && !appStore.isSidebarOpen"
          class="flex-shrink-0 p-2 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
          title="Open sidebar"
          @click="appStore.toggleSidebar()">
          <PanelLeftOpen :size="20" />
        </button>

        <!-- Search form -->
        <form class="flex-1 flex gap-2" @submit="handleSearch">
          <div class="relative flex-1">
            <Search
              :size="16"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
            <input
              v-model="query"
              type="search"
              :placeholder="
                errorMode
                  ? 'Enter download command (starts with !)'
                  : 'Search for a book…'
              "
              :disabled="!appStore.isConnected || (appStore.activeItem !== null && !appStore.activeItem.results)"
              class="w-full pl-9 pr-4 py-2.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-50 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-brand-400 focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed transition" />
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
        <template v-if="appStore.activeItem?.results">
          <span class="tabular-nums">{{ resultCount.toLocaleString() }} results</span>
          <span
            v-if="onlineCount > 0"
            class="flex items-center gap-1 text-green-600 dark:text-green-500">
            <Wifi :size="11" />
            {{ onlineCount }} online
          </span>
          <span v-if="hasErrors" class="text-amber-500">
            · {{ appStore.activeItem.errors?.length }} parse errors
          </span>
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

      <!-- Loading while waiting for search results -->
      <div
        v-else-if="
          appStore.activeItem && appStore.activeItem.results === undefined
        "
        class="h-full flex items-center justify-center">
        <div class="flex flex-col items-center gap-4 max-w-sm w-full px-4">
          <Loader :size="28" class="animate-spin text-brand-400" />
          <p class="text-sm text-slate-500 dark:text-slate-400 text-center">
            Searching for &ldquo;{{ appStore.activeItem.query }}&rdquo;&hellip;
          </p>
          <!-- Progress bar -->
          <div class="w-full">
            <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
              <div
                class="h-full bg-brand-400 transition-all duration-100 ease-linear"
                :style="{ width: searchProgress + '%' }" />
            </div>
            <p class="text-xs text-slate-400 dark:text-slate-500 text-center mt-1.5">
              {{ Math.ceil((60000 - (searchProgress / 100 * 60000)) / 1000) }}s remaining
            </p>
          </div>
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
      <BookCards
        v-else-if="isMobile"
        :books="appStore.activeItem.results ?? []" />
      <BookTable v-else :books="appStore.activeItem.results ?? []" />
    </div>
  </div>
</template>
