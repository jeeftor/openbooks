<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { useAppStore } from "../stores/app";

const isMobile = useMediaQuery("(max-width: 767px)");

const appStore = useAppStore();
const waiting = computed(() => appStore.waitingDownload);
const phase = computed(() => appStore.downloadPhase);

// Remember the last title/bot so we can show them during the "transferring" phase
// after waitingDownload has been cleared.
const lastTitle = ref<string | undefined>(undefined);
const lastBot = ref<string | undefined>(undefined);

watch(waiting, (w) => {
  if (w?.active) {
    lastTitle.value = w.bookTitle;
    lastBot.value = w.bot;
  }
}, { immediate: true });

// Countdown timer (only relevant while waiting)
const elapsed = ref(0);
let timer: ReturnType<typeof setInterval> | null = null;

watch(waiting, (w) => {
  elapsed.value = 0;
  if (timer) { clearInterval(timer); timer = null; }
  if (w?.active) {
    timer = setInterval(() => { elapsed.value++; }, 1000);
  }
}, { immediate: true });

onUnmounted(() => { if (timer) clearInterval(timer); });

const timeoutSecs = computed(() => waiting.value?.timeoutSecs ?? 300);
const remaining = computed(() => Math.max(0, timeoutSecs.value - elapsed.value));
const progress = computed(() => (elapsed.value / timeoutSecs.value) * 100);

const isVisible = computed(() => waiting.value?.active || phase.value === "transferring");
const isTransferring = computed(() => phase.value === "transferring");

function formatTime(secs: number) {
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}
</script>

<template>
  <Transition name="banner">
    <div
      v-if="isVisible"
      class="fixed left-1/2 -translate-x-1/2 z-40 w-[min(420px,calc(100vw-2rem))]
             bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl overflow-hidden"
      :style="isMobile
        ? { bottom: 'calc(3.5rem + env(safe-area-inset-bottom) + 0.5rem)' }
        : { bottom: '1rem' }"
    >
      <!-- Progress bar along the top -->
      <div class="h-0.5 bg-slate-700">
        <div
          v-if="!isTransferring"
          class="h-full bg-brand-400 transition-all duration-1000 ease-linear"
          :style="{ width: progress + '%' }"
        />
        <!-- Indeterminate shimmer while transferring -->
        <div v-else class="h-full w-full overflow-hidden relative">
          <div class="absolute inset-y-0 w-1/3 bg-brand-400 shimmer-bar" />
        </div>
      </div>

      <div class="px-4 py-3 flex items-center gap-3">
        <!-- Waiting: countdown ring / Transferring: pulsing spinner -->
        <div class="flex-shrink-0 relative w-9 h-9">
          <template v-if="!isTransferring">
            <svg class="w-9 h-9 -rotate-90" viewBox="0 0 36 36">
              <circle
                cx="18" cy="18" r="15"
                fill="none" stroke="rgb(51 65 85)" stroke-width="3"
              />
              <circle
                cx="18" cy="18" r="15"
                fill="none" stroke="rgb(96 165 250)" stroke-width="3"
                stroke-linecap="round"
                stroke-dasharray="94.2"
                :stroke-dashoffset="94.2 * (1 - (remaining / timeoutSecs))"
                class="transition-all duration-1000 ease-linear"
              />
            </svg>
            <span class="absolute inset-0 flex items-center justify-center text-[9px] font-mono text-slate-400 tabular-nums">
              {{ formatTime(remaining) }}
            </span>
          </template>
          <template v-else>
            <svg class="w-9 h-9 animate-spin" viewBox="0 0 36 36">
              <circle
                cx="18" cy="18" r="15"
                fill="none" stroke="rgb(51 65 85)" stroke-width="3"
              />
              <path
                d="M 18 3 A 15 15 0 0 1 33 18"
                fill="none" stroke="rgb(96 165 250)" stroke-width="3"
                stroke-linecap="round"
              />
            </svg>
          </template>
        </div>

        <!-- Text -->
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-slate-100 truncate">
            <template v-if="isTransferring">
              Transferring<span v-if="lastBot"> from <span class="text-brand-400">{{ lastBot }}</span></span>…
            </template>
            <template v-else>
              Waiting for <span class="text-brand-400">{{ waiting!.bot }}</span>
            </template>
          </p>
          <p class="text-xs text-slate-400 truncate mt-0.5">
            {{ isTransferring ? lastTitle : waiting!.bookTitle }}
          </p>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.banner-enter-active, .banner-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.banner-enter-from, .banner-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(12px);
}

.shimmer-bar {
  animation: shimmer 1.4s ease-in-out infinite;
}

@keyframes shimmer {
  0%   { left: -33%; }
  100% { left: 100%; }
}
</style>
