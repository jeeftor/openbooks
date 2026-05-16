<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { SlidersHorizontal, Eye, EyeOff } from "lucide-vue-next";
import { useVirtualizer } from "@tanstack/vue-virtual";
import type { BookDetail } from "../../types/messages";
import { useServers } from "../../composables/useApi";
import { usePreferencesStore } from "../../stores/preferences";
import DownloadButton from "./DownloadButton.vue";
import FilterDrawer from "./FilterDrawer.vue";

const props = defineProps<{ books: BookDetail[] }>();

const prefStore = usePreferencesStore();
const { servers } = useServers();
const scrollContainer = ref<HTMLElement | null>(null);
const filterOpen = ref(false);

const authorFilter = ref("");
const titleFilter = ref("");
const formatFilter = ref(prefStore.preferredFormats[0] ?? "");
const excludeNoSize = ref(false);

const sortedBooks = computed(() => {
  if (!servers.value.length) return props.books;
  return [...props.books].sort((a, b) => {
    const aOn = servers.value.includes(a.server) ? 0 : 1;
    const bOn = servers.value.includes(b.server) ? 0 : 1;
    return aOn - bOn;
  });
});

const formats = computed(() =>
  [...new Set(sortedBooks.value.map((b) => b.format))].filter(Boolean).sort()
);

function matchesBook(b: BookDetail) {
  if (authorFilter.value && !b.author.toLowerCase().includes(authorFilter.value.toLowerCase())) return false;
  if (titleFilter.value && !b.title.toLowerCase().includes(titleFilter.value.toLowerCase())) return false;
  if (formatFilter.value && b.format !== formatFilter.value) return false;
  if (excludeNoSize.value && (!b.size || b.size === "N/A")) return false;
  return true;
}

const matchedBooks = computed(() => sortedBooks.value.filter(matchesBook));
const hiddenBooks = computed(() => sortedBooks.value.filter((b) => !matchesBook(b)));
const hiddenCount = computed(() => hiddenBooks.value.length);

const displayBooks = computed(() =>
  prefStore.showUnmatched && hiddenCount.value > 0
    ? [...matchedBooks.value, ...hiddenBooks.value]
    : matchedBooks.value
);

const activeFilterCount = computed(
  () => [authorFilter.value, titleFilter.value, formatFilter.value].filter(Boolean).length
);

const virtualizer = useVirtualizer(
  computed(() => ({
    count: displayBooks.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 82,
    overscan: 8
  }))
);

watch(
  () => props.books,
  () => {
    authorFilter.value = "";
    titleFilter.value = "";
    formatFilter.value = prefStore.preferredFormats[0] ?? "";
    excludeNoSize.value = false;
  }
);
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Filter bar -->
    <div
      class="flex-shrink-0 flex items-center justify-between gap-2 px-4 py-2 border-b border-slate-200 dark:border-slate-700">
      <div class="flex items-center gap-2">
        <span class="text-xs text-slate-500 dark:text-slate-400">
          {{ matchedBooks.length }}<span class="text-slate-300 dark:text-slate-600">/{{ books.length }}</span>
        </span>
        <!-- Show unmatched toggle -->
        <button
          v-if="hiddenCount > 0"
          class="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border transition-colors"
          :class="prefStore.showUnmatched
            ? 'border-slate-400 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'
            : 'border-slate-200 dark:border-slate-700 text-slate-400'"
          @click="prefStore.showUnmatched = !prefStore.showUnmatched">
          <EyeOff v-if="prefStore.showUnmatched" :size="11" />
          <Eye v-else :size="11" />
          {{ hiddenCount }} hidden
        </button>
      </div>
      <button
        class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors"
        :class="activeFilterCount > 0
          ? 'bg-brand-400 text-white'
          : 'hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-600 dark:text-slate-300'"
        @click="filterOpen = true">
        <SlidersHorizontal :size="14" />
        Filter
        <span v-if="activeFilterCount > 0">({{ activeFilterCount }})</span>
      </button>
    </div>

    <!-- Virtual list -->
    <div ref="scrollContainer" class="flex-1 overflow-auto">
      <div :style="{ height: virtualizer.getTotalSize() + 'px', position: 'relative' }">
        <div
          v-for="vItem in virtualizer.getVirtualItems()"
          :key="displayBooks[vItem.index]?.full ?? String(vItem.key)"
          :style="{
            position: 'absolute',
            top: 0, left: 0, right: 0,
            transform: `translateY(${vItem.start}px)`,
            padding: '4px 12px'
          }"
          :class="prefStore.showUnmatched && vItem.index >= matchedBooks.length ? 'opacity-30' : ''">
          <div class="flex items-center gap-3 p-3 rounded-xl bg-white dark:bg-slate-800/80 border border-slate-100 dark:border-slate-700/60 shadow-sm">
            <!-- Text -->
            <div class="flex-1 min-w-0">
              <p class="text-sm font-semibold text-slate-900 dark:text-slate-50 truncate leading-snug">
                {{ displayBooks[vItem.index]?.title }}
              </p>
              <p class="text-xs text-slate-500 dark:text-slate-400 truncate">
                {{ displayBooks[vItem.index]?.author }}
              </p>
              <div class="flex items-center gap-2 mt-1">
                <span
                  v-if="displayBooks[vItem.index]?.format"
                  class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-brand-50 dark:bg-brand-900/30 text-brand-600 dark:text-brand-300">
                  {{ displayBooks[vItem.index]?.format.toUpperCase() }}
                </span>
                <span
                  v-if="displayBooks[vItem.index]?.size"
                  class="text-[11px] text-slate-400 dark:text-slate-500">
                  {{ displayBooks[vItem.index]?.size }}
                </span>
                <span
                  class="text-[10px] px-1.5 py-0.5 rounded"
                  :class="servers.includes(displayBooks[vItem.index]?.server)
                    ? 'bg-green-50 dark:bg-green-900/30 text-green-600 dark:text-green-400'
                    : 'bg-slate-100 dark:bg-slate-700 text-slate-400'">
                  {{ displayBooks[vItem.index]?.server }}
                </span>
              </div>
            </div>
            <!-- Download (only for matched items) -->
            <DownloadButton
              v-if="!(prefStore.showUnmatched && vItem.index >= matchedBooks.length)"
              :key="displayBooks[vItem.index]?.full ?? String(vItem.key)"
              :book="displayBooks[vItem.index]?.full ?? ''"
              :author="displayBooks[vItem.index]?.author ?? ''"
              :title="displayBooks[vItem.index]?.title ?? ''"
              compact />
          </div>
        </div>
      </div>
    </div>

    <!-- Filter drawer -->
    <FilterDrawer
      v-model:open="filterOpen"
      :formats="formats"
      v-model:author-filter="authorFilter"
      v-model:title-filter="titleFilter"
      v-model:format-filter="formatFilter"
      v-model:exclude-no-size="excludeNoSize" />
  </div>
</template>
