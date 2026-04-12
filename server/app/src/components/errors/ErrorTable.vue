<script setup lang="ts">
import { ref, computed } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { AlertTriangle, Search, Download } from "lucide-vue-next";
import type { ParseError } from "../../types/messages";

const props = defineProps<{ errors: ParseError[] }>();
const emit = defineEmits<{ download: [cmd: string] }>();

const isMobile = useMediaQuery("(max-width: 767px)");
const filter = ref("");
const filtered = computed(() => {
  if (!filter.value) return props.errors;
  const q = filter.value.toLowerCase();
  return props.errors.filter(
    (e) => e.line.toLowerCase().includes(q) || e.error.toLowerCase().includes(q)
  );
});
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <div
      class="flex-shrink-0 border-b border-amber-200 dark:border-amber-800/50 bg-amber-50 dark:bg-amber-900/20">
      <div class="flex items-center gap-2 px-4 py-2.5">
        <AlertTriangle :size="14" class="text-amber-500 flex-shrink-0" />
        <span class="text-xs font-medium text-amber-700 dark:text-amber-400">
          {{ errors.length }} parsing {{ errors.length === 1 ? "error" : "errors" }}
          — click Download to send the raw IRC command
        </span>
      </div>
      <div class="flex items-center gap-1.5 px-4 pb-2">
        <Search :size="12" class="text-amber-400 flex-shrink-0" />
        <input
          v-model="filter"
          type="text"
          placeholder="Filter errors…"
          class="flex-1 text-xs rounded border border-amber-200 dark:border-amber-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-1 focus:ring-amber-400 px-2 py-0.5" />
        <span v-if="filter" class="text-xs text-amber-600 dark:text-amber-400 whitespace-nowrap">
          {{ filtered.length }}/{{ errors.length }}
        </span>
        <button v-if="filter" class="text-amber-400 hover:text-red-400 text-xs" @click="filter = ''">✕</button>
      </div>
    </div>

    <div class="flex-1 overflow-auto">
      <table class="w-full text-xs">
        <thead class="sticky top-0 bg-slate-50 dark:bg-slate-900">
          <tr>
            <th
              class="hidden md:table-cell px-4 py-2 text-left font-medium text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700 w-32">
              Error
            </th>
            <th
              class="px-3 md:px-4 py-2 text-left font-medium text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
              Raw line
            </th>
            <th
              class="px-3 md:px-4 py-2 border-b border-slate-200 dark:border-slate-700 w-10 md:w-24"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
          <tr
            v-for="(err, i) in filtered"
            :key="i"
            class="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
            <td class="hidden md:table-cell px-4 py-2 text-red-500 dark:text-red-400 font-medium">
              {{ err.error }}
            </td>
            <td
              class="px-3 md:px-4 py-2 font-mono text-slate-500 dark:text-slate-400"
              :class="isMobile ? '' : 'max-w-0'">
              <span class="block" :class="isMobile ? 'break-all' : 'truncate'" :title="err.line">{{ err.line }}</span>
            </td>
            <td class="px-3 md:px-4 py-2 text-right">
              <!-- Mobile: icon-only button -->
              <button
                v-if="isMobile"
                class="p-1.5 rounded bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 hover:bg-brand-50 dark:hover:bg-brand-900/30 hover:text-brand-600 dark:hover:text-brand-400 transition-colors"
                :title="'Download: ' + err.line"
                @click="emit('download', err.line)">
                <Download :size="14" />
              </button>
              <!-- Desktop: text button -->
              <button
                v-else
                class="px-2 py-1 rounded text-[11px] font-medium bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 hover:bg-brand-50 dark:hover:bg-brand-900/30 hover:text-brand-600 dark:hover:text-brand-400 transition-colors"
                @click="emit('download', err.line)">
                Download
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
