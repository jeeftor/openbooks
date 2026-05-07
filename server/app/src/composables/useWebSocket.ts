import { onUnmounted } from "vue";
import { toast } from "vue-sonner";
import {
  MessageType,
  NotificationType,
  type WsResponse,
  type ConnectionResponse,
  type SearchResponse,
  type DownloadResponse,
  type RenamePromptResponse,
  type DownloadWaitingResponse,
  type AppNotification
} from "../types/messages";
import { useAppStore } from "../stores/app";
import { useHistoryStore } from "../stores/history";
import { useNotificationStore } from "../stores/notifications";

const MAX_RETRIES = 10;
const INITIAL_DELAY = 1000;
const MAX_DELAY = 30000;

export function getWsUrl(): string {
  const url = new URL("/ws", window.location.href);
  url.protocol = url.protocol.replace("http", "ws");
  if (import.meta.env.DEV) {
    url.port = "5228";
  }
  return url.toString();
}

export function getApiUrl(path: string): string {
  const url = new URL(path, window.location.href);
  if (import.meta.env.DEV) {
    url.port = "5228";
  }
  return url.toString();
}

export function downloadFile(relativeURL?: string) {
  if (!relativeURL) return;
  const link = document.createElement("a");
  link.href = getApiUrl("/" + relativeURL);
  link.download = "";
  link.target = "_blank";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}

let _sendFn: ((serialized: string) => void) | null = null;

export function sendMessage(msg: unknown) {
  const serialized = JSON.stringify(msg);
  if (_sendFn) {
    _sendFn(serialized);
  }
}

export function useWebSocket() {
  const appStore = useAppStore();
  const historyStore = useHistoryStore();
  const notifStore = useNotificationStore();

  let socket: WebSocket | null = null;
  let retryCount = 0;
  let retryTimeout: ReturnType<typeof setTimeout> | null = null;
  let pendingMessages: string[] = [];

  function showToast(notif: AppNotification) {
    switch (notif.appearance) {
      case NotificationType.SUCCESS:
        toast.success(notif.title, { description: notif.detail });
        break;
      case NotificationType.WARNING:
        toast.warning(notif.title, { description: notif.detail });
        break;
      case NotificationType.DANGER:
        toast.error(notif.title, { description: notif.detail });
        break;
      default:
        toast(notif.title, { description: notif.detail });
    }
  }

  function route(event: MessageEvent) {
    const response = JSON.parse(event.data as string) as WsResponse;
    const timestamp = Date.now();
    const notification: AppNotification = { ...response, timestamp };

    switch (response.type) {
      case MessageType.CONNECT:
        // Internal protocol event — just update username, no toast/notification.
        appStore.setUsername((response as ConnectionResponse).name);
        return;
      case MessageType.STATUS:
        // Internal status ping — no toast/notification.
        return;
      case MessageType.SEARCH: {
        const { books, errors } = response as SearchResponse;
        const active = appStore.activeItem;
        if (active) {
          const updated = { ...active, results: books, errors };
          appStore.setActiveItem(updated);
          historyStore.updateItem(updated);
        }
        break;
      }
      case MessageType.DOWNLOAD:
        downloadFile((response as DownloadResponse).downloadPath);
        appStore.removeInFlightDownload();
        break;
      case MessageType.RENAME_PROMPT:
        appStore.pendingRename = response as RenamePromptResponse;
        appStore.waitingDownload = null; // clear waiting state when modal appears
        return;
      case MessageType.DOWNLOAD_WAITING: {
        const dw = response as DownloadWaitingResponse;
        appStore.waitingDownload = dw.active ? dw : null;
        return;
      }
      case MessageType.RATELIMIT:
        historyStore.deleteItem(undefined);
        break;
      default:
        console.error("Unknown WS message type:", response);
    }

    // Only add user-facing messages to the notification store and show toasts.
    notifStore.add(notification);
    showToast(notification);
  }

  function send(serialized: string) {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(serialized);
    } else {
      pendingMessages.push(serialized);
      toast.warning("Not connected — message queued until reconnected.");
    }
  }

  function connect() {
    socket = new WebSocket(getWsUrl());

    socket.onopen = () => {
      retryCount = 0;
      appStore.setConnected(true);
      appStore.setConnecting(false);
      sendMessage({ type: MessageType.CONNECT, payload: {} });
      if (pendingMessages.length > 0) {
        pendingMessages.forEach((m) => socket?.send(m));
        pendingMessages = [];
      }
    };

    socket.onclose = (event) => {
      appStore.setConnected(false);
      if (event.code !== 1000 && retryCount < MAX_RETRIES) {
        scheduleRetry();
      } else {
        // Intentional close (code 1000) or max retries exhausted — stop spinning.
        appStore.setConnecting(false);
      }
    };

    socket.onerror = () => {
      /* handled by onclose */
    };
    socket.onmessage = route;

    _sendFn = send;
  }

  function scheduleRetry() {
    retryCount++;
    appStore.setConnecting(true);
    const delay = Math.min(
      INITIAL_DELAY * Math.pow(2, retryCount - 1),
      MAX_DELAY
    );
    toast.warning(
      `Connection lost. Retrying in ${Math.round(delay / 1000)}s… (${retryCount}/${MAX_RETRIES})`
    );
    retryTimeout = setTimeout(connect, delay);
  }

  connect();

  onUnmounted(() => {
    if (retryTimeout) clearTimeout(retryTimeout);
    _sendFn = null;
    socket?.close(1000, "App unmounted");
  });
}
