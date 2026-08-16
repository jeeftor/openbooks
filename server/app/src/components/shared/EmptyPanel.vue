<script setup lang="ts">
import type { Component } from "vue";
defineProps<{ icon: Component }>();
</script>

<template>
  <div class="flex-1 flex flex-col items-center justify-center gap-2 text-center px-4">
    <component :is="icon" :size="28" class="text-slate-300 dark:text-slate-600" />
    <p class="text-xs text-slate-400 dark:text-slate-500">
      <slot />
    </p>
  </div>
</template>
