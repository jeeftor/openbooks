<script setup lang="ts">
import { computed } from "vue";
import { storeToRefs } from "pinia";
import { ArrowDownAZ, BookOpen, Clock3, DownloadCloud, Library, RefreshCw, Trash2 } from "lucide-vue-next";
import { toast } from "vue-sonner";
import { useAppStore } from "../../stores/app";
import { useBooks, deleteBook } from "../../composables/useApi";
import { getApiUrl } from "../../composables/useWebSocket";
import type { Book } from "../../types/messages";
import { groupLibraryBooks, type BookGroup } from "../../utils/library";
import EmptyPanel from "../shared/EmptyPanel.vue";
import PanelHeader from "../shared/PanelHeader.vue";

const appStore = useAppStore();
const { libraryVersion } = storeToRefs(appStore);
const { books, loading, refresh } = useBooks(libraryVersion);

const groups = computed(() => groupLibraryBooks(books.value, appStore.librarySortMode));

const sortLabel = computed(() =>
  appStore.librarySortMode === "newest" ? "Newest" : "A–Z"
);
const totalLabel = computed(() =>
  `${groups.value.length} book${groups.value.length === 1 ? "" : "s"} · ${books.value.length} file${books.value.length === 1 ? "" : "s"}`
);

function toggleSort() {
  appStore.librarySortMode = appStore.librarySortMode === "newest" ? "alpha" : "newest";
}

function formats(group: BookGroup): string[] {
  return [...new Set(group.files.map((book) => book.format || "file"))].sort();
}

function seriesLabel(group: BookGroup): string {
  return group.seriesIndex ? `${group.series} #${group.seriesIndex}` : group.series;
}

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
    year: "numeric"
  });
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden bg-slate-50 dark:bg-slate-950">
    <!-- Header -->
    <PanelHeader>
      {{ totalLabel }}
      <template #actions>
        <div class="flex items-center gap-1">
          <button
            class="flex items-center gap-1 px-2 py-1 rounded-md text-[10px] font-medium text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
            :title="`Sort library: ${sortLabel}`"
            @click="toggleSort">
            <Clock3 v-if="appStore.librarySortMode === 'newest'" :size="12" />
            <ArrowDownAZ v-else :size="12" />
            {{ sortLabel }}
          </button>
          <button
            class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
            :class="{ 'animate-spin': loading }"
            title="Refresh"
            @click="refresh()">
            <RefreshCw :size="13" />
          </button>
        </div>
      </template>
    </PanelHeader>

    <!-- Loading -->
    <div
      v-if="loading && !books.length"
      class="flex-1 flex items-center justify-center">
      <RefreshCw :size="20" class="animate-spin text-slate-300 dark:text-slate-600" />
    </div>

    <!-- Empty -->
    <EmptyPanel v-else-if="!books.length" :icon="Library">
      No downloaded books yet.
    </EmptyPanel>

    <!-- Grouped list -->
    <div v-else class="flex-1 overflow-y-auto p-3 sm:p-4">
      <div class="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-3">
        <article
          v-for="group in groups"
          :key="group.key"
          class="min-w-0 overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm">
          <div class="flex gap-3 p-3.5">
            <div class="w-14 h-20 flex-shrink-0 rounded-lg bg-gradient-to-br from-brand-100 to-slate-200 dark:from-brand-950 dark:to-slate-800 flex items-center justify-center ring-1 ring-slate-200 dark:ring-slate-700">
              <BookOpen :size="24" class="text-brand-500 dark:text-brand-400" />
            </div>
            <div class="min-w-0 flex-1">
              <h3 class="text-sm font-semibold leading-snug text-slate-800 dark:text-slate-100 [overflow-wrap:anywhere]">
                {{ group.title }}
              </h3>
              <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400 truncate" :title="group.author">
                {{ group.author }}
              </p>
              <p v-if="group.series" class="mt-1 text-[10px] text-brand-500 dark:text-brand-400 truncate" :title="seriesLabel(group)">
                {{ seriesLabel(group) }}
              </p>
              <div class="mt-2 flex flex-wrap items-center gap-1">
                <span
                  v-for="format in formats(group)"
                  :key="format"
                  class="rounded-full bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                  {{ format }}
                </span>
                <span class="text-[9px] text-slate-400 dark:text-slate-500">
                  {{ group.files.length }} {{ group.files.length === 1 ? "file" : "files" }}
                </span>
              </div>
            </div>
          </div>

          <ul class="border-t border-slate-100 dark:border-slate-800 divide-y divide-slate-100 dark:divide-slate-800">
            <li
              v-for="book in group.files"
              :key="book.path"
              class="flex items-center gap-2 px-3 py-2">
              <div class="min-w-0 flex-1">
                <p class="truncate text-[11px] font-medium text-slate-600 dark:text-slate-300" :title="book.path">
                  {{ book.name }}
                </p>
                <p class="text-[9px] text-slate-400 dark:text-slate-500">
                  {{ formatDate(book.time) }} · {{ (book.format || "file").toUpperCase() }}
                </p>
              </div>
              <button
                class="flex-shrink-0 p-1.5 rounded-md text-slate-400 hover:text-brand-500 hover:bg-brand-50 dark:hover:bg-brand-950/40 transition-colors"
                title="Download"
                @click="downloadBook(book.downloadLink)">
                <DownloadCloud :size="13" />
              </button>
              <button
                class="flex-shrink-0 p-1.5 rounded-md text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-950/30 transition-colors"
                title="Delete"
                @click="handleDelete(book.path, book.name)">
                <Trash2 :size="13" />
              </button>
            </li>
          </ul>
        </article>
      </div>
    </div>
  </div>
</template>
