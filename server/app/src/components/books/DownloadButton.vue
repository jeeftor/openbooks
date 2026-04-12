<script setup lang="ts">
import { ref } from "vue";
import { DownloadCloud, Loader } from "lucide-vue-next";
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
const clicked = ref(false);

const isInFlight = () => appStore.isDownloading(props.book);

function handleDownload() {
  if (clicked.value) return;
  clicked.value = true;
  appStore.addInFlightDownload(props.book);
  sendMessage({
    type: MessageType.DOWNLOAD,
    payload: { book: props.book, author: props.author, title: props.title }
  });
}
</script>

<template>
  <!-- Compact icon-only button (mobile cards) -->
  <button
    v-if="compact"
    :disabled="clicked && !isInFlight()"
    class="flex-shrink-0 w-9 h-9 flex items-center justify-center rounded-lg bg-brand-400 hover:bg-brand-500 text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
    @click="handleDownload">
    <Loader v-if="isInFlight()" :size="16" class="animate-spin" />
    <DownloadCloud v-else :size="16" />
  </button>

  <!-- Full button (desktop table) -->
  <button
    v-else
    :disabled="clicked && !isInFlight()"
    class="px-3 py-1 text-xs font-medium rounded-md border border-slate-200 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
    @click="handleDownload">
    <span v-if="isInFlight()" class="flex items-center gap-1.5">
      <Loader :size="12" class="animate-spin" /> Downloading…
    </span>
    <span v-else>Download</span>
  </button>
</template>
