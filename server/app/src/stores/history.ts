import { defineStore } from "pinia";
import { useLocalStorage } from "@vueuse/core";
import type { HistoryItem } from "../types/messages";
import { useAppStore } from "./app";

export const useHistoryStore = defineStore("history", () => {
  const items = useLocalStorage<HistoryItem[]>("ob-history", []);

  function addItem(item: HistoryItem) {
    items.value = [item, ...items.value].slice(0, 16);
  }

  function updateItem(updated: HistoryItem) {
    const idx = items.value.findIndex((x) => x.timestamp === updated.timestamp);
    if (idx !== -1) {
      const copy = [...items.value];
      copy[idx] = updated;
      items.value = copy;
    }
  }

  function deleteItem(timestamp?: number) {
    const appStore = useAppStore();
    if (timestamp === undefined) {
      appStore.setActiveItem(null);
      const first = items.value[0]?.timestamp;
      if (first !== undefined) {
        items.value = items.value.filter((x) => x.timestamp !== first);
      }
      return;
    }
    if (appStore.activeItem?.timestamp === timestamp) {
      appStore.setActiveItem(null);
    }
    items.value = items.value.filter((x) => x.timestamp !== timestamp);
  }

  function restoreItem(item: HistoryItem) {
    const appStore = useAppStore();
    appStore.setActiveItem(item);
  }

  return { items, addItem, updateItem, deleteItem, restoreItem };
});
