<script setup lang="ts">
import { ref, watch } from "vue";
import {
  History,
  BookMarked,
  Activity,
  X,
  Bell,
  Moon,
  Sun,
  BadgeCheck,
  PlugZap,
  Terminal
} from "lucide-vue-next";
import { useDark, useToggle } from "@vueuse/core";
import { useAppStore } from "../../stores/app";
import { useNotificationStore } from "../../stores/notifications";
import { useTaskStore } from "../../stores/tasks";
import { useVersion } from "../../composables/useApi";
import { sendMessage } from "../../composables/useWebSocket";
import { MessageType } from "../../types/messages";
import HistoryPanel from "../sidebar/HistoryPanel.vue";
import LibraryPanel from "../sidebar/LibraryPanel.vue";
import ActivityPanel from "../sidebar/ActivityPanel.vue";
import IrcPanel from "../sidebar/IrcPanel.vue";
import VersionLink from "./VersionLink.vue";

type Tab = "history" | "books" | "activity" | "irc";

const activeTab = ref<Tab | null>(null);
const appStore = useAppStore();
const notifStore = useNotificationStore();
const taskStore = useTaskStore();
const version = useVersion();

const isDark = useDark({ storageKey: "ob-dark-mode" });
const toggleDark = useToggle(isDark);

const TABS = [
  { id: "history" as Tab, label: "History", icon: History },
  { id: "books" as Tab, label: "Downloads", icon: BookMarked },
  { id: "activity" as Tab, label: "Activity", icon: Activity },
  { id: "irc" as Tab, label: "IRC", icon: Terminal }
];

function selectTab(tab: Tab) {
  if (tab === "books" && activeTab.value === "books") {
    appStore.toggleLibrarySortMode();
    return;
  }
  // Close the desktop IRC overlay when opening the mobile IRC tab.
  if (tab === "irc" && appStore.showIrcPanel) {
    appStore.setIrcPanelOpen(false);
  }
  activeTab.value = activeTab.value === tab ? null : tab;
}

// Close the mobile IRC tab when the desktop overlay opens.
watch(() => appStore.showIrcPanel, (open) => {
  if (open && activeTab.value === "irc") {
    activeTab.value = null;
  }
});

// Subscribe/unsubscribe to IRC line broadcasts when the mobile IRC tab opens/closes.
watch(activeTab, (tab, oldTab) => {
  if (tab === "irc") {
    sendMessage({ type: MessageType.IRC_SUBSCRIBE, payload: { subscribed: true } });
  } else if (oldTab === "irc") {
    sendMessage({ type: MessageType.IRC_SUBSCRIBE, payload: { subscribed: false } });
    appStore.clearIrcMessages();
  }
});
</script>

<template>
  <!-- Bottom sheet panel -->
  <Transition
    enter-active-class="transition-transform duration-300 ease-out"
    enter-from-class="translate-y-full"
    enter-to-class="translate-y-0"
    leave-active-class="transition-transform duration-200 ease-in"
    leave-from-class="translate-y-0"
    leave-to-class="translate-y-full">
    <div
      v-if="activeTab"
      class="fixed inset-x-0 bottom-14 z-40 bg-white dark:bg-slate-900 border-t border-slate-200 dark:border-slate-800 rounded-t-2xl shadow-2xl flex flex-col"
      style="max-height: 65dvh">
      <!-- Sheet header -->
      <div
        class="flex items-center justify-between px-4 pt-3 pb-2 flex-shrink-0">
        <div class="flex items-center gap-2 min-w-0 flex-1">
          <component
            :is="appStore.username ? BadgeCheck : PlugZap"
            :size="16"
            class="flex-shrink-0"
            :class="appStore.username ? 'text-brand-400' : 'text-slate-500 dark:text-slate-600'"
          />
          <span class="text-sm font-medium text-slate-700 dark:text-slate-200 truncate">
            {{ appStore.username ?? "Not connected" }}
          </span>
        </div>
        <div class="flex items-center gap-1 flex-shrink-0">
          <VersionLink v-if="version" :version="version" />
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
            @click="notifStore.toggleDrawer()">
            <Bell :size="16" />
          </button>
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
            @click="toggleDark()">
            <Moon v-if="!isDark" :size="16" />
            <Sun v-else :size="16" />
          </button>
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
            @click="activeTab = null">
            <X :size="16" />
          </button>
        </div>
      </div>

      <!-- Panel content -->
      <div class="flex-1 overflow-hidden">
        <HistoryPanel v-if="activeTab === 'history'" />
        <LibraryPanel v-else-if="activeTab === 'books'" />
        <ActivityPanel v-else-if="activeTab === 'activity'" />
        <IrcPanel v-else-if="activeTab === 'irc'" />
      </div>
    </div>
  </Transition>

  <!-- Fixed bottom nav bar -->
  <nav
    class="fixed bottom-0 inset-x-0 z-50 bg-white dark:bg-slate-900 border-t border-slate-200 dark:border-slate-800
           flex items-stretch h-14 safe-area-inset-bottom">
    <button
      v-for="tab in TABS"
      :key="tab.id"
      class="flex-1 flex flex-col items-center justify-center gap-0.5 text-xs font-medium transition-colors"
      :class="activeTab === tab.id
        ? 'text-brand-400 dark:text-brand-300'
        : 'text-slate-500 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'"
      @click="selectTab(tab.id)">
      <div class="relative">
        <component :is="tab.icon" :size="20" />
        <span
          v-if="tab.id === 'activity' && taskStore.activeCount > 0"
          class="absolute -top-1 -right-1 min-w-[14px] h-3.5 px-0.5 text-[9px] font-bold leading-none rounded-full bg-blue-500 text-white flex items-center justify-center">
          {{ taskStore.activeCount }}
        </span>
      </div>
      <span>{{ tab.label }}</span>
    </button>
  </nav>
</template>
