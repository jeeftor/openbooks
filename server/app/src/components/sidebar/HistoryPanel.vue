<script setup lang="ts">
import { Clock, Trash2, SearchCode } from "lucide-vue-next";
import { useHistoryStore } from "../../stores/history";
import { useAppStore } from "../../stores/app";
import type { HistoryItem } from "../../types/messages";

const historyStore = useHistoryStore();
const appStore = useAppStore();

function formatTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString("en-US", {
    timeStyle: "short"
  });
}

function formatDate(timestamp: number) {
  return new Date(timestamp).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric"
  });
}

function isToday(timestamp: number) {
  return new Date(timestamp).toDateString() === new Date().toDateString();
}

function select(item: HistoryItem) {
  // If the item has cached results and didn't time out, just show them.
  // Otherwise re-issue the search.
  if (item.results !== undefined && !item.timedOut) {
    historyStore.restoreItem(item);
  } else {
    appStore.pendingQuery = item.query;
  }
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <div
      class="flex-shrink-0 flex items-center justify-between px-3 py-2 border-b border-slate-100 dark:border-slate-800">
      <span class="text-xs font-medium text-slate-500 dark:text-slate-400">
        {{ historyStore.items.length }} searches
      </span>
      <button
        v-if="historyStore.items.length"
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-red-400 transition-colors"
        title="Clear all history"
        @click="historyStore.items = []">
        <Trash2 :size="13" />
      </button>
    </div>

    <!-- Empty -->
    <div
      v-if="!historyStore.items.length"
      class="flex-1 flex flex-col items-center justify-center gap-2 text-center px-4">
      <Clock :size="28" class="text-slate-300 dark:text-slate-600" />
      <p class="text-xs text-slate-400 dark:text-slate-500">
        No search history yet.
      </p>
    </div>

    <!-- List -->
    <ul
      v-else
      class="flex-1 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800/60">
      <li
        v-for="item in historyStore.items"
        :key="item.timestamp"
        class="group flex items-center gap-2 px-3 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-800/50 cursor-pointer transition-colors"
        @click="select(item)">
        <SearchCode
          :size="14"
          class="flex-shrink-0 text-slate-300 dark:text-slate-600 group-hover:text-brand-400 transition-colors" />
        <div class="flex-1 min-w-0">
          <p
            class="text-sm truncate font-medium transition-colors"
            :class="
              appStore.activeItem?.timestamp === item.timestamp
                ? 'text-brand-500 dark:text-brand-300'
                : 'text-slate-700 dark:text-slate-200'
            ">
            {{ item.query }}
          </p>
          <p class="text-[11px] text-slate-400 dark:text-slate-500">
            {{
              isToday(item.timestamp)
                ? formatTime(item.timestamp)
                : formatDate(item.timestamp)
            }}
            <span v-if="item.timedOut" class="ml-1 text-red-400">· timed out</span>
            <span v-else-if="item.results != null" class="ml-1"
              >· {{ item.results.length }} results</span
            >
          </p>
        </div>
        <button
          class="flex-shrink-0 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-red-400 transition-all"
          title="Delete"
          @click.stop="historyStore.deleteItem(item.timestamp)">
          <Trash2 :size="12" />
        </button>
      </li>
    </ul>
  </div>
</template>
