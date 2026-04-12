import { defineStore } from "pinia";
import { ref } from "vue";
import type { AppNotification } from "../types/messages";

export const useNotificationStore = defineStore("notifications", () => {
  const isOpen = ref(false);
  const notifications = ref<AppNotification[]>([]);

  function add(notification: AppNotification) {
    notifications.value = [notification, ...notifications.value];
  }

  function dismiss(timestamp: number) {
    notifications.value = notifications.value.filter(
      (n) => n.timestamp !== timestamp
    );
  }

  function clear() {
    notifications.value = [];
  }

  function toggleDrawer() {
    isOpen.value = !isOpen.value;
  }

  return { isOpen, notifications, add, dismiss, clear, toggleDrawer };
});
