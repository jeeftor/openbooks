<script setup lang="ts">
import { ref } from "vue";
import { Trash2, BookMarked, ChevronRight, X } from "lucide-vue-next";
import { useAppStore } from "../stores/app";
import { MessageType, type StagedBookSummary } from "../types/messages";
import { sendMessage } from "../composables/useWebSocket";

const appStore = useAppStore();

const confirmDeleteId = ref<string | null>(null);

function processOne(book: StagedBookSummary) {
  sendMessage({ type: MessageType.PROCESS_ONE_STAGED, payload: { stagedId: book.id } });
  appStore.setStagedBooksList(null);
}

function requestDelete(id: string) {
  confirmDeleteId.value = id;
}

function confirmDelete(book: StagedBookSummary) {
  sendMessage({ type: MessageType.DELETE_STAGED, payload: { stagedId: book.id } });
  // Optimistically remove from list
  if (appStore.stagedBooksList) {
    appStore.setStagedBooksList(appStore.stagedBooksList.filter((b) => b.id !== book.id));
  }
  confirmDeleteId.value = null;
}

function close() {
  appStore.setStagedBooksList(null);
}

function displayTitle(book: StagedBookSummary): string {
  return book.metadata?.title || book.ircFilename;
}

function displayAuthor(book: StagedBookSummary): string {
  return book.metadata?.author || "";
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}
</script>

<template>
  <Transition name="modal">
    <div
      v-if="appStore.stagedBooksList !== null"
      class="fixed inset-0 z-50 flex items-end justify-center p-4 sm:items-center bg-black/60 backdrop-blur-sm"
      @click.self="close"
    >
      <div class="w-full max-w-lg bg-white dark:bg-slate-900 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]">
        <!-- Header -->
        <div class="flex items-center justify-between px-5 py-4 border-b border-slate-200 dark:border-slate-700">
          <div class="flex items-center gap-2">
            <BookMarked :size="18" class="text-amber-500" />
            <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-50">
              Staged Books
              <span class="ml-1 text-xs font-normal text-slate-400 dark:text-slate-500">
                {{ appStore.stagedBooksList?.length ?? 0 }} waiting
              </span>
            </h2>
          </div>
          <button
            @click="close"
            class="p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X :size="16" />
          </button>
        </div>

        <!-- Empty state -->
        <div
          v-if="!appStore.stagedBooksList?.length"
          class="flex-1 flex flex-col items-center justify-center py-12 text-center px-6"
        >
          <BookMarked :size="32" class="text-slate-300 dark:text-slate-600 mb-2" />
          <p class="text-sm text-slate-400 dark:text-slate-500">No staged books remaining.</p>
        </div>

        <!-- List -->
        <ul v-else class="flex-1 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800">
          <li
            v-for="book in appStore.stagedBooksList"
            :key="book.id"
            class="group flex items-center gap-3 px-4 py-3"
          >
            <!-- Cover thumbnail -->
            <img
              v-if="book.coverBase64"
              :src="`data:${book.coverMime};base64,${book.coverBase64}`"
              alt=""
              class="flex-shrink-0 w-9 h-13 object-cover rounded shadow-sm ring-1 ring-slate-200 dark:ring-slate-700"
            />
            <div
              v-else
              class="flex-shrink-0 w-9 h-13 rounded bg-slate-100 dark:bg-slate-800 flex items-center justify-center"
            >
              <BookMarked :size="14" class="text-slate-300 dark:text-slate-600" />
            </div>

            <!-- Info -->
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-slate-900 dark:text-slate-100 truncate">
                {{ displayTitle(book) }}
              </p>
              <p v-if="displayAuthor(book)" class="text-xs text-slate-500 dark:text-slate-400 truncate">
                {{ displayAuthor(book) }}
              </p>
              <p class="text-[11px] text-slate-400 dark:text-slate-500 mt-0.5 font-mono truncate">
                {{ book.ircFilename }}
              </p>
            </div>

            <!-- Time -->
            <span class="flex-shrink-0 text-[11px] text-slate-400 dark:text-slate-500 hidden sm:block">
              {{ formatDate(book.stagedAt) }}
            </span>

            <!-- Inline delete confirmation -->
            <template v-if="confirmDeleteId === book.id">
              <span class="text-xs text-red-500 dark:text-red-400 flex-shrink-0">Delete?</span>
              <button
                @click="confirmDelete(book)"
                class="flex-shrink-0 px-2 py-1 text-xs font-medium rounded bg-red-600 hover:bg-red-700 text-white transition-colors"
              >Yes</button>
              <button
                @click="confirmDeleteId = null"
                class="flex-shrink-0 px-2 py-1 text-xs rounded border border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
              >No</button>
            </template>

            <!-- Normal actions -->
            <template v-else>
              <button
                @click.stop="requestDelete(book.id)"
                class="flex-shrink-0 p-1.5 rounded text-slate-400 dark:text-slate-500 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                title="Delete"
              >
                <Trash2 :size="14" />
              </button>
              <button
                @click="processOne(book)"
                class="flex-shrink-0 flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors"
              >
                Save
                <ChevronRight :size="12" />
              </button>
            </template>
          </li>
        </ul>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
