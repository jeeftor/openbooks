<script setup lang="ts">
import { computed } from "vue";
import { X, Star } from "lucide-vue-next";
import { usePreferencesStore } from "../../stores/preferences";

const props = defineProps<{
  open: boolean;
  formats: string[];
  authorFilter: string;
  titleFilter: string;
  formatFilter: string;
  excludeNoSize: boolean;
}>();

const emit = defineEmits<{
  "update:open": [value: boolean];
  "update:authorFilter": [value: string];
  "update:titleFilter": [value: string];
  "update:formatFilter": [value: string];
  "update:excludeNoSize": [value: boolean];
}>();

const prefStore = usePreferencesStore();

const prefMatchesCurrent = computed(() => {
  const saved = prefStore.preferredFormats;
  if (!props.formatFilter) return saved.length === 0;
  return saved.length === 1 && saved[0] === props.formatFilter;
});

function saveDefault() {
  if (props.formatFilter) {
    prefStore.setPreferredFormats([props.formatFilter]);
  } else {
    prefStore.clearPreferredFormats();
  }
}

function clearAll() {
  emit("update:authorFilter", "");
  emit("update:titleFilter", "");
  emit("update:formatFilter", "");
  emit("update:excludeNoSize", false);
}
</script>

<template>
  <!-- Backdrop -->
  <Transition
    enter-active-class="transition-opacity duration-200"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition-opacity duration-150"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0">
    <div
      v-if="open"
      class="fixed inset-0 bg-black/30 z-[60]"
      @click="emit('update:open', false)" />
  </Transition>

  <!-- Bottom sheet -->
  <Transition
    enter-active-class="transition-transform duration-300 ease-out"
    enter-from-class="translate-y-full"
    enter-to-class="translate-y-0"
    leave-active-class="transition-transform duration-200 ease-in"
    leave-from-class="translate-y-0"
    leave-to-class="translate-y-full">
    <div
      v-if="open"
      class="fixed inset-x-0 bottom-0 z-[70] bg-white dark:bg-slate-900 rounded-t-2xl shadow-2xl px-4 pt-4"
      style="padding-bottom: max(1.5rem, env(safe-area-inset-bottom, 1.5rem))">
      <div class="flex items-center justify-between mb-4">
        <span class="font-semibold text-slate-900 dark:text-slate-50"
          >Filter Results</span
        >
        <button
          class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 transition-colors"
          @click="emit('update:open', false)">
          <X :size="18" />
        </button>
      </div>

      <!-- Author filter -->
      <div class="mb-3">
        <label
          class="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1"
          >Author</label
        >
        <input
          :value="authorFilter"
          type="text"
          placeholder="Filter by author…"
          class="w-full px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-50 focus:outline-none focus:ring-2 focus:ring-brand-400 transition"
          @input="
            emit(
              'update:authorFilter',
              ($event.target as HTMLInputElement).value
            )
          " />
      </div>

      <!-- Title filter -->
      <div class="mb-3">
        <label
          class="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1"
          >Title</label
        >
        <input
          :value="titleFilter"
          type="text"
          placeholder="Filter by title…"
          class="w-full px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-800 text-sm text-slate-900 dark:text-slate-50 focus:outline-none focus:ring-2 focus:ring-brand-400 transition"
          @input="
            emit(
              'update:titleFilter',
              ($event.target as HTMLInputElement).value
            )
          " />
      </div>

      <!-- Format chips -->
      <div class="mb-3">
        <label
          class="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-2"
          >Format</label
        >
        <div class="flex flex-wrap gap-2">
          <button
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
            :class="
              formatFilter === ''
                ? 'bg-brand-400 border-brand-400 text-white'
                : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:border-brand-300'
            "
            @click="emit('update:formatFilter', '')">
            All
          </button>
          <button
            v-for="fmt in formats"
            :key="fmt"
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors"
            :class="
              formatFilter === fmt
                ? 'bg-brand-400 border-brand-400 text-white'
                : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:border-brand-300'
            "
            @click="emit('update:formatFilter', fmt)">
            {{ fmt.toUpperCase() }}
          </button>
        </div>
        <!-- Save / clear default -->
        <div class="mt-2">
          <button
            v-if="!prefMatchesCurrent"
            class="flex items-center gap-1 text-xs px-2 py-1 rounded border border-brand-300 dark:border-brand-700 text-brand-500 dark:text-brand-400 hover:bg-brand-50 dark:hover:bg-brand-900/20 transition-colors"
            @click="saveDefault">
            <Star :size="11" />
            Save as default
          </button>
          <button
            v-else-if="prefStore.preferredFormats.length > 0"
            class="text-xs px-2 py-1 rounded border border-slate-200 dark:border-slate-600 text-slate-400 hover:text-red-400 hover:border-red-300 transition-colors"
            @click="prefStore.clearPreferredFormats(); emit('update:formatFilter', '')">
            ✕ Clear default
          </button>
        </div>
      </div>

      <!-- Size filter -->
      <div class="mb-5">
        <label class="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-2">Size</label>
        <label class="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            :checked="excludeNoSize"
            class="rounded text-brand-400 focus:ring-brand-400"
            @change="emit('update:excludeNoSize', ($event.target as HTMLInputElement).checked)" />
          <span class="text-sm text-slate-700 dark:text-slate-300">Exclude unknown size (N/A)</span>
        </label>
      </div>

      <!-- Actions -->
      <div class="flex gap-3">
        <button
          class="flex-1 py-2.5 rounded-xl border border-slate-200 dark:border-slate-700 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
          @click="clearAll">
          Clear All
        </button>
        <button
          class="flex-1 py-2.5 rounded-xl bg-brand-400 hover:bg-brand-500 text-white text-sm font-medium transition-colors"
          @click="emit('update:open', false)">
          Done
        </button>
      </div>
    </div>
  </Transition>
</template>
