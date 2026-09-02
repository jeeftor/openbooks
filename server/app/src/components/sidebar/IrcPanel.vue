<script setup lang="ts">
import { ref, nextTick, watch, onMounted, computed } from "vue";
import { Terminal, Send, Trash2, ArrowDown, MessageSquare, FileText } from "lucide-vue-next";
import { useAppStore } from "../../stores/app";
import { sendMessage } from "../../composables/useWebSocket";
import { MessageType } from "../../types/messages";
import { parseIrcLine, nickHue, formatIrcTime, type ParsedIrcLine } from "../../utils/ircParser";
import EmptyPanel from "../shared/EmptyPanel.vue";
import PanelHeader from "../shared/PanelHeader.vue";

const appStore = useAppStore();

const input = ref("");
const scrollEl = ref<HTMLElement | null>(null);
const autoScroll = ref(true);

// Filter mode: "chat" shows only channel messages, notices, joins, quits, DCC.
// "all" shows everything including numerics, MOTD, NAMES lists.
type FilterMode = "chat" | "all";
const filterMode = ref<FilterMode>("chat");

// Parse all stored raw lines into structured objects.
const parsedMessages = computed<ParsedIrcLine[]>(() =>
  appStore.ircMessages.map((m) => parseIrcLine(m.line))
);

// Apply filter — "chat" hides noisy server numerics.
const visibleMessages = computed<ParsedIrcLine[]>(() => {
  if (filterMode.value === "all") return parsedMessages.value;
  return parsedMessages.value.filter((m) => {
    switch (m.type) {
      case "privmsg":
      case "notice":
      case "dcc":
      case "join":
      case "quit":
      case "part":
      case "nick":
      case "kick":
      case "error":
      case "welcome":
      case "topic":
      case "ban":        // 474 — always show ban errors
      case "chan_error": // 471/473/475/433 — always show channel errors
        return true;
      // Hide MOTD, NAMES, numerics, ping in chat mode
      case "motd":
      case "motd_end":
      case "names":
      case "names_end":
      case "numeric":
      case "ping":
      case "mode":
      case "raw":
        return false;
      default:
        return false;
    }
  });
});

const messageCount = computed(() => appStore.ircMessages.length);

function send() {
  const msg = input.value.trim();
  if (!msg) return;
  sendMessage({ type: MessageType.IRC_SEND, payload: { message: msg } });
  input.value = "";
}

