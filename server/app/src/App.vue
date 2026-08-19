<script setup lang="ts">
import { ref } from "vue";
import { useDark } from "@vueuse/core";
import { Toaster } from "vue-sonner";
import { useWebSocket } from "./composables/useWebSocket";
import AppHeader from "./components/layout/AppHeader.vue";
import SearchTabBar from "./components/layout/SearchTabBar.vue";
import FloatingTaskPanel from "./components/layout/FloatingTaskPanel.vue";
import SearchView from "./pages/SearchView.vue";
import LibraryPanel from "./components/sidebar/LibraryPanel.vue";
import NotificationDrawer from "./components/notifications/NotificationDrawer.vue";
import RenameModal from "./components/RenameModal.vue";
import StagedBooksModal from "./components/StagedBooksModal.vue";
import StagedBooksListModal from "./components/StagedBooksListModal.vue";
import StagedRenameModal from "./components/StagedRenameModal.vue";
import DownloadWaitingBanner from "./components/DownloadWaitingBanner.vue";
import MobileNav from "./components/layout/MobileNav.vue";

useDark({ storageKey: "ob-dark-mode", initialValue: "dark" });
useWebSocket();

const showLibrary = ref(false);
</script>

<template>
  <div
    class="h-dvh flex flex-col overflow-hidden bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-50"
    style="padding-top: env(safe-area-inset-top)">

    <AppHeader />
    <SearchTabBar :library-open="showLibrary" @toggle-library="showLibrary = !showLibrary" />

    <main class="flex-1 overflow-hidden relative flex flex-col">
      <!-- Library overlay (slides in from right) -->
      <Transition name="library-slide">
        <div
          v-if="showLibrary"
          class="absolute inset-0 z-10 bg-white dark:bg-slate-900 flex flex-col border-l border-slate-200 dark:border-slate-800">
          <LibraryPanel />
        </div>
      </Transition>

      <SearchView />
    </main>

    <!-- Spacer so content doesn't disappear under the fixed MobileNav bar (mobile only) -->
    <div class="h-14 flex-shrink-0 md:hidden" />

    <FloatingTaskPanel />

    <!-- Mobile bottom navigation (hidden on md+ breakpoint) -->
    <div class="md:hidden">
      <MobileNav />
    </div>

    <!-- Global overlays (order matters for z-index stacking) -->
    <NotificationDrawer />
    <RenameModal />
    <StagedBooksModal />
    <StagedBooksListModal />
    <StagedRenameModal />
    <DownloadWaitingBanner />
    <Toaster rich-colors position="top-center" />
  </div>
</template>

<style>
.library-slide-enter-active,
.library-slide-leave-active {
  transition: transform 0.22s ease, opacity 0.22s ease;
}
.library-slide-enter-from,
.library-slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
