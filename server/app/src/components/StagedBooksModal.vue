<script setup lang="ts">
import { ref, watch } from "vue";
import { BookMarked } from "lucide-vue-next";
import { useAppStore } from "../stores/app";
import { MessageType } from "../types/messages";
import { sendMessage } from "../composables/useWebSocket";

const appStore = useAppStore();
const dismissed = ref(false);

// Reset dismissed state whenever staged count increases (new books arrived).
watch(
  () => appStore.stagedBooksCount,
  (count, prev) => {
    if (count > (prev ?? 0)) dismissed.value = false;
  }
);

function processNow() {
  dismissed.value = true;
  sendMessage({ type: MessageType.PROCESS_STAGED_BOOKS });
}
</script>

<template>
  <Transition name="modal">
    <div
      v-if="appStore.stagedBooksCount > 0 && !appStore.pendingStagedBook && !dismissed"
      class="fixed inset-0 z-50 flex items-end justify-center p-4 sm:items-center bg-black/60 backdrop-blur-sm"
      @click.self="dismissed = true"
    >
      <div class="w-full max-w-sm bg-white dark:bg-slate-900 rounded-2xl shadow-2xl overflow-hidden">
        <!-- Header -->
        <div class="px-6 pt-6 pb-4 flex items-center gap-3">
          <div class="flex-shrink-0 w-10 h-10 rounded-xl bg-amber-100 dark:bg-amber-900/40 flex items-center justify-center">
            <BookMarked :size="20" class="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-slate-900 dark:text-slate-50">
              Staged Books Ready
            </h2>
            <p class="text-sm text-slate-500 dark:text-slate-400">
              {{ appStore.stagedBooksCount === 1
                ? '1 book is waiting to be organized.'
                : `${appStore.stagedBooksCount} books are waiting to be organized.` }}
            </p>
          </div>
        </div>

        <!-- Footer -->
        <div class="px-6 pb-6 flex items-center justify-end gap-3">
          <button
            @click="dismissed = true"
            class="px-4 py-2 text-sm rounded-lg border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
          >
            Later
          </button>
          <button
            @click="processNow"
            class="px-5 py-2 text-sm font-medium rounded-lg bg-amber-500 hover:bg-amber-600 text-white transition-colors"
          >
            Process Now
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
