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

// Module-level task tracking so IDs survive component remounts.
// _searchTaskQueue is FIFO: front = currently being searched by IRC.
interface SearchTaskEntry { id: string; query: string; }
const _searchTaskQueue: SearchTaskEntry[] = [];
const _downloadTaskIds = new Map<string, string>(); // IRC book string → task ID

export function sendMessage(msg: unknown) {
  const serialized = JSON.stringify(msg);
  if (_sendFn) {
    _sendFn(serialized);
  }
}

/**
 * Called by SearchView before sending SEARCH so the task is created immediately.
 * Queues behind any already-running searches rather than superseding them.
 */
export function registerSearchTask(query: string): void {
  const taskStore = useTaskStore();
  const isFirst = _searchTaskQueue.length === 0;
  const now = Date.now();
  const task = taskStore.createTask('search', query, {
    status: isFirst ? 'active' : 'queued',
    events: [{ time: now, message: isFirst ? 'Started' : 'Queued', level: 'info' as const }],
    meta: { query },
    activeAt: isFirst ? now : undefined,
  });
  _searchTaskQueue.push({ id: task.id, query });
}

/** Returns the query currently being IRC-searched (front of FIFO queue). */
export function getActiveSearchQuery(): string | undefined {
  return _searchTaskQueue[0]?.query;
}

/**
 * Called by SearchView when the 60s timeout fires with no results.
 * Accepts the query so it can verify the front of the queue still matches
 * before shifting — prevents corrupting the queue when results arrived
 * while the user was viewing a different tab (and the timeout wasn't cleared).
 */
export function markSearchTimedOut(query: string): void {
  if (!query || _searchTaskQueue[0]?.query !== query) return;
  const entry = _searchTaskQueue.shift()!;
  const taskStore = useTaskStore();
  taskStore.updateTask(entry.id, { status: 'timed-out' }, 'Timed out — no response from IRC');
  // Promote the next queued search to active
  const next = _searchTaskQueue[0];
  if (next) taskStore.updateTask(next.id, { status: 'active', activeAt: Date.now() }, 'Started');
}

/**
 * Called when the user explicitly saves a download for later via the rename modal.
 * Clears the in-flight download state and marks the task done without waiting for
 * a server round-trip (the server only sends STAGED_BOOKS_NOTIFY, not DOWNLOAD).
 */
export function markActiveDownloadStaged(): void {
  const appStore = useAppStore();
  const taskStore = useTaskStore();
  const book = appStore.inFlightDownloads[0];
  if (!book) return;
  const taskId = _downloadTaskIds.get(book);
  if (taskId) {
    taskStore.updateTask(taskId, { status: 'done', phase: 'staged' }, 'Saved for later');
    _downloadTaskIds.delete(book);
  }
  appStore.removeInFlightDownload();
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
        // Route results to the correct history item.
        // Match by query from the front of the FIFO queue (server processes in order).
        // historyStore.items never carries results directly (updateItem strips them),
        // so we cannot use i.results === undefined to find the pending item.
        const completedEntry = _searchTaskQueue[0]; // peek before shifting
        const target = completedEntry
          ? (historyStore.items.find(i => i.query === completedEntry.query) ?? appStore.activeItem)
          : appStore.activeItem;
        if (target) {
          const updated = { ...target, results: books, errors, cachedAt };
          appStore.setRawSearchResult(target.timestamp, raw);
          historyStore.updateItem(updated);
          // Only update activeItem if this is the one currently being viewed
          if (appStore.activeItem?.timestamp === target.timestamp) {
            appStore.setActiveItem(updated);
          }
        }
        // Complete the task and promote the next queued search
        const done = _searchTaskQueue.shift();
        if (done) {
          const resultCount = books?.length ?? 0;
          const errorCount = errors?.length ?? 0;
          const summary = resultCount > 0
            ? `${resultCount} result${resultCount === 1 ? '' : 's'}${errorCount > 0 ? `, ${errorCount} error${errorCount === 1 ? '' : 's'}` : ''}`
            : 'No results';
          taskStore.updateTask(done.id, { status: 'done', resultCount, errorCount }, summary);
          const next = _searchTaskQueue[0];
          if (next) taskStore.updateTask(next.id, { status: 'active', activeAt: Date.now() }, 'Started');
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
      case MessageType.DOWNLOAD_FAILED: {
        const failedBook = appStore.inFlightDownloads[0];
        appStore.removeInFlightDownload();
        appStore.waitingDownload = null;
        appStore.setDownloadPhase(null);
        if (failedBook) {
          const taskId = _downloadTaskIds.get(failedBook);
          if (taskId) {
            taskStore.updateTask(taskId, { status: 'failed' }, 'Download failed');
            _downloadTaskIds.delete(failedBook);
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
      case MessageType.RATELIMIT: {
        const rateLimited = _searchTaskQueue.shift();
        if (rateLimited) {
          taskStore.updateTask(rateLimited.id, { status: 'failed' }, 'Rate limited');
          // Mark the tab as timed-out (shows WifiOff icon) rather than deleting it.
          const histItem = historyStore.items.find(i => i.query === rateLimited.query);
          if (histItem) historyStore.updateItem({ ...histItem, timedOut: true });
          // Promote next search to active — server already has it in its cooldown queue.
          const next = _searchTaskQueue[0];
          if (next) taskStore.updateTask(next.id, { status: 'active', activeAt: Date.now() }, 'Started');
        }
        break;
      }
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
    _searchTaskQueue.length = 0;
    _downloadTaskIds.clear();
    socket?.close(1000, "App unmounted");
  });
}