function onScroll() {
  const el = scrollEl.value;
  if (!el) return;
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

watch(
  () => visibleMessages.value.length,
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

const showJumpButton = computed(() => {
  if (autoScroll.value) return false;
  const el = scrollEl.value;
  if (!el) return false;
  return el.scrollHeight - el.scrollTop - el.clientHeight > 40;
});

// Nick color style — uses HSL with consistent hue per nick.
function nickStyle(nick: string) {
  if (!nick) return {};
  return { color: `hsl(${nickHue(nick)}, 65%, 55%)` };
}

onMounted(() => scrollToBottom());
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header -->
    <PanelHeader>
      {{ messageCount }} lines
      <template #actions>
        <!-- Filter toggle -->
        <div class="flex items-center gap-0.5 mr-1">
          <button
            class="px-1.5 py-0.5 rounded text-[10px] font-medium transition-colors"
            :class="filterMode === 'chat'
              ? 'bg-brand-500 text-white'
              : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-300'"
            title="Show only chat messages, notices, joins, quits"
            @click="filterMode = 'chat'">
            <MessageSquare :size="11" />
          </button>
          <button
            class="px-1.5 py-0.5 rounded text-[10px] font-medium transition-colors"
            :class="filterMode === 'all'
              ? 'bg-brand-500 text-white'
              : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-300'"
            title="Show all IRC traffic including server numerics"
            @click="filterMode = 'all'">
            <FileText :size="11" />
          </button>
        </div>
        <button
          class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
          title="Clear"
          @click="appStore.clearIrcMessages()">
          <Trash2 :size="13" />
        </button>
      </template>
    </PanelHeader>

    <!-- Empty -->
    <EmptyPanel v-if="!messageCount" :icon="Terminal">
      No IRC traffic yet. Connect and the channel feed will appear here.
    </EmptyPanel>

    <!-- Empty after filter -->
    <EmptyPanel v-else-if="!visibleMessages.length" :icon="MessageSquare">
      No chat messages in this filter. Switch to "all" to see server traffic.
    </EmptyPanel>

    <!-- Messages -->
    <div v-else class="flex-1 min-h-0 relative overflow-hidden">
      <div
        ref="scrollEl"
        class="h-full overflow-y-auto font-mono text-[11px] leading-relaxed px-2 py-1"
        @scroll="onScroll">

        <div
          v-for="msg in visibleMessages"
          :key="msg.timestamp + msg.raw"
          class="grid grid-cols-[4.75rem_minmax(0,1fr)] items-start gap-x-2 px-1.5 py-0.5 rounded hover:bg-slate-50 dark:hover:bg-slate-800/40">
          <span class="text-right text-slate-400 dark:text-slate-600 select-none tabular-nums">{{ formatIrcTime(msg.timestamp) }}</span>
          <div class="min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]">

          <!-- PRIVMSG: <nick> content -->
          <template v-if="msg.type === 'privmsg'">
            <span class="text-slate-400 dark:text-slate-600 select-none">&lt;</span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-600 select-none">&gt; </span>
            <span class="text-slate-700 dark:text-slate-300">{{ msg.content }}</span>
          </template>

          <!-- NOTICE: -nick- content (highlighted for search responses) -->
          <template v-else-if="msg.type === 'notice'">
            <span class="text-slate-400 dark:text-slate-600 select-none">-</span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-600 select-none">- </span>
            <span
              class="text-slate-600 dark:text-slate-400"
              :class="{
                'text-amber-600 dark:text-amber-400': msg.content.includes('accepted'),
                'text-green-600 dark:text-green-400': msg.content.includes('matches'),
                'text-red-500 dark:text-red-400': msg.content.includes('Sorry') || msg.content.includes('try another server'),
              }">
              {{ msg.content }}
            </span>
          </template>

          <!-- DCC SEND: highlighted -->
          <template v-else-if="msg.type === 'dcc'">
            <span class="text-purple-600 dark:text-purple-400 font-semibold">DCC</span>
            <span class="text-slate-400 dark:text-slate-600"> from </span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-600">: </span>
            <span class="text-purple-600 dark:text-purple-400">{{ msg.content }}</span>
          </template>

          <!-- JOIN -->
          <template v-else-if="msg.type === 'join'">
            <span class="text-green-600 dark:text-green-500">→</span>
            <span class="text-slate-400 dark:text-slate-500"> </span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> joined {{ msg.target || '#ebooks' }}</span>
          </template>

          <!-- QUIT -->
          <template v-else-if="msg.type === 'quit'">
            <span class="text-slate-400 dark:text-slate-500">←</span>
            <span class="text-slate-400 dark:text-slate-500"> </span>
            <span class="font-semibold text-slate-500 dark:text-slate-500">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> quit</span>
            <span v-if="msg.content" class="text-slate-400 dark:text-slate-600"> ({{ msg.content }})</span>
          </template>

          <!-- PART -->
          <template v-else-if="msg.type === 'part'">
            <span class="text-slate-400 dark:text-slate-500">←</span>
            <span class="text-slate-400 dark:text-slate-500"> </span>
            <span class="font-semibold text-slate-500 dark:text-slate-500">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> left {{ msg.target }}</span>
            <span v-if="msg.content" class="text-slate-400 dark:text-slate-600"> ({{ msg.content }})</span>
          </template>

          <!-- NICK change -->
          <template v-else-if="msg.type === 'nick'">
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> is now known as </span>
            <span class="font-semibold" :style="nickStyle(msg.content)">{{ msg.content }}</span>
          </template>

          <!-- KICK -->
          <template v-else-if="msg.type === 'kick'">
            <span class="text-red-500 dark:text-red-400 font-semibold">!</span>
            <span class="text-slate-400 dark:text-slate-500"> </span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> kicked from {{ msg.target }}</span>
            <span v-if="msg.content" class="text-slate-400 dark:text-slate-600"> ({{ msg.content }})</span>
          </template>

          <!-- Welcome (001) -->
          <template v-else-if="msg.type === 'welcome'">
            <span class="text-green-600 dark:text-green-400 font-semibold">✓ Connected:</span>
            <span class="text-slate-500 dark:text-slate-400"> {{ msg.content }}</span>
          </template>

          <!-- Topic (332) -->
          <template v-else-if="msg.type === 'topic'">
            <span class="text-brand-500 dark:text-brand-400 font-semibold">Topic:</span>
            <span class="text-slate-500 dark:text-slate-400"> {{ msg.content }}</span>
          </template>

          <!-- NAMES (353) — collapsed -->
          <template v-else-if="msg.type === 'names'">
            <span class="text-slate-400 dark:text-slate-600 italic">NAMES: {{ msg.content.split(' ').filter(Boolean).length }} users</span>
          </template>

          <!-- NAMES end (366) -->
          <template v-else-if="msg.type === 'names_end'">
            <span class="text-slate-400 dark:text-slate-600 italic">End of NAMES list.</span>
          </template>

          <!-- MOTD -->
          <template v-else-if="msg.type === 'motd'">
            <span class="text-slate-400 dark:text-slate-600 italic">{{ msg.content }}</span>
          </template>

          <!-- MOTD end -->
          <template v-else-if="msg.type === 'motd_end'">
            <span class="text-slate-400 dark:text-slate-600 italic">End of MOTD.</span>
          </template>

          <!-- ERROR -->
          <template v-else-if="msg.type === 'error'">
            <span class="text-red-500 dark:text-red-400 font-semibold">ERROR:</span>
            <span class="text-red-500 dark:text-red-400"> {{ msg.content }}</span>
          </template>

          <!-- BAN (474) -->
          <template v-else-if="msg.type === 'ban'">
            <span class="text-red-500 dark:text-red-400 font-semibold">BANNED:</span>
            <span class="text-red-500 dark:text-red-400"> Cannot join #ebooks — you are banned from this channel.</span>
          </template>

          <!-- Channel error (471/473/475/433) -->
          <template v-else-if="msg.type === 'chan_error'">
            <span class="text-amber-600 dark:text-amber-400 font-semibold">{{ msg.numeric }}:</span>
            <span class="text-amber-600 dark:text-amber-400"> {{ msg.content }}</span>
          </template>

          <!-- PING -->
          <template v-else-if="msg.type === 'ping'">
            <span class="text-slate-400 dark:text-slate-600 italic">PING {{ msg.content }}</span>
          </template>

          <!-- MODE -->
          <template v-else-if="msg.type === 'mode'">
            <span class="text-slate-400 dark:text-slate-500">mode: </span>
            <span class="font-semibold" :style="nickStyle(msg.nick)">{{ msg.nick }}</span>
            <span class="text-slate-400 dark:text-slate-500"> sets {{ msg.content }}</span>
          </template>

          <!-- Generic numeric -->
          <template v-else-if="msg.type === 'numeric'">
            <span class="text-slate-400 dark:text-slate-600">{{ msg.numeric }} {{ msg.target }} {{ msg.content }}</span>
          </template>

          <!-- Raw fallback -->
          <template v-else>
            <span class="text-slate-400 dark:text-slate-600">{{ msg.raw }}</span>
          </template>
          </div>
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
