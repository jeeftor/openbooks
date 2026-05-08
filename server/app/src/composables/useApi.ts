import { ref, watch, onUnmounted } from "vue";
import { getApiUrl } from "./useWebSocket";
import type { Book, LogEntry, VersionInfo } from "../types/messages";

export function useVersion() {
  const version = ref<VersionInfo | undefined>(undefined);
  fetch(getApiUrl("/version"))
    .then((r) => r.json())
    .then((v: string | VersionInfo) => {
      version.value = normalizeVersion(v);
    })
    .catch(() => {});
  return version;
}

function normalizeVersion(version: string | VersionInfo): VersionInfo {
  if (typeof version !== "string") {
    return version;
  }

  return {
    displayVersion: version,
    rawVersion: version,
    commitSha: "",
    buildDate: "",
    isRelease: false
  };
}

export function useServers() {
  const servers = ref<string[]>([]);

  async function fetchServers() {
    try {
      const res = await fetch(getApiUrl("/servers"));
      if (res.ok) {
        const data = (await res.json()) as { elevatedUsers?: string[] };
        servers.value = data.elevatedUsers ?? [];
      }
    } catch {
      /* network error */
    }
  }

  fetchServers();
  const interval = setInterval(fetchServers, 30_000);
  onUnmounted(() => clearInterval(interval));

  return { servers, refresh: fetchServers };
}

export function useBooks(libraryVersion: { readonly value: number }) {
  const books = ref<Book[]>([]);
  const loading = ref(false);

  async function fetchBooks() {
    loading.value = true;
    try {
      const res = await fetch(getApiUrl("/library"));
      books.value = res.ok ? ((await res.json()) as Book[]) : [];
    } catch {
      books.value = [];
    } finally {
      loading.value = false;
    }
  }

  fetchBooks();
  watch(() => libraryVersion.value, fetchBooks);

  return { books, loading, refresh: fetchBooks };
}

export async function deleteBook(path: string): Promise<boolean> {
  try {
    const res = await fetch(getApiUrl("/library/" + path), {
      method: "DELETE"
    });
    return res.ok;
  } catch {
    return false;
  }
}

export function useLogs() {
  const logs = ref<LogEntry[]>([]);
  const loading = ref(false);

  async function fetchLogs() {
    loading.value = true;
    try {
      const res = await fetch(getApiUrl("/logs"));
      if (res.ok) logs.value = (await res.json()) as LogEntry[];
    } catch {
      /* network error */
    } finally {
      loading.value = false;
    }
  }

  fetchLogs();
  const interval = setInterval(fetchLogs, 5_000);
  onUnmounted(() => clearInterval(interval));
  return { logs, loading, refresh: fetchLogs };
}
