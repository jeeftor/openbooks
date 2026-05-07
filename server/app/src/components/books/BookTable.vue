<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useVirtualizer } from "@tanstack/vue-virtual";
import { User, Search, Circle, Eye, EyeOff, Star, Wifi, ChevronDown, ChevronUp, Layers, Download } from "lucide-vue-next";
import type { BookDetail } from "../../types/messages";
import { useServers } from "../../composables/useApi";
import { usePreferencesStore } from "../../stores/preferences";
import DownloadButton from "./DownloadButton.vue";

const props = defineProps<{ books: BookDetail[] }>();

const prefStore = usePreferencesStore();
const { servers } = useServers();
const scrollContainer = ref<HTMLElement | null>(null);

type SortColumn = 'server' | 'author' | 'title' | 'format' | 'size';
type SortDirection = 'asc' | 'desc';

const sortColumn = ref<SortColumn>('server');
const sortDirection = ref<SortDirection>('asc');

const sortedBooks = computed(() => {
  const sorted = [...props.books].sort((a, b) => {
    let cmp = 0;
    switch (sortColumn.value) {
      case 'server':
        cmp = a.server.localeCompare(b.server);
        break;
      case 'author':
        cmp = a.author.localeCompare(b.author);
        break;
      case 'title':
        cmp = a.title.localeCompare(b.title);
        break;
      case 'format':
        cmp = a.format.localeCompare(b.format);
        break;
      case 'size':
        // Parse size strings like "1.2MB" for numeric comparison
        const aNum = parseFloat(a.size) || 0;
        const bNum = parseFloat(b.size) || 0;
        cmp = aNum - bNum;
        break;
    }
    return sortDirection.value === 'asc' ? cmp : -cmp;
  });
  // Always put online servers first regardless of sort
  if (servers.value.length) {
    return sorted.sort((a, b) => {
      const aOn = servers.value.includes(a.server) ? 0 : 1;
      const bOn = servers.value.includes(b.server) ? 0 : 1;
      return aOn - bOn;
    });
  }
  return sorted;
});

function toggleSort(col: SortColumn) {
  if (sortColumn.value === col) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortColumn.value = col;
    sortDirection.value = 'asc';
  }
}

const serverFilter = ref<string[]>([]);
const onlineOnly = ref(false);
const serverDropdownOpen = ref(false);
const authorFilter = ref("");
const titleFilter = ref("");
const formatFilter = ref<string[]>([...prefStore.preferredFormats]);
const groupBooks = ref(false);
const expandedGroups = ref<Set<string>>(new Set());

const allServers = computed(() =>
  [...new Set(sortedBooks.value.map((b) => b.server))].filter(Boolean).sort()
);
const allFormats = computed(() =>
  [...new Set(sortedBooks.value.map((b) => b.format))].filter(Boolean).sort()
);

function matchesBook(book: BookDetail): boolean {
  if (onlineOnly.value && !servers.value.includes(book.server)) return false;
  if (serverFilter.value.length && !serverFilter.value.includes(book.server)) return false;
  if (authorFilter.value && !book.author.toLowerCase().includes(authorFilter.value.toLowerCase())) return false;
  if (titleFilter.value && !book.title.toLowerCase().includes(titleFilter.value.toLowerCase())) return false;
  if (formatFilter.value.length && !formatFilter.value.includes(book.format)) return false;
  return true;
}

function toggleServer(srv: string) {
  serverFilter.value = serverFilter.value.includes(srv)
    ? serverFilter.value.filter(s => s !== srv)
    : [...serverFilter.value, srv];
}

const matchedBooks = computed(() => sortedBooks.value.filter(matchesBook));
const hiddenBooks = computed(() => sortedBooks.value.filter((b) => !matchesBook(b)));
const hiddenCount = computed(() => hiddenBooks.value.length);

// Grouping logic for EPUBs only
interface BookGroup {
  key: string;
  books: BookDetail[];
  representative: BookDetail;
}

function normalizeString(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9]/g, '');
}

