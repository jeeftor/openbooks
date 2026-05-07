<script setup lang="ts">
import { useLocalStorage } from "@vueuse/core";
import {
  Bell,
  PlugZap,
  BadgeCheck,
  PanelLeftClose,
  Moon,
  Sun
} from "lucide-vue-next";
import { useDark, useToggle } from "@vueuse/core";
import { useAppStore } from "../../stores/app";
import { useNotificationStore } from "../../stores/notifications";
import { useVersion } from "../../composables/useApi";
import HistoryPanel from "../sidebar/HistoryPanel.vue";
import LibraryPanel from "../sidebar/LibraryPanel.vue";
import LogsPanel from "../sidebar/LogsPanel.vue";

type Tab = "history" | "books" | "logs";

const activeTab = useLocalStorage<Tab>("ob-sidebar-tab", "history");
const appStore = useAppStore();
const notifStore = useNotificationStore();
const version = useVersion();

const isDark = useDark({ storageKey: "ob-dark-mode" });
const toggleDark = useToggle(isDark);

const TABS: { id: Tab; label: string }[] = [
  { id: "history", label: "History" },
  { id: "books", label: "Downloads" },
  { id: "logs", label: "Logs" }
];

function selectTab(tab: Tab) {
  if (tab === "books" && activeTab.value === "books") {
    appStore.toggleLibrarySortMode();
    return;
  }
  activeTab.value = tab;
}
</script>

<template>
  <aside
    class="w-72 flex-shrink-0 flex flex-col border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden"
  >
    <!-- Header -->
    <div class="px-4 pt-4 pb-2 flex-shrink-0">
      <div class="flex items-center justify-between mb-1">
        <span class="font-bold text-lg tracking-tight text-slate-900 dark:text-slate-50">OpenBooks ABS</span>
        <button
          class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 disabled:opacity-30 transition-colors"
          :disabled="!appStore.isConnected"
          :title="appStore.isConnected ? 'View notifications' : 'Not connected'"
          @click="notifStore.toggleDrawer()"
        >
          <Bell :size="18" />
        </button>
      </div>
      <p class="text-xs text-slate-400 dark:text-slate-500">Prepare eBooks for Audiobookshelf</p>

      <!-- Tabs -->
      <div class="mt-3 flex rounded-lg bg-slate-100 dark:bg-slate-800/60 p-0.5 gap-0.5">
        <button
          v-for="tab in TABS"
          :key="tab.id"
          class="flex-1 text-xs py-1.5 rounded-md font-medium transition-all"
          :class="activeTab === tab.id
            ? 'bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-50 shadow-sm'
            : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200'"
          @click="selectTab(tab.id)"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <!-- Panel content -->
    <div class="flex-1 overflow-hidden">
      <HistoryPanel v-if="activeTab === 'history'" />
      <LibraryPanel v-else-if="activeTab === 'books'" />
      <LogsPanel v-else />
    </div>

    <!-- Footer -->
    <div class="flex-shrink-0 border-t border-slate-200 dark:border-slate-800 px-4 py-3">
      <div class="flex items-center justify-between gap-2">
        <!-- Connection / username -->
        <div class="flex items-center gap-2 min-w-0">
          <BadgeCheck
            v-if="appStore.username"
            :size="18"
            class="flex-shrink-0 text-brand-400"
          />
          <PlugZap
            v-else
            :size="18"
            class="flex-shrink-0 text-slate-500 dark:text-slate-600"
          />
          <span
            class="text-sm truncate"
            :class="appStore.username ? 'text-slate-700 dark:text-slate-200' : 'text-slate-400 dark:text-slate-600'"
          >
            {{ appStore.username ?? 'Not connected' }}
          </span>
        </div>

        <!-- Controls -->
        <div class="flex items-center gap-1 flex-shrink-0">
          <span v-if="version" class="text-xs text-slate-400 dark:text-slate-500 mr-1">{{ version }}</span>
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
            :title="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
            @click="toggleDark()"
          >
            <Moon v-if="!isDark" :size="16" />
            <Sun v-else :size="16" />
          </button>
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 transition-colors"
            title="Collapse sidebar"
            @click="appStore.toggleSidebar()"
          >
            <PanelLeftClose :size="16" />
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>
