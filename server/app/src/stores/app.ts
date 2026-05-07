import { defineStore } from "pinia";
import { ref } from "vue";
import { useLocalStorage } from "@vueuse/core";
import type { DownloadWaitingResponse, HistoryItem, RenamePromptResponse } from "../types/messages";

export const useAppStore = defineStore("app", () => {
  const isConnected = ref(false);
  const isConnecting = ref(true); // true until first successful connect or max retries
  const isSidebarOpen = useLocalStorage("ob-sidebar-open", true);
  const username = ref<string | undefined>(undefined);
  const inFlightDownloads = ref<string[]>([]);
  const libraryVersion = ref(0);

  // Session-only — not persisted. Results arrive async via WebSocket;
  // using localStorage + computed caused the getter to return null while
  // results were in-flight, so the WS handler would discard them.
  const activeItem = ref<HistoryItem | null>(null);

  // Set to a query string to trigger a new search from any component.
  // SearchView watches this and clears it after issuing the search.
  const pendingQuery = ref<string | null>(null);

  // Set when the server sends a RENAME_PROMPT. RenameModal watches this
  // and clears it after the user confirms or cancels.
  const pendingRename = ref<RenamePromptResponse | null>(null);

  // Set while waiting for an IRC bot to send its DCC offer.
  const waitingDownload = ref<DownloadWaitingResponse | null>(null);

  // Phase of the currently-active (first in queue) download after the DCC offer is accepted.
  const downloadPhase = ref<"transferring" | "cleaning" | null>(null);

  function setConnected(connected: boolean) {
    isConnected.value = connected;
  }

  function setConnecting(connecting: boolean) {
    isConnecting.value = connecting;
  }

  function setUsername(name: string) {
    username.value = name;
  }

  function setActiveItem(item: HistoryItem | null) {
    activeItem.value = item;
  }

  function toggleSidebar() {
    isSidebarOpen.value = !isSidebarOpen.value;
  }

  function addInFlightDownload(book: string) {
    inFlightDownloads.value.push(book);
  }

  function removeInFlightDownload() {
    inFlightDownloads.value.shift();
    downloadPhase.value = null;
    libraryVersion.value++;
  }

  function setDownloadPhase(phase: "transferring" | "cleaning" | null) {
    downloadPhase.value = phase;
  }

  function isDownloading(book: string) {
    return inFlightDownloads.value.includes(book);
  }

  return {
    isConnected,
    isConnecting,
    isSidebarOpen,
    username,
    inFlightDownloads,
    activeItem,
    pendingQuery,
    pendingRename,
    waitingDownload,
    downloadPhase,
    libraryVersion,
    setConnected,
    setConnecting,
    setUsername,
    setActiveItem,
    toggleSidebar,
    addInFlightDownload,
    removeInFlightDownload,
    setDownloadPhase,
    isDownloading
  };
});