function levenshteinDistance(a: string, b: string): number {
  const matrix: number[][] = [];
  for (let i = 0; i <= b.length; i++) {
    matrix[i] = [i];
  }
  for (let j = 0; j <= a.length; j++) {
    matrix[0][j] = j;
  }
  for (let i = 1; i <= b.length; i++) {
    for (let j = 1; j <= a.length; j++) {
      if (b.charAt(i - 1) === a.charAt(j - 1)) {
        matrix[i][j] = matrix[i - 1][j - 1];
      } else {
        matrix[i][j] = Math.min(
          matrix[i - 1][j - 1] + 1,
          matrix[i][j - 1] + 1,
          matrix[i - 1][j] + 1
        );
      }
    }
  }
  return matrix[b.length][a.length];
}

function similarity(a: string, b: string): number {
  const distance = levenshteinDistance(a, b);
  const maxLen = Math.max(a.length, b.length);
  return maxLen === 0 ? 1 : 1 - distance / maxLen;
}

function areSimilar(a: BookDetail, b: BookDetail): boolean {
  // Only group EPUBs
  if (a.format?.toLowerCase() !== 'epub' || b.format?.toLowerCase() !== 'epub') return false;
  
  // Author must be very similar (>85%)
  const authorSim = similarity(normalizeString(a.author), normalizeString(b.author));
  if (authorSim < 0.85) return false;
  
  // Title must be very similar (>85%)
  const titleSim = similarity(normalizeString(a.title), normalizeString(b.title));
  if (titleSim < 0.85) return false;
  
  // Size should be within 10%
  const aSize = parseFloat(a.size) || 0;
  const bSize = parseFloat(b.size) || 0;
  if (aSize > 0 && bSize > 0) {
    const sizeDiff = Math.abs(aSize - bSize) / Math.max(aSize, bSize);
    if (sizeDiff > 0.1) return false;
  }
  
  return true;
}

const groupedBooks = computed(() => {
  if (!groupBooks.value) return null;
  
  const groups: BookGroup[] = [];
  const processed = new Set<number>();
  
  matchedBooks.value.forEach((book, idx) => {
    if (processed.has(idx)) return;
    
    const group: BookDetail[] = [book];
    processed.add(idx);
    
    // Find similar books
    for (let i = idx + 1; i < matchedBooks.value.length; i++) {
      if (processed.has(i)) continue;
      if (areSimilar(book, matchedBooks.value[i])) {
        group.push(matchedBooks.value[i]);
        processed.add(i);
      }
    }
    
    // Only create a group if there are duplicates
    if (group.length > 1) {
      // Pick representative: online first, then largest
      const representative = group.sort((a, b) => {
        const aOnline = servers.value.includes(a.server) ? 0 : 1;
        const bOnline = servers.value.includes(b.server) ? 0 : 1;
        if (aOnline !== bOnline) return aOnline - bOnline;
        const aSize = parseFloat(a.size) || 0;
        const bSize = parseFloat(b.size) || 0;
        return bSize - aSize;
      })[0];
      
      groups.push({
        key: `${normalizeString(book.author)}-${normalizeString(book.title)}`,
        books: group,
        representative
      });
    } else {
      // Single book, not grouped
      groups.push({
        key: `single-${idx}`,
        books: [book],
        representative: book
      });
    }
  });
  
  return groups;
});

// Track which books are in expanded groups for styling
interface DisplayBook extends BookDetail {
  _isGroupMember?: boolean;
  _isGroupRepresentative?: boolean;
  _groupKey?: string;
}

const displayBooks = computed<DisplayBook[]>(() => {
  if (groupBooks.value && groupedBooks.value) {
    // Flatten groups, showing only representative or all if expanded
    const result: DisplayBook[] = [];
    groupedBooks.value.forEach(group => {
      if (group.books.length === 1) {
        result.push(group.books[0]);
      } else if (expandedGroups.value.has(group.key)) {
        // Mark all books in expanded group
        group.books.forEach((book, idx) => {
          result.push({
            ...book,
            _isGroupMember: true,
            _isGroupRepresentative: idx === 0,
            _groupKey: group.key
          });
        });
      } else {
        result.push({
          ...group.representative,
          _isGroupRepresentative: true,
          _groupKey: group.key
        });
      }
    });
    return result;
  }
  
  return matchedBooks.value;
});

function getGroupForBook(book: BookDetail): BookGroup | null {
  if (!groupBooks.value || !groupedBooks.value) return null;
  return groupedBooks.value.find(g => g.books.includes(book)) || null;
}

