<script setup lang="ts">
defineProps<{ active: boolean }>();
defineEmits<{ toggle: [] }>();
</script>

<template>
  <button
    class="px-2 py-0.5 rounded text-[11px] font-medium border transition-colors"
    :class="active
      ? 'bg-brand-400 border-brand-400 text-white'
      : 'border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-brand-300'"
    @click="$emit('toggle')">
    <slot />
  </button>
</template>
