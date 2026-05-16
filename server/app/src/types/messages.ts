export enum NotificationType {
  NOTIFY,
  SUCCESS,
  WARNING,
  DANGER
}

export enum MessageType {
  STATUS,
  CONNECT,
  SEARCH,
  DOWNLOAD,
  RATELIMIT,
  RENAME_PROMPT,
  RENAME_CONFIRM,
  DOWNLOAD_WAITING,
  DOWNLOAD_STARTED,
  POST_PROCESS_STARTED,
  STAGED_BOOKS_NOTIFY,
  STAGED_BOOK_RESUME,
  STAGED_QUEUE_LATER,
  SERIES_AUTOCOMPLETE,
  PROCESS_STAGED_BOOKS,
  DELETE_STAGED,
  GET_STAGED_LIST,
  STAGED_BOOKS_LIST,
  PROCESS_ONE_STAGED
}

export interface AppNotification {
  appearance: NotificationType;
  title: string;
  detail?: string;
  timestamp: number;
}

export interface WsResponse {
  type: MessageType;
  appearance: NotificationType;
  title: string;
  detail?: string;
}

export interface ConnectionResponse extends WsResponse {
  name: string;
}

export interface SearchResponse extends WsResponse {
  books: BookDetail[];
  errors: ParseError[];
  raw?: string;
}

export interface DownloadResponse extends WsResponse {
  downloadPath?: string;
}

export interface BookDetail {
  server: string;
  author: string;
  title: string;
  format: string;
  size: string;
  full: string;
}

export interface ParseError {
  error: string;
  line: string;
}

export interface Book {
  name: string;
  path: string;
  downloadLink: string;
  time: string;
}

export interface LogEntry {
  time: string;
  level: "info" | "warn" | "error";
  message: string;
  detail?: string;
  group?: string;
}

export interface VersionInfo {
  displayVersion: string;
  rawVersion: string;
  commitSha: string;
  buildDate: string;
  releaseNotesUrl?: string;
  isRelease: boolean;
  update?: VersionUpdate;
}

export interface VersionUpdate {
  status: "available" | "current" | "unknown";
  available: boolean;
  currentVersion: string;
  latestVersion?: string;
  releaseNotesUrl?: string;
  checkedAt?: string;
  reason?: string;
}

export interface HistoryItem {
  query: string;
  timestamp: number;
  results?: BookDetail[];
  errors?: ParseError[];
  timedOut?: boolean;
}

export interface EPUBMetadata {
  Author: string;
  Title: string;
  Series: string;
  SeriesIndex: string;
}

export interface RenameOption {
  id: string;
  label: string;
  preview: string;
  isOrganized: boolean;
}

export interface RenamePromptResponse extends WsResponse {
  ircFilename: string;
  metadata?: EPUBMetadata;
  options: RenameOption[];
  replaceSpace: string;
  coverBase64?: string;
  coverMime?: string;
}

export interface DownloadWaitingResponse extends WsResponse {
  active: boolean;
  bot?: string;
  bookTitle?: string;
  timeoutSecs?: number;
}

export interface RenameConfirmRequest {
  type: MessageType.RENAME_CONFIRM;
  payload: {
    optionId: string;
    customName: string;
    fileName: string;
    rewriteMetadata: boolean;
    author: string;
    title: string;
    series: string;
    seriesIndex: string;
    stagedId?: string;
  };
}

export interface StagedBooksNotifyResponse extends WsResponse {
  count: number;
}

export interface StagedBookResumeResponse extends WsResponse {
  stagedId: string;
  ircFilename: string;
  metadata?: EPUBMetadata;
  options: RenameOption[];
  replaceSpace: string;
  coverBase64?: string;
  coverMime?: string;
  stagedAt: string;
  queuePosition: number;
  totalQueued: number;
}

export interface SeriesAutocompleteResponse extends WsResponse {
  series: string[];
}

export interface StagedBookSummary {
  id: string;
  ircFilename: string;
  metadata?: EPUBMetadata;
  coverBase64?: string;
  coverMime?: string;
  stagedAt: string;
}

export interface StagedBooksListResponse extends WsResponse {
  books: StagedBookSummary[];
}

export interface StageQueueLaterRequest {
  type: MessageType.STAGED_QUEUE_LATER;
  payload: {
    stagedId: string;
  };
}
