<script setup lang="ts">
import { computed, ref } from "vue";
import { storeToRefs } from "pinia";
import { Library, Folder, FolderOpen, Trash2, DownloadCloud, RefreshCw } from "lucide-vue-next";
import { toast } from "vue-sonner";
import { useAppStore } from "../../stores/app";
import { useBooks, deleteBook } from "../../composables/useApi";
import { getApiUrl } from "../../composables/useWebSocket";
import type { Book } from "../../types/messages";

const appStore = useAppStore();
const { libraryVersion } = storeToRefs(appStore);
const { books, loading, refresh } = useBooks(libraryVersion);

interface FileGroup {
  dir: string;    // parent path relative to DownloadDir (empty = root)
  label: string;  // display name for the group header
  files: Book[];
}

// Group files by their parent directory. Root-level files (no subdir) are each
// their own group so they render flat without a folder header.
const groups = computed((): FileGroup[] => {
  const map = new Map<string, FileGroup>();

  for (const book of books.value) {
    const parts = book.path.split("/");
    const isNested = parts.length > 1;
    const dir = isNested ? parts.slice(0, -1).join("/") : `__root__:${book.path}`;
    const label = isNested ? parts[parts.length - 2] : "";

    if (!map.has(dir)) {
      map.set(dir, { dir, label, files: [] });
    }
    map.get(dir)!.files.push(book);
  }

  return [...map.values()];
});

const groupCount = computed(() => groups.value.filter(g => g.label !== "").length);
const rootCount = computed(() => groups.value.filter(g => g.label === "").length);
const totalLabel = computed(() => {
  const parts = [];
  if (groupCount.value) parts.push(`${groupCount.value} folder${groupCount.value !== 1 ? "s" : ""}`);
  if (rootCount.value) parts.push(`${rootCount.value} file${rootCount.value !== 1 ? "s" : ""}`);
  return parts.join(", ");
});

// Track which folder groups are collapsed (expanded by default)
const collapsed = ref(new Set<string>());
function toggleGroup(dir: string) {
  if (collapsed.value.has(dir)) {
    collapsed.value.delete(dir);
  } else {
    collapsed.value.add(dir);
  }
  collapsed.value = new Set(collapsed.value); // trigger reactivity
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
        {{ totalLabel || `${books.length} files` }}
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
      <p class="text-xs text-slate-400 dark:text-slate-500">No downloaded books yet.</p>
    </div>

    <!-- Grouped list -->
    <div v-else class="flex-1 overflow-y-auto">
      <template v-for="group in groups" :key="group.dir">

        <!-- Folder group header (only for files in subdirectories) -->
        <button
          v-if="group.label"
          class="w-full flex items-center gap-2 px-3 py-2 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors border-b border-slate-100 dark:border-slate-800/60 text-left"
          @click="toggleGroup(group.dir)">
          <component
            :is="collapsed.has(group.dir) ? Folder : FolderOpen"
            :size="13"
            class="flex-shrink-0 text-brand-400" />
          <span class="flex-1 min-w-0 text-xs font-semibold text-slate-700 dark:text-slate-200 truncate">
            {{ group.label }}
          </span>
          <span class="flex-shrink-0 text-[10px] text-slate-400 dark:text-slate-500 tabular-nums">
            {{ group.files.length }}
          </span>
        </button>

        <!-- Files in this group -->
        <ul
          v-if="!group.label || !collapsed.has(group.dir)"
          class="divide-y divide-slate-100/60 dark:divide-slate-800/40">
          <li
            v-for="book in group.files"
            :key="book.path"
            class="group flex items-center gap-2 py-1.5 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
            :class="group.label ? 'pl-7 pr-3' : 'px-3'">
            <Library
              :size="12"
              class="flex-shrink-0 text-slate-300 dark:text-slate-600" />
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium text-slate-700 dark:text-slate-200 truncate">
                {{ book.name }}
              </p>
              <p class="text-[10px] text-slate-400 dark:text-slate-500">
                {{ formatDate(book.time) }}
              </p>
            </div>
            <div
              class="flex-shrink-0 flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-brand-500 transition-colors"
                title="Download"
                @click="downloadBook(book.downloadLink)">
                <DownloadCloud :size="12" />
              </button>
              <button
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-red-400 transition-colors"
                title="Delete"
                @click="handleDelete(book.path, book.name)">
                <Trash2 :size="12" />
              </button>
            </div>
          </li>
        </ul>

      </template>
    </div>
  </div>
</template>
