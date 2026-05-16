import { defineStore } from "pinia";
import { ref } from "vue";
import { useLocalStorage } from "@vueuse/core";
import type { BookDetail, HistoryItem, ParseError } from "../types/messages";
import { useAppStore } from "./app";

// Strip heavy result/error arrays before writing to localStorage.
// Only query, timestamp, and timedOut are needed to rebuild the history list.
function slim(item: HistoryItem): HistoryItem {
  return { query: item.query, timestamp: item.timestamp, timedOut: item.timedOut };
}

interface CachedResults {
  results: BookDetail[];
  errors: ParseError[];
}

export const useHistoryStore = defineStore("history", () => {
  const items = useLocalStorage<HistoryItem[]>("ob-history", []);

  // In-memory cache of results keyed by timestamp. Never persisted.
  const resultsCache = ref<Map<number, CachedResults>>(new Map());

  function addItem(item: HistoryItem) {
    items.value = [slim(item), ...items.value].slice(0, 16);
  }

  function updateItem(updated: HistoryItem) {
    const idx = items.value.findIndex((x) => x.timestamp === updated.timestamp);
    if (idx !== -1) {
      const copy = [...items.value];
      copy[idx] = slim(updated);
      items.value = copy;
    }
    // Keep in-memory cache up to date when results arrive.
    if (updated.results !== undefined) {
      resultsCache.value = new Map(resultsCache.value).set(updated.timestamp, {
        results: updated.results,
        errors: updated.errors ?? [],
      });
    }
  }

  function cacheResults(timestamp: number, results: BookDetail[], errors: ParseError[]) {
    resultsCache.value = new Map(resultsCache.value).set(timestamp, { results, errors });
  }

  function clearCachedResults(timestamp: number) {
    const next = new Map(resultsCache.value);
    next.delete(timestamp);
    resultsCache.value = next;
  }

  function getCachedResults(timestamp: number): CachedResults | undefined {
    return resultsCache.value.get(timestamp);
  }

  function deleteItem(timestamp?: number) {
    const appStore = useAppStore();
    if (timestamp === undefined) {
      appStore.setActiveItem(null);
      const first = items.value[0]?.timestamp;
      if (first !== undefined) {
        items.value = items.value.filter((x) => x.timestamp !== first);
        clearCachedResults(first);
      }
      return;
    }
    if (appStore.activeItem?.timestamp === timestamp) {
      appStore.setActiveItem(null);
    }
    items.value = items.value.filter((x) => x.timestamp !== timestamp);
    clearCachedResults(timestamp);
  }

  function restoreItem(item: HistoryItem) {
    const appStore = useAppStore();
    const cached = resultsCache.value.get(item.timestamp);
    if (cached) {
      appStore.setActiveItem({ ...item, results: cached.results, errors: cached.errors });
    } else {
      appStore.setActiveItem(item);
    }
  }

  return {
    items,
    resultsCache,
    addItem,
    updateItem,
    cacheResults,
    clearCachedResults,
    getCachedResults,
    restoreItem,
    deleteItem,
  };
});
