<script setup lang="ts">
withDefaults(defineProps<{
  message: string;
  actionLabel?: string;
}>(), {
  actionLabel: 'Search again'
});

defineEmits<{ refresh: [] }>();
</script>

<template>
  <div
    class="flex-shrink-0 flex items-center justify-between gap-3 px-4 py-2
           bg-amber-50 dark:bg-amber-900/20 border-b
           border-amber-200 dark:border-amber-800
           text-xs text-amber-700 dark:text-amber-400">
    <span>{{ message }}</span>
    <button
      class="flex items-center gap-1 font-medium px-2.5 py-1 rounded border
             border-amber-300 dark:border-amber-700
             hover:bg-amber-100 dark:hover:bg-amber-900/40 transition-colors"
      @click="$emit('refresh')">
      <slot name="icon" />
      {{ actionLabel }}
    </button>
  </div>
</template>
