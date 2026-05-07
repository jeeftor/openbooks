<script setup lang="ts">
import { useDark, useMediaQuery } from "@vueuse/core";
import { Toaster } from "vue-sonner";
import { useWebSocket } from "./composables/useWebSocket";
import { useAppStore } from "./stores/app";
import Sidebar from "./components/layout/Sidebar.vue";
import MobileNav from "./components/layout/MobileNav.vue";
import SearchView from "./pages/SearchView.vue";
import NotificationDrawer from "./components/notifications/NotificationDrawer.vue";
import RenameModal from "./components/RenameModal.vue";

useDark({ storageKey: 'ob-dark-mode', initialValue: 'dark' });
useWebSocket();

const appStore = useAppStore();
const isMobile = useMediaQuery("(max-width: 767px)");
</script>

<template>
  <div class="h-dvh flex overflow-hidden bg-slate-100 dark:bg-slate-950 text-slate-900 dark:text-slate-50">
    <Sidebar v-if="!isMobile && appStore.isSidebarOpen" />

    <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <SearchView />
    </main>

    <MobileNav v-if="isMobile" />
    <NotificationDrawer />
    <RenameModal />
    <Toaster rich-colors position="top-center" />
  </div>
</template>
