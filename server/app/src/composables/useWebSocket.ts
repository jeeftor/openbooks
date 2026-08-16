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
  type StagedBooksNotifyResponse,
  type StagedBookResumeResponse,
  type StagedBooksListResponse,
  type HistoryListResponse,
  type SeriesAutocompleteResponse,
  type ServerListResponse,
  type AppNotification
} from "../types/messages";
import { useAppStore } from "../stores/app";
import { useHistoryStore } from "../stores/history";
import { useNotificationStore } from "../stores/notifications";
import { useTaskStore } from "../stores/tasks";

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

// Module-level task ID tracking so IDs survive component remounts.
let _activeSearchTaskId: string | null = null;
const _downloadTaskIds = new Map<string, string>(); // IRC book string → task ID

export function sendMessage(msg: unknown) {
  const serialized = JSON.stringify(msg);
  if (_sendFn) {
    _sendFn(serialized);
  }
}

/**
 * Called by SearchView before sending SEARCH so the task is created immediately.
 */
export function registerSearchTask(query: string): void {
  const taskStore = useTaskStore();
  const task = taskStore.createTask('search', query, { query });
  _activeSearchTaskId = task.id;
}

/**
 * Called by SearchView when the 60s timeout fires with no results.
 */
export function markSearchTimedOut(): void {
  if (!_activeSearchTaskId) return;
  const taskStore = useTaskStore();
  taskStore.updateTask(_activeSearchTaskId, { status: 'timed-out' }, 'Timed out — no response from IRC');
  _activeSearchTaskId = null;
}

/**
 * Called by DownloadButton before sending DOWNLOAD so the task is created immediately.
 */
export function registerDownloadTask(book: string, title?: string): void {
  const taskStore = useTaskStore();
  const label = title ?? book;
  const task = taskStore.createTask('download', label, { bookTitle: label, phase: 'queued' });
  _downloadTaskIds.set(book, task.id);
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
    const taskStore = useTaskStore();
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
        const { books, errors, raw, cachedAt } = response as SearchResponse;
        // Route results to the oldest pending (in-flight) history item, in
        // case the user navigated to a different item while waiting.
        const pending = historyStore.items.find(i => i.results === undefined && !i.timedOut);
        const target = pending ?? appStore.activeItem;
        if (target) {
          const updated = { ...target, results: books, errors, cachedAt };
          appStore.setRawSearchResult(target.timestamp, raw);
          historyStore.updateItem(updated);
          // Only update activeItem if this is the one currently being viewed
          if (appStore.activeItem?.timestamp === target.timestamp) {
            appStore.setActiveItem(updated);
          }
        }
        // Update search task
        if (_activeSearchTaskId) {
          const resultCount = books?.length ?? 0;
          const errorCount = errors?.length ?? 0;
          const summary = resultCount > 0
            ? `${resultCount} result${resultCount === 1 ? '' : 's'}${errorCount > 0 ? `, ${errorCount} error${errorCount === 1 ? '' : 's'}` : ''}`
            : 'No results';
          taskStore.updateTask(_activeSearchTaskId, { status: 'done', resultCount, errorCount }, summary);
          _activeSearchTaskId = null;
        }
        break;
      }
      case MessageType.DOWNLOAD: {
        const completedBook = appStore.inFlightDownloads[0];
        downloadFile((response as DownloadResponse).downloadPath);
        appStore.removeInFlightDownload();
        if (completedBook) {
          const taskId = _downloadTaskIds.get(completedBook);
          if (taskId) {
            taskStore.updateTask(taskId, { status: 'done', phase: 'done' }, 'Download complete');
            _downloadTaskIds.delete(completedBook);
          }
        }
        break;
      }
      case MessageType.RENAME_PROMPT: {
        appStore.pendingRename = response as RenamePromptResponse;
        appStore.waitingDownload = null;
        appStore.setDownloadPhase(null);
        const activeBook = appStore.inFlightDownloads[0];
        if (activeBook) {
          const taskId = _downloadTaskIds.get(activeBook);
          if (taskId) taskStore.updateTask(taskId, { phase: 'rename' }, 'Rename prompt');
        }
        return;
      }
      case MessageType.DOWNLOAD_WAITING: {
        const dw = response as DownloadWaitingResponse;
        appStore.waitingDownload = dw.active ? dw : null;
        const waitingBook = appStore.inFlightDownloads[0];
        if (waitingBook && dw.active) {
          const taskId = _downloadTaskIds.get(waitingBook);
          if (taskId) {
            taskStore.updateTask(taskId, {
              status: 'active',
              phase: 'waiting',
              bookTitle: dw.bookTitle ?? undefined,
            }, `Waiting for ${dw.bot ?? 'bot'}`);
          }
        }
        return;
      }
      case MessageType.DOWNLOAD_STARTED: {
        appStore.setDownloadPhase("transferring");
        const transferBook = appStore.inFlightDownloads[0];
        if (transferBook) {
          const taskId = _downloadTaskIds.get(transferBook);
          if (taskId) taskStore.updateTask(taskId, { phase: 'transferring' }, 'Transfer started');
        }
        return;
      }
      case MessageType.POST_PROCESS_STARTED: {
        appStore.setDownloadPhase("cleaning");
        const cleanBook = appStore.inFlightDownloads[0];
        if (cleanBook) {
          const taskId = _downloadTaskIds.get(cleanBook);
          if (taskId) taskStore.updateTask(taskId, { phase: 'cleaning' }, 'Post-processing');
        }
        return;
      }
      case MessageType.RATELIMIT:
        historyStore.deleteItem(undefined);
        if (_activeSearchTaskId) {
          taskStore.updateTask(_activeSearchTaskId, { status: 'failed' }, 'Rate limited');
          _activeSearchTaskId = null;
        }
        break;
      case MessageType.STAGED_BOOKS_NOTIFY:
        appStore.setStagedBooksCount((response as StagedBooksNotifyResponse).count);
        return;
      case MessageType.STAGED_BOOKS_LIST:
        appStore.setStagedBooksList((response as StagedBooksListResponse).books);
        break;
      case MessageType.STAGED_BOOK_RESUME:
        appStore.setPendingStagedBook(response as StagedBookResumeResponse);
        return;
      case MessageType.SERIES_AUTOCOMPLETE:
        appStore.setKnownSeries((response as SeriesAutocompleteResponse).series);
        return;
      case MessageType.HISTORY_LIST:
        historyStore.loadFromServer((response as HistoryListResponse).entries);
        return;
      case MessageType.SERVER_LIST: {
        const { servers, timestamp } = response as ServerListResponse;
        appStore.setServerList(servers, new Date(timestamp).getTime());
        return;
      }
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
