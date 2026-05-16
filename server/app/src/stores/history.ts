import { defineStore } from "pinia";
import { ref } from "vue";
import type { BookDetail, HistoryItem, ParseError } from "../types/messages";
import { MessageType } from "../types/messages";
import { sendMessage } from "../composables/useWebSocket";
import { useAppStore } from "./app";

interface CachedResults {
  results: BookDetail[];
  errors: ParseError[];
}

export const useHistoryStore = defineStore("history", () => {
  // Items are populated from the server via HISTORY_LIST on connect.
  const items = ref<HistoryItem[]>([]);

  // In-memory cache of results keyed by timestamp. Never persisted.
  const resultsCache = ref<Map<number, CachedResults>>(new Map());

  // Populate items from server-sent HISTORY_LIST payload.
  function loadFromServer(entries: Array<{ query: string; timestamp: number; timedOut?: boolean }>) {
    items.value = entries.map((e) => ({ query: e.query, timestamp: e.timestamp, timedOut: e.timedOut }));
  }

  function addItem(item: HistoryItem) {
    // Prepend, de-duplicate by query, cap at 50 (mirrors server logic).
    const deduped = items.value.filter((x) => x.query !== item.query);
    items.value = [{ query: item.query, timestamp: item.timestamp, timedOut: item.timedOut }, ...deduped].slice(0, 50);
  }

  function updateItem(updated: HistoryItem) {
    const idx = items.value.findIndex((x: HistoryItem) => x.timestamp === updated.timestamp);
    if (idx !== -1) {
      const copy = [...items.value];
      copy[idx] = { query: updated.query, timestamp: updated.timestamp, timedOut: updated.timedOut };
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
      // Legacy: delete the oldest rate-limited entry (first in list).
      appStore.setActiveItem(null);
      const first = items.value[0]?.timestamp;
      if (first !== undefined) {
        sendMessage({ type: MessageType.HISTORY_DELETE, payload: { timestamp: first } });
        items.value = items.value.filter((x: HistoryItem) => x.timestamp !== first);
        clearCachedResults(first);
      }
      return;
    }
    if (appStore.activeItem?.timestamp === timestamp) {
      appStore.setActiveItem(null);
    }
    sendMessage({ type: MessageType.HISTORY_DELETE, payload: { timestamp } });
    items.value = items.value.filter((x: HistoryItem) => x.timestamp !== timestamp);
    clearCachedResults(timestamp);
  }

  function clearAll() {
    const appStore = useAppStore();
    appStore.setActiveItem(null);
    sendMessage({ type: MessageType.HISTORY_CLEAR });
    items.value = [];
    resultsCache.value = new Map();
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
    loadFromServer,
    addItem,
    updateItem,
    cacheResults,
    clearCachedResults,
    getCachedResults,
    restoreItem,
    deleteItem,
    clearAll,
  };
});
