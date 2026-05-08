<script setup lang="ts">
import type { VersionInfo } from "../../types/messages";

defineProps<{
  version: VersionInfo;
}>();
</script>

<template>
  <a
    v-if="version.isRelease && version.releaseNotesUrl"
    class="inline-block max-w-24 truncate text-xs text-slate-400 dark:text-slate-500 hover:text-brand-500 dark:hover:text-brand-300 mr-1 whitespace-nowrap transition-colors"
    :href="version.releaseNotesUrl"
    target="_blank"
    rel="noopener noreferrer"
    :title="`View release notes for ${version.displayVersion}`"
  >
    {{ version.displayVersion }}
  </a>
  <span
    v-else
    class="inline-block max-w-24 truncate text-xs text-slate-400 dark:text-slate-500 mr-1 whitespace-nowrap"
    :title="version.rawVersion"
  >
    {{ version.displayVersion }}
  </span>
</template>
