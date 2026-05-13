import { defineStore } from "pinia";
import { ref } from "vue";
import { useLocalStorage } from "@vueuse/core";
import type { DownloadWaitingResponse, HistoryItem, RenamePromptResponse, StagedBookResumeResponse } from "../types/messages";

type LibrarySortMode = "newest" | "alpha";

export const useAppStore = defineStore("app", () => {
  const isConnected = ref(false);
  const isConnecting = ref(true); // true until first successful connect or max retries
  const isSidebarOpen = useLocalStorage("ob-sidebar-open", true);
  const username = ref<string | undefined>(undefined);
  const inFlightDownloads = ref<string[]>([]);
  const libraryVersion = ref(0);
  const rawSearchResults = ref<Record<number, string>>({});
  const librarySortMode = useLocalStorage<LibrarySortMode>("ob-library-sort", "newest");

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

  // Number of books waiting in staging (from STAGED_BOOKS_NOTIFY).
  const stagedBooksCount = ref(0);

  // The staged book currently being processed (from STAGED_BOOK_RESUME).
  const pendingStagedBook = ref<StagedBookResumeResponse | null>(null);

  // Known series names for autocomplete (from SERIES_AUTOCOMPLETE).
  const knownSeries = ref<string[]>([]);

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

  function setRawSearchResult(timestamp: number, raw: string | undefined) {
    if (!raw) return;

    const next = { ...rawSearchResults.value, [timestamp]: raw };
    const timestamps = Object.keys(next)
      .map(Number)
      .sort((a, b) => b - a);
    for (const oldTimestamp of timestamps.slice(5)) {
      delete next[oldTimestamp];
    }
    rawSearchResults.value = next;
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

  function setStagedBooksCount(count: number) {
    stagedBooksCount.value = count;
  }

  function setPendingStagedBook(book: StagedBookResumeResponse | null) {
    pendingStagedBook.value = book;
  }

  function setKnownSeries(series: string[]) {
    knownSeries.value = series;
  }

  function toggleLibrarySortMode() {
    librarySortMode.value = librarySortMode.value === "newest" ? "alpha" : "newest";
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
    rawSearchResults,
    pendingQuery,
    pendingRename,
    waitingDownload,
    downloadPhase,
    libraryVersion,
    librarySortMode,
    stagedBooksCount,
    pendingStagedBook,
    knownSeries,
    setConnected,
    setConnecting,
    setUsername,
    setActiveItem,
    setRawSearchResult,
    toggleSidebar,
    addInFlightDownload,
    removeInFlightDownload,
    setDownloadPhase,
    toggleLibrarySortMode,
    isDownloading,
    setStagedBooksCount,
    setPendingStagedBook,
    setKnownSeries
  };
});
