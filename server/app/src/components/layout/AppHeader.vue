<script setup lang="ts">
import { BookMarked, Bell, Moon, Sun, BadgeCheck, PlugZap } from "lucide-vue-next";
import { useDark, useToggle } from "@vueuse/core";
import { useAppStore } from "../../stores/app";
import { useNotificationStore } from "../../stores/notifications";
import { MessageType } from "../../types/messages";
import { sendMessage } from "../../composables/useWebSocket";
import { useVersion } from "../../composables/useApi";
import VersionLink from "./VersionLink.vue";

const appStore = useAppStore();
const notifStore = useNotificationStore();
const version = useVersion();
const isDark = useDark({ storageKey: "ob-dark-mode" });
const toggleDark = useToggle(isDark);
</script>

<template>
  <header class="flex-shrink-0 flex items-center gap-2 px-4 h-11 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 z-20 select-none">
    <!-- Brand -->
    <span class="font-bold text-sm tracking-tight text-slate-800 dark:text-slate-100 flex-shrink-0">
      OpenBooks <span class="text-brand-400">ABS</span>
    </span>

    <div class="flex-1" />

    <!-- Staged books badge -->
    <button
      v-if="appStore.stagedBooksCount > 0"
      class="relative p-1.5 rounded-lg hover:bg-amber-50 dark:hover:bg-amber-900/20 text-amber-600 dark:text-amber-400 transition-colors"
      :title="`${appStore.stagedBooksCount} staged book${appStore.stagedBooksCount === 1 ? '' : 's'} — click to review`"
      @click="sendMessage({ type: MessageType.GET_STAGED_LIST })">
      <BookMarked :size="15" />
      <span class="absolute -top-0.5 -right-0.5 min-w-[13px] h-3 px-0.5 text-[8px] font-bold leading-none rounded-full bg-amber-500 text-white flex items-center justify-center">
        {{ appStore.stagedBooksCount }}
      </span>
    </button>

    <!-- Notifications -->
    <button
      class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 disabled:opacity-30 transition-colors"
      :disabled="!appStore.isConnected"
      :title="appStore.isConnected ? 'Notifications' : 'Not connected'"
      @click="notifStore.toggleDrawer()">
      <Bell :size="15" />
    </button>

    <VersionLink v-if="version" :version="version" />

    <!-- Dark mode -->
    <button
      class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
      :title="isDark ? 'Light mode' : 'Dark mode'"
      @click="toggleDark()">
      <Moon v-if="!isDark" :size="15" />
      <Sun v-else :size="15" />
    </button>

    <!-- Connection / username -->
    <div class="flex items-center gap-1.5 text-xs pl-1 border-l border-slate-200 dark:border-slate-700"
      :class="appStore.username ? 'text-slate-600 dark:text-slate-300' : 'text-slate-400 dark:text-slate-600'">
      <BadgeCheck v-if="appStore.username" :size="14" class="flex-shrink-0 text-brand-400" />
      <PlugZap v-else :size="14" class="flex-shrink-0" />
      <span class="hidden sm:block truncate max-w-[100px] text-[11px]">
        {{ appStore.username ?? (appStore.isConnecting ? 'Connecting…' : 'Disconnected') }}
      </span>
    </div>
  </header>
</template>