function toggleGroup(groupKey: string) {
  if (expandedGroups.value.has(groupKey)) {
    expandedGroups.value.delete(groupKey);
  } else {
    expandedGroups.value.add(groupKey);
  }
  expandedGroups.value = new Set(expandedGroups.value);
}

function exportResults() {
  const data = {
    timestamp: new Date().toISOString(),
    totalBooks: props.books.length,
    matchedBooks: matchedBooks.value.length,
    filters: {
      servers: serverFilter.value,
      onlineOnly: onlineOnly.value,
      author: authorFilter.value,
      title: titleFilter.value,
      formats: formatFilter.value
    },
    books: props.books
  };
  
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `openbooks-abs-results-${new Date().toISOString().split('T')[0]}.json`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

const isMatched = (idx: number) =>
  !prefStore.showUnmatched || idx < matchedBooks.value.length;

watch(
  () => props.books,
  () => {
    serverFilter.value = [];
    onlineOnly.value = false;
    authorFilter.value = "";
    titleFilter.value = "";
    formatFilter.value = [...prefStore.preferredFormats];
  }
);

// ── Column resizing ──────────────────────────────────────────────────────────
// Widths in px: [server, author, title, format, size, actions]
const columnWidths = ref([110, 160, 320, 72, 64, 88]);

function startResize(colIdx: number, event: MouseEvent) {
  event.preventDefault();
  const startX = event.clientX;
  const startWidth = columnWidths.value[colIdx];

  function onMove(e: MouseEvent) {
    const newWidth = Math.max(40, startWidth + e.clientX - startX);
    columnWidths.value = columnWidths.value.map((w, i) => (i === colIdx ? newWidth : w));
  }
  function onUp() {
    document.removeEventListener("mousemove", onMove);
    document.removeEventListener("mouseup", onUp);
  }
  document.addEventListener("mousemove", onMove);
  document.addEventListener("mouseup", onUp);
}

const virtualizer = useVirtualizer(
  computed(() => ({
    count: displayBooks.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 48,
    overscan: 10
  }))
);

const virtualItems = computed(() => virtualizer.value.getVirtualItems());
const paddingTop = computed(() => virtualItems.value[0]?.start ?? 0);
const paddingBottom = computed(() => {
  const last = virtualItems.value[virtualItems.value.length - 1];
  return last ? virtualizer.value.getTotalSize() - last.end : 0;
});

const prefMatchesCurrent = computed(() => {
  const p = prefStore.preferredFormats;
  const c = formatFilter.value;
  return p.length === c.length && p.every((f) => c.includes(f));
});

function toggleFormat(fmt: string) {
  formatFilter.value = formatFilter.value.includes(fmt)
    ? formatFilter.value.filter((f) => f !== fmt)
    : [...formatFilter.value, fmt];
}
</script>

<template>
  <div class="flex flex-col h-full overflow-hidden">
    <!-- Filter bar -->
    <div class="flex-shrink-0 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900">
      <!-- Row 1: text filters + count -->
      <div class="flex items-center gap-2 px-3 pt-2 pb-1 flex-wrap">
        <!-- Server filter dropdown -->
        <div class="relative">
          <button
            class="flex items-center gap-1 text-xs px-2 py-0.5 rounded border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
            @click="serverDropdownOpen = !serverDropdownOpen">
            <Circle :size="9" class="text-slate-400" />
            Servers
            <span v-if="serverFilter.length" class="text-brand-400">({{ serverFilter.length }})</span>
          </button>
          <div v-if="serverDropdownOpen" class="absolute top-full left-0 mt-1 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded shadow-lg z-20 min-w-[140px] py-1">
            <label class="flex items-center gap-2 px-3 py-1 hover:bg-slate-50 dark:hover:bg-slate-700 cursor-pointer">
              <input type="checkbox" v-model="onlineOnly" class="rounded text-brand-400 focus:ring-brand-400" />
              <Wifi :size="11" class="text-green-500" />
              <span class="text-xs text-slate-700 dark:text-slate-300">Online only</span>
            </label>
            <div class="border-t border-slate-200 dark:border-slate-700 my-1"></div>
            <label
              v-for="srv in allServers"
              :key="srv"
              class="flex items-center gap-2 px-3 py-1 hover:bg-slate-50 dark:hover:bg-slate-700 cursor-pointer">
              <input type="checkbox" :value="srv" @change="toggleServer(srv)" :checked="serverFilter.includes(srv)" class="rounded text-brand-400 focus:ring-brand-400" />
              <span
                class="inline-block w-1.5 h-1.5 rounded-full flex-shrink-0"
                :class="servers.includes(srv) ? 'bg-green-500' : 'bg-slate-300 dark:bg-slate-600'" />
              <span class="text-xs text-slate-700 dark:text-slate-300">{{ srv }}</span>
            </label>
            <div v-if="serverFilter.length" class="border-t border-slate-200 dark:border-slate-700 mt-1 pt-1 px-3">
              <button class="text-xs text-red-400 hover:text-red-500" @click="serverFilter = []">Clear all</button>
            </div>
          </div>
        </div>
        <!-- Author -->
        <div class="flex items-center gap-1">
          <User :size="11" class="text-slate-400 flex-shrink-0" />
          <input
            v-model="authorFilter"
            type="text" placeholder="Author…"
            class="text-xs rounded border border-slate-200 dark:border-slate-700 bg-transparent text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-brand-400 px-2 py-0.5 w-24" />
          <button v-if="authorFilter" class="text-slate-400 hover:text-red-400 text-xs" @click="authorFilter = ''">✕</button>
        </div>
        <!-- Title -->
        <div class="flex items-center gap-1">
          <Search :size="11" class="text-slate-400 flex-shrink-0" />
          <input
            v-model="titleFilter"
            type="text" placeholder="Title…"
            class="text-xs rounded border border-slate-200 dark:border-slate-700 bg-transparent text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-brand-400 px-2 py-0.5 w-40" />
          <button v-if="titleFilter" class="text-slate-400 hover:text-red-400 text-xs" @click="titleFilter = ''">✕</button>
        </div>
        <!-- Right: stats + show-unmatched toggle -->
        <div class="ml-auto flex items-center gap-2">
          <button
            v-if="hiddenCount > 0"
            class="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border transition-colors"
            :class="prefStore.showUnmatched
              ? 'border-slate-400 text-slate-600 dark:text-slate-300 bg-slate-100 dark:bg-slate-800'
              : 'border-slate-200 dark:border-slate-700 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300'"
            :title="prefStore.showUnmatched ? 'Hide filtered items' : 'Show filtered items dimmed'"
            @click="prefStore.showUnmatched = !prefStore.showUnmatched">
            <EyeOff v-if="prefStore.showUnmatched" :size="11" />
            <Eye v-else :size="11" />
            {{ hiddenCount }} hidden
          </button>
          <span class="text-xs text-slate-400 tabular-nums whitespace-nowrap">
            {{ matchedBooks.length }}<span class="text-slate-300 dark:text-slate-600">/{{ books.length }}</span>
          </span>
        </div>
      </div>

      <!-- Row 2: format chips + save preference -->
      <div class="flex items-center gap-1.5 px-3 pb-2 flex-wrap">
        <button
          class="px-2 py-0.5 rounded text-[11px] font-medium border transition-colors"
          :class="!formatFilter.length
            ? 'bg-brand-400 border-brand-400 text-white'
            : 'border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-brand-300'"
          @click="formatFilter = []">
          All
        </button>
        <button
          v-for="fmt in allFormats"
          :key="fmt"
          class="px-2 py-0.5 rounded text-[11px] font-medium border transition-colors"
          :class="formatFilter.includes(fmt)
            ? 'bg-brand-400 border-brand-400 text-white'
            : 'border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-brand-300'"
          @click="toggleFormat(fmt)">
          {{ fmt.toUpperCase() }}
        </button>
        <!-- Group Books toggle -->
        <button
          class="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded border transition-colors whitespace-nowrap"
          :class="groupBooks
            ? 'bg-brand-400 border-brand-400 text-white'
            : 'border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-brand-300'"
          title="Group duplicate EPUBs together"
          @click="groupBooks = !groupBooks; expandedGroups.clear()">
          <Layers :size="10" />
          Group Books
        </button>
        <!-- Export button -->
        <button
          class="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded border border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-brand-300 transition-colors whitespace-nowrap"
          title="Export all results as JSON"
          @click="exportResults">
          <Download :size="10" />
          Export
        </button>
        <!-- Save / clear preference -->
        <div class="ml-auto flex items-center gap-1.5">
          <button
            v-if="!prefMatchesCurrent"
            class="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded border border-brand-300 dark:border-brand-700 text-brand-500 dark:text-brand-400 hover:bg-brand-50 dark:hover:bg-brand-900/20 transition-colors whitespace-nowrap"
            title="Save current format filter as default for all future searches"
            @click="prefStore.setPreferredFormats([...formatFilter])">
            <Star :size="10" />
            Save default
          </button>
          <button
            v-else-if="prefStore.preferredFormats.length > 0"
            class="text-[11px] px-2 py-0.5 rounded border border-slate-200 dark:border-slate-700 text-slate-400 hover:text-red-400 hover:border-red-300 transition-colors whitespace-nowrap"
            title="Clear saved default format"
            @click="prefStore.clearPreferredFormats(); formatFilter = []">
            ✕ clear default
          </button>
        </div>
      </div>
    </div>

    <!-- Virtualised table -->
    <div ref="scrollContainer" class="flex-1 overflow-auto">
      <table class="w-full text-xs border-collapse" style="table-layout: fixed">
        <colgroup>
          <col v-for="(w, i) in columnWidths" :key="i" :style="{ width: w + 'px' }" />
        </colgroup>
        <thead class="sticky top-0 bg-slate-50 dark:bg-slate-900/95 z-10 backdrop-blur-sm">
          <tr>
            <th class="relative px-3 py-2 text-left font-medium border-b border-slate-200 dark:border-slate-800 select-none">
              <button class="flex items-center gap-1 text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors" @click="toggleSort('server')">
                Server
                <ChevronDown v-if="sortColumn === 'server' && sortDirection === 'asc'" :size="12" />
                <ChevronUp v-else-if="sortColumn === 'server' && sortDirection === 'desc'" :size="12" />
              </button>
              <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-brand-400/60 opacity-0 hover:opacity-100 transition-opacity" @mousedown.stop="startResize(0, $event)" />
            </th>
            <th class="relative px-3 py-2 text-left font-medium border-b border-slate-200 dark:border-slate-800 select-none">
              <button class="flex items-center gap-1 text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors" @click="toggleSort('author')">
                Author
                <ChevronDown v-if="sortColumn === 'author' && sortDirection === 'asc'" :size="12" />
                <ChevronUp v-else-if="sortColumn === 'author' && sortDirection === 'desc'" :size="12" />
              </button>
              <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-brand-400/60 opacity-0 hover:opacity-100 transition-opacity" @mousedown.stop="startResize(1, $event)" />
            </th>
            <th class="relative px-3 py-2 text-left font-medium border-b border-slate-200 dark:border-slate-800 select-none">
              <button class="flex items-center gap-1 text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors" @click="toggleSort('title')">
                Title
                <ChevronDown v-if="sortColumn === 'title' && sortDirection === 'asc'" :size="12" />
                <ChevronUp v-else-if="sortColumn === 'title' && sortDirection === 'desc'" :size="12" />
              </button>
              <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-brand-400/60 opacity-0 hover:opacity-100 transition-opacity" @mousedown.stop="startResize(2, $event)" />
            </th>
            <th class="relative px-3 py-2 text-left font-medium border-b border-slate-200 dark:border-slate-800 select-none">
              <button class="flex items-center gap-1 text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors" @click="toggleSort('format')">
                Format
                <ChevronDown v-if="sortColumn === 'format' && sortDirection === 'asc'" :size="12" />
                <ChevronUp v-else-if="sortColumn === 'format' && sortDirection === 'desc'" :size="12" />
              </button>
              <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-brand-400/60 opacity-0 hover:opacity-100 transition-opacity" @mousedown.stop="startResize(3, $event)" />
            </th>
            <th class="relative px-3 py-2 text-left font-medium border-b border-slate-200 dark:border-slate-800 select-none">
              <button class="flex items-center gap-1 text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors" @click="toggleSort('size')">
                Size
                <ChevronDown v-if="sortColumn === 'size' && sortDirection === 'asc'" :size="12" />
                <ChevronUp v-else-if="sortColumn === 'size' && sortDirection === 'desc'" :size="12" />
              </button>
              <div class="absolute right-0 top-0 h-full w-1 cursor-col-resize hover:bg-brand-400/60 opacity-0 hover:opacity-100 transition-opacity" @mousedown.stop="startResize(4, $event)" />
            </th>
            <th class="px-3 py-2 border-b border-slate-200 dark:border-slate-800"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="paddingTop > 0"><td :style="{ height: paddingTop + 'px' }" colspan="6" /></tr>

          <tr
            v-for="vItem in virtualItems"
            :key="String(vItem.key)"
            class="border-b border-slate-100 dark:border-slate-800/60 h-12 transition-all"
            :class="[
              isMatched(vItem.index)
                ? 'hover:bg-slate-50 dark:hover:bg-slate-800/40'
                : 'opacity-30',
              displayBooks[vItem.index]?._isGroupMember
                ? 'bg-brand-50/30 dark:bg-brand-900/10 border-l-2 border-l-brand-400'
                : ''
            ]">
            <template v-if="displayBooks[vItem.index]">
              <td class="py-1.5" :class="displayBooks[vItem.index]?._isGroupMember ? 'pl-6 pr-3' : 'px-3'">
                <div class="flex items-center gap-1.5">
                  <!-- Group indicator -->
                  <button
                    v-if="groupBooks && displayBooks[vItem.index]?._groupKey && (() => {
                      const g = groupedBooks?.find(gr => gr.key === displayBooks[vItem.index]._groupKey);
                      return g && g.books.length > 1;
                    })()"
                    class="flex-shrink-0 text-slate-400 hover:text-brand-400 transition-colors"
                    :title="(() => {
                      const g = groupedBooks?.find(gr => gr.key === displayBooks[vItem.index]._groupKey);
                      return `${g?.books.length || 0} sources`;
                    })()"
                    @click="() => {
                      const key = displayBooks[vItem.index]._groupKey;
                      if (key) toggleGroup(key);
                    }">
                    <Layers :size="12" />
                    <span class="text-[10px] ml-0.5">{{ (() => {
                      const g = groupedBooks?.find(gr => gr.key === displayBooks[vItem.index]._groupKey);
                      return g?.books.length || 0;
                    })() }}</span>
                  </button>
                  <span
                    class="inline-block w-1.5 h-1.5 rounded-full flex-shrink-0"
                    :class="servers.includes(displayBooks[vItem.index].server) ? 'bg-green-500' : 'bg-slate-300 dark:bg-slate-600'" />
                  <span class="truncate text-slate-600 dark:text-slate-300">
                    {{ displayBooks[vItem.index].server }}
                  </span>
                </div>
              </td>
              <td class="px-3 py-1.5 text-slate-700 dark:text-slate-300 overflow-hidden">
                <span class="truncate block">{{ displayBooks[vItem.index].author }}</span>
              </td>
              <td class="px-3 py-1.5 text-slate-900 dark:text-slate-100 overflow-hidden">
                <span class="truncate block">{{ displayBooks[vItem.index].title }}</span>
              </td>
              <td class="px-3 py-1.5">
                <span
                  v-if="displayBooks[vItem.index].format"
                  class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-brand-50 dark:bg-brand-900/30 text-brand-600 dark:text-brand-300">
                  {{ displayBooks[vItem.index].format.toUpperCase() }}
                </span>
              </td>
              <td class="px-3 py-1.5 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {{ displayBooks[vItem.index].size }}
              </td>
              <td class="px-3 py-1.5">
                <DownloadButton
                  v-if="isMatched(vItem.index)"
                  :book="displayBooks[vItem.index].full"
                  :author="displayBooks[vItem.index].author"
                  :title="displayBooks[vItem.index].title" />
              </td>
            </template>
          </tr>

          <tr v-if="paddingBottom > 0"><td :style="{ height: paddingBottom + 'px' }" colspan="6" /></tr>
        </tbody>
      </table>

      <div
        v-if="matchedBooks.length === 0 && books.length > 0"
        class="py-12 text-center">
        <p class="text-sm text-slate-400 mb-2">No results match your filters.</p>
        <button
          v-if="!prefStore.showUnmatched"
          class="text-xs text-brand-400 hover:text-brand-500 underline"
          @click="prefStore.showUnmatched = true">
          Show {{ hiddenCount }} hidden results
        </button>
      </div>
    </div>
  </div>
</template>
