<script setup lang="ts">
import { storeToRefs } from "pinia";
import { Library, Trash2, DownloadCloud, RefreshCw } from "lucide-vue-next";
import { toast } from "vue-sonner";
import { useAppStore } from "../../stores/app";
import { useBooks, deleteBook } from "../../composables/useApi";
import { getApiUrl } from "../../composables/useWebSocket";

const appStore = useAppStore();
const { libraryVersion } = storeToRefs(appStore);
const { books, loading, refresh } = useBooks(libraryVersion);

function downloadBook(link: string) {
  const a = document.createElement("a");
  a.href = getApiUrl("/" + link);
  a.download = "";
  a.target = "_blank";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

async function handleDelete(path: string, name: string) {
  const ok = await deleteBook(path);
  if (ok) {
    toast.success(`Deleted "${name}"`);
    await refresh();
  } else {
    toast.error(`Failed to delete "${name}"`);
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "2-digit"
  });
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <div
      class="flex-shrink-0 flex items-center justify-between px-3 py-2 border-b border-slate-100 dark:border-slate-800">
      <span class="text-xs font-medium text-slate-500 dark:text-slate-400">
        {{ books.length }} file{{ books.length !== 1 ? "s" : "" }}
      </span>
      <button
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
        :class="{ 'animate-spin': loading }"
        title="Refresh"
        @click="refresh()">
        <RefreshCw :size="13" />
      </button>
    </div>

    <!-- Loading -->
    <div
      v-if="loading && !books.length"
      class="flex-1 flex items-center justify-center">
      <RefreshCw :size="20" class="animate-spin text-slate-300 dark:text-slate-600" />
    </div>

    <!-- Empty -->
    <div
      v-else-if="!books.length"
      class="flex-1 flex flex-col items-center justify-center gap-2 text-center px-4">
      <Library :size="28" class="text-slate-300 dark:text-slate-600" />
      <p class="text-xs text-slate-400 dark:text-slate-500">
        No downloaded books yet.<br />
        <span class="text-[11px]"
          >Downloads appear here if persistence is enabled.</span
        >
      </p>
    </div>

    <!-- List -->
    <ul
      v-else
      class="flex-1 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800/60">
      <li
        v-for="book in books"
        :key="book.path"
        class="group flex items-center gap-2 px-3 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
        <Library
          :size="13"
          class="flex-shrink-0 text-slate-300 dark:text-slate-600" />
        <div class="flex-1 min-w-0">
          <p
            class="text-xs font-medium text-slate-700 dark:text-slate-200 truncate">
            {{ book.name }}
          </p>
          <p class="text-[10px] text-slate-400 dark:text-slate-500">
            {{ formatDate(book.time) }}
          </p>
        </div>
        <div
          class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-brand-500 transition-colors"
            title="Download"
            @click="downloadBook(book.downloadLink)">
            <DownloadCloud :size="13" />
          </button>
          <button
            class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-red-400 transition-colors"
            title="Delete"
            @click="handleDelete(book.path, book.name)">
            <Trash2 :size="13" />
          </button>
        </div>
      </li>
    </ul>
  </div>
</template>
