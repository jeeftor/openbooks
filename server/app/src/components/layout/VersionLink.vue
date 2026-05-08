<script setup lang="ts">
import type { VersionInfo } from "../../types/messages";

defineProps<{
  version: VersionInfo;
}>();
</script>

<template>
  <span class="flex items-center gap-1 mr-1 min-w-0">
    <a
      v-if="version.isRelease && version.releaseNotesUrl"
      class="inline-block max-w-24 truncate text-xs text-slate-400 dark:text-slate-500 hover:text-brand-500 dark:hover:text-brand-300 whitespace-nowrap transition-colors"
      :href="version.releaseNotesUrl"
      target="_blank"
      rel="noopener noreferrer"
      :title="`View release notes for ${version.displayVersion}`"
    >
      {{ version.displayVersion }}
    </a>
    <span
      v-else
      class="inline-block max-w-24 truncate text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap"
      :title="version.rawVersion"
    >
      {{ version.displayVersion }}
    </span>
    <a
      v-if="version.update?.available && version.update.releaseNotesUrl"
      class="inline-block rounded border border-amber-300/70 dark:border-amber-500/40 px-1 py-0.5 text-[10px] leading-none font-medium text-amber-700 dark:text-amber-300 hover:bg-amber-50 dark:hover:bg-amber-500/10 whitespace-nowrap transition-colors"
      :href="version.update.releaseNotesUrl"
      target="_blank"
      rel="noopener noreferrer"
      :title="`Update available: ${version.update.latestVersion}`"
    >
      Update
    </a>
  </span>
</template>
