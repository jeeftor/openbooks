<script setup lang="ts">
import { BellOff, X, Trash2 } from "lucide-vue-next";
import { useNotificationStore } from "../../stores/notifications";
import { NotificationType } from "../../types/messages";
import type { AppNotification } from "../../types/messages";

const notifStore = useNotificationStore();

function intentClass(appearance: NotificationType): string {
  switch (appearance) {
    case NotificationType.SUCCESS:
      return "border-l-green-500 bg-green-50 dark:bg-green-900/20";
    case NotificationType.WARNING:
      return "border-l-amber-500 bg-amber-50 dark:bg-amber-900/20";
    case NotificationType.DANGER:
      return "border-l-red-500 bg-red-50 dark:bg-red-900/20";
    default:
      return "border-l-brand-400 bg-brand-50 dark:bg-brand-900/20";
  }
}

function titleClass(appearance: NotificationType): string {
  switch (appearance) {
    case NotificationType.SUCCESS:
      return "text-green-800 dark:text-green-300";
    case NotificationType.WARNING:
      return "text-amber-800 dark:text-amber-300";
    case NotificationType.DANGER:
      return "text-red-800 dark:text-red-300";
    default:
      return "text-brand-800 dark:text-brand-300";
  }
}

function formatTime(ts: number) {
  return new Date(ts).toLocaleTimeString("en-US", { timeStyle: "short" });
}
</script>

<template>
  <!-- Backdrop -->
  <Transition
    enter-active-class="transition-opacity duration-200"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition-opacity duration-150"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0">
    <div
      v-if="notifStore.isOpen"
      class="fixed inset-0 bg-black/20 z-40"
      @click="notifStore.toggleDrawer()" />
  </Transition>

  <!-- Right-side panel -->
  <Transition
    enter-active-class="transition-transform duration-300 ease-out"
    enter-from-class="translate-x-full"
    enter-to-class="translate-x-0"
    leave-active-class="transition-transform duration-200 ease-in"
    leave-from-class="translate-x-0"
    leave-to-class="translate-x-full">
    <div
      v-if="notifStore.isOpen"
      class="fixed inset-y-0 right-0 z-50 w-80 flex flex-col bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-800 shadow-xl">
      <!-- Header -->
      <div
        class="flex-shrink-0 flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800">
        <span class="font-semibold text-slate-900 dark:text-slate-50"
          >Notifications</span
        >
        <div class="flex items-center gap-1">
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-red-400 disabled:opacity-30 transition-colors"
            title="Clear all"
            :disabled="!notifStore.notifications.length"
            @click="notifStore.clear()">
            <Trash2 :size="15" />
          </button>
          <button
            class="p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 transition-colors"
            @click="notifStore.toggleDrawer()">
            <X :size="15" />
          </button>
        </div>
      </div>

      <!-- Empty state -->
      <div
        v-if="!notifStore.notifications.length"
        class="flex-1 flex flex-col items-center justify-center gap-2 text-center px-4">
        <BellOff :size="28" class="text-slate-300 dark:text-slate-600" />
        <p class="text-xs text-slate-400 dark:text-slate-500">
          No notifications.
        </p>
      </div>

      <!-- Notification list -->
      <ul
        v-else
        class="flex-1 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800/60 px-3 py-2 space-y-1.5">
        <TransitionGroup
          enter-active-class="transition-all duration-200"
          enter-from-class="opacity-0 -translate-x-2"
          enter-to-class="opacity-100 translate-x-0"
          leave-active-class="transition-all duration-150"
          leave-from-class="opacity-100"
          leave-to-class="opacity-0"
          tag="div"
          class="space-y-1.5">
          <div
            v-for="notif in notifStore.notifications"
            :key="notif.timestamp"
            class="relative rounded-lg border-l-4 px-3 py-2.5 pr-8"
            :class="intentClass(notif.appearance)">
            <p
              class="text-xs font-medium leading-snug"
              :class="titleClass(notif.appearance)">
              {{ notif.title }}
            </p>
            <p
              v-if="notif.detail"
              class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              {{ notif.detail }}
            </p>
            <p class="text-[10px] text-slate-400 dark:text-slate-500 mt-1">
              {{ formatTime(notif.timestamp) }}
            </p>
            <button
              class="absolute top-2 right-2 p-0.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-slate-600 transition-colors"
              @click="notifStore.dismiss(notif.timestamp)">
              <X :size="12" />
            </button>
          </div>
        </TransitionGroup>
      </ul>
    </div>
  </Transition>
</template>
