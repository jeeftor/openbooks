<script setup lang="ts">
defineProps<{ format?: string }>();
</script>

<template>
  <span
    v-if="format"
    class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-brand-50 dark:bg-brand-900/30 text-brand-600 dark:text-brand-300">
    {{ format.toUpperCase() }}
  </span>
</template>
