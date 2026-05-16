<script setup lang="ts">
import { computed } from "vue";
import { DownloadCloud, Loader, Clock, Sparkles } from "lucide-vue-next";
import { useAppStore } from "../../stores/app";
import { sendMessage } from "../../composables/useWebSocket";
import { MessageType } from "../../types/messages";

const props = defineProps<{
  book: string;
  author?: string;
  title?: string;
  compact?: boolean;
}>();

const appStore = useAppStore();

// clicked is derived from the store's session-scoped set so it persists
// across filter changes and virtual-list instance recycling.
const clicked = computed(() => appStore.clickedDownloads.has(props.book));

function handleDownload() {
  if (clicked.value) return;
  appStore.addInFlightDownload(props.book);
  sendMessage({
    type: MessageType.DOWNLOAD,
    payload: { book: props.book, author: props.author, title: props.title }
  });
}

type Phase = "idle" | "queued" | "waiting" | "transferring" | "cleaning";

const phase = computed((): Phase => {
  if (!appStore.isDownloading(props.book)) return "idle";
  // Only the first item in the queue is actively progressing.
  if (appStore.inFlightDownloads[0] !== props.book) return "queued";
  if (appStore.waitingDownload?.active) return "waiting";
  return appStore.downloadPhase ?? "transferring";
});

const isDisabled = computed(() => clicked.value && phase.value === "idle");
</script>

<template>
  <!-- Compact icon-only button (mobile cards) -->
  <button
    v-if="compact"
    :disabled="isDisabled"
    class="flex-shrink-0 w-9 h-9 flex items-center justify-center rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
    :class="phase === 'idle'
      ? 'bg-brand-400 hover:bg-brand-500 text-white'
      : phase === 'cleaning'
        ? 'bg-emerald-500 text-white'
        : 'bg-slate-200 dark:bg-slate-700 text-slate-500 dark:text-slate-400'"
    :title="phase === 'idle' ? 'Download' : phase === 'queued' ? 'Queued' : phase === 'waiting' ? 'Waiting for bot…' : phase === 'transferring' ? 'Downloading…' : 'Cleaning…'"
    @click="handleDownload">
    <Clock v-if="phase === 'queued'" :size="15" />
    <Sparkles v-else-if="phase === 'cleaning'" :size="15" class="animate-pulse" />
    <Loader v-else-if="phase !== 'idle'" :size="15" class="animate-spin" />
    <DownloadCloud v-else :size="15" />
  </button>

  <!-- Full button (desktop table) -->
  <button
    v-else
    :disabled="isDisabled"
    class="px-3 py-1 text-xs font-medium rounded-md border transition-colors whitespace-nowrap disabled:opacity-50 disabled:cursor-not-allowed"
    :class="phase === 'idle'
      ? 'border-slate-200 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700'
      : phase === 'cleaning'
        ? 'border-emerald-400 dark:border-emerald-600 text-emerald-700 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-950/30'
        : 'border-brand-300 dark:border-brand-700 text-brand-700 dark:text-brand-300 bg-brand-50 dark:bg-brand-950/30'"
    @click="handleDownload">
    <span v-if="phase === 'queued'" class="flex items-center gap-1.5">
      <Clock :size="11" /> Queued
    </span>
    <span v-else-if="phase === 'waiting'" class="flex items-center gap-1.5">
      <Loader :size="11" class="animate-spin" /> Waiting…
    </span>
    <span v-else-if="phase === 'transferring'" class="flex items-center gap-1.5">
      <Loader :size="11" class="animate-spin" /> Downloading…
    </span>
    <span v-else-if="phase === 'cleaning'" class="flex items-center gap-1.5">
      <Sparkles :size="11" class="animate-pulse" /> Cleaning…
    </span>
    <span v-else>Download</span>
  </button>
</template>
