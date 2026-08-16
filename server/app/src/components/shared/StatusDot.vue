<script setup lang="ts">
defineProps<{ online: boolean }>();
</script>

<template>
  <span
    class="inline-block w-1.5 h-1.5 rounded-full flex-shrink-0"
    :class="online ? 'bg-green-500' : 'bg-slate-300 dark:bg-slate-600'"
    :title="online ? 'Online' : 'Offline'" />
</template>
