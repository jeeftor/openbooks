<script setup lang="ts">
import { ref, nextTick, watch, onMounted, onUnmounted, computed } from "vue";
import { Terminal, Send, Trash2, ArrowDown } from "lucide-vue-next";
import { useAppStore } from "../../stores/app";
import { sendMessage } from "../../composables/useWebSocket";
import { MessageType } from "../../types/messages";
import EmptyPanel from "../shared/EmptyPanel.vue";
import PanelHeader from "../shared/PanelHeader.vue";

const appStore = useAppStore();

const input = ref("");
const scrollEl = ref<HTMLElement | null>(null);
const autoScroll = ref(true);

const messages = computed(() => appStore.ircMessages);

function send() {
  const msg = input.value.trim();
  if (!msg) return;
  sendMessage({ type: MessageType.IRC_SEND, payload: { message: msg } });
  input.value = "";
}

function onScroll() {
  const el = scrollEl.value;
  if (!el) return;
  // If user scrolled up more than ~40px from the bottom, pause auto-scroll.
  autoScroll.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
}

function jumpToBottom() {
  autoScroll.value = true;
  scrollToBottom();
}

function scrollToBottom() {
  const el = scrollEl.value;
  if (el) el.scrollTop = el.scrollHeight;
}

// Auto-scroll to bottom when new messages arrive (unless user scrolled up).
watch(
  () => messages.value.length,
  async () => {
    if (autoScroll.value) {
      await nextTick();
      scrollToBottom();
    }
  }
);

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    send();
  }
}

// Show a "new messages" indicator when paused and messages arrive.
const showJumpButton = computed(() => {
  if (autoScroll.value) return false;
  const el = scrollEl.value;
  if (!el) return false;
  return el.scrollHeight - el.scrollTop - el.clientHeight > 40;
});

onMounted(() => scrollToBottom());
onUnmounted(() => {});
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <PanelHeader>
      {{ messages.length }} lines
      <template #actions>
        <button
          class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
          title="Clear"
          @click="appStore.clearIrcMessages()">
          <Trash2 :size="13" />
        </button>
      </template>
    </PanelHeader>

    <!-- Empty -->
    <EmptyPanel v-if="!messages.length" :icon="Terminal">
      No IRC traffic yet. Connect and the raw channel feed will appear here.
    </EmptyPanel>

    <!-- Messages -->
    <div v-else class="flex-1 min-h-0 relative overflow-hidden">
      <div
        ref="scrollEl"
        class="h-full overflow-y-auto font-mono text-[10px] leading-relaxed px-2 py-1"
        @scroll="onScroll">
        <div
          v-for="(msg, i) in messages"
          :key="i"
          class="px-1.5 py-0.5 rounded hover:bg-slate-50 dark:hover:bg-slate-800/40 break-all whitespace-pre-wrap text-slate-600 dark:text-slate-400">
          {{ msg.line }}
        </div>
      </div>

      <!-- Jump to bottom button -->
      <Transition name="jump-fade">
        <button
          v-if="showJumpButton"
          class="absolute bottom-2 left-1/2 -translate-x-1/2 flex items-center gap-1 px-2 py-1 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 shadow-md text-[10px] text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
          @click="jumpToBottom">
          <ArrowDown :size="11" />
          New lines
        </button>
      </Transition>
    </div>

    <!-- Input -->
    <div class="flex-shrink-0 flex items-center gap-1.5 px-2 py-2 border-t border-slate-100 dark:border-slate-800">
      <input
        v-model="input"
        type="text"
        :disabled="!appStore.isConnected"
        :placeholder="appStore.isConnected ? 'Send to #ebooks…  (e.g. @search tolkien)' : 'Not connected'"
        class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-[11px] text-slate-700 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600 focus:outline-none focus:ring-1 focus:ring-brand-400 focus:border-brand-400 disabled:opacity-40"
        @keydown="onKeydown" />
      <button
        class="flex-shrink-0 p-1.5 rounded-lg bg-brand-500 text-white hover:bg-brand-600 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        :disabled="!input.trim() || !appStore.isConnected"
        title="Send"
        @click="send">
        <Send :size="13" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.jump-fade-enter-active,
.jump-fade-leave-active { transition: opacity 0.15s ease; }
.jump-fade-enter-from,
.jump-fade-leave-to { opacity: 0; }
</style>
