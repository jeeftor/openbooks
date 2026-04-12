import {
  AnyAction,
  Dispatch,
  Middleware,
  MiddlewareAPI,
  PayloadAction
} from "@reduxjs/toolkit";
import { openbooksApi } from "./api";
import { deleteHistoryItem } from "./historySlice";
import {
  ConnectionResponse,
  DownloadResponse,
  MessageType,
  Notification,
  NotificationType,
  Response,
  SearchResponse
} from "./messages";
import { addNotification } from "./notificationSlice";
import {
  removeInFlightDownload,
  sendMessage,
  setConnectionState,
  setSearchResults,
  setUsername
} from "./stateSlice";
import { AppDispatch, RootState } from "./store";
import { displayNotification, downloadFile } from "./util";

const MAX_RETRIES = 10;
const INITIAL_RETRY_DELAY = 1000; // 1 second
const MAX_RETRY_DELAY = 30000; // 30 seconds

// Web socket redux middleware with automatic reconnection.
// Listens to socket and dispatches handlers.
// Handles send_message actions by sending to socket.
export const websocketConn =
  (wsUrl: string): Middleware =>
  ({ dispatch, getState }: MiddlewareAPI<AppDispatch, RootState>) => {
    let socket: WebSocket | null = null;
    let retryCount = 0;
    let retryTimeout: ReturnType<typeof setTimeout> | null = null;
    let pendingMessages: string[] = []; // Queue for messages sent while disconnected

    const connect = () => {
      console.log(`WebSocket connecting to ${wsUrl}...`);

      socket = new WebSocket(wsUrl);

      socket.onopen = () => {
        console.log("WebSocket connected.");
        retryCount = 0; // Reset retry count on successful connection
        dispatch(setConnectionState(true));
        dispatch(sendMessage({ type: MessageType.CONNECT, payload: {} }));

        // Send any queued messages
        if (pendingMessages.length > 0) {
          console.log(`Sending ${pendingMessages.length} queued message(s)...`);
          pendingMessages.forEach((msg) => {
            socket?.send(msg);
          });
          pendingMessages = [];
        }
      };

      socket.onclose = (event) => {
        console.log("WebSocket closed.", event.code, event.reason);
        dispatch(setConnectionState(false));

        // Don't retry if closed cleanly (code 1000) or if we've exceeded retries
        if (event.code !== 1000 && retryCount < MAX_RETRIES) {
          scheduleRetry();
        }
      };

      socket.onerror = (event) => {
        console.error("WebSocket error:", event);
        // Don't show error notification on every error - onclose will handle retry
      };

      socket.onmessage = (message) => route(dispatch, message);
    };

    const scheduleRetry = () => {
      retryCount++;
      const delay = Math.min(
        INITIAL_RETRY_DELAY * Math.pow(2, retryCount - 1),
        MAX_RETRY_DELAY
      );

      console.log(
        `Reconnecting in ${delay / 1000}s (attempt ${retryCount}/${MAX_RETRIES})...`
      );

      displayNotification({
        appearance: NotificationType.WARNING,
        title: `Connection lost. Retrying in ${Math.round(delay / 1000)}s... (${retryCount}/${MAX_RETRIES})`,
        timestamp: new Date().getTime()
      });

      retryTimeout = setTimeout(() => {
        connect();
      }, delay);
    };

    // Initial connection
    connect();

    return (next: Dispatch<AnyAction>) => (action: PayloadAction<any>) => {
      // Send Message action? Send data to the socket.
      if (sendMessage.match(action)) {
        if (socket && socket.readyState === WebSocket.OPEN) {
          socket.send(action.payload.message);
        } else {
          // Queue the message to be sent when reconnected
          pendingMessages.push(action.payload.message);
          displayNotification({
            appearance: NotificationType.WARNING,
            title:
              "Not connected. Message queued - will send when reconnected.",
            timestamp: new Date().getTime()
          });
        }
      }

      return next(action);
    };
  };

const route = (dispatch: AppDispatch, msg: MessageEvent<any>): void => {
  const getNotif = (): Notification => {
    let response = JSON.parse(msg.data) as Response;
    const timestamp = new Date().getTime();
    const notification: Notification = {
      ...response,
      timestamp
    };

    switch (response.type) {
      case MessageType.STATUS:
        return notification;
      case MessageType.CONNECT:
        dispatch(setUsername((response as ConnectionResponse).name));
        return notification;
      case MessageType.SEARCH:
        dispatch(setSearchResults(response as SearchResponse));
        return notification;
      case MessageType.DOWNLOAD:
        downloadFile((response as DownloadResponse)?.downloadPath);
        dispatch(openbooksApi.util.invalidateTags(["books"]));
        dispatch(removeInFlightDownload());
        return notification;
      case MessageType.RATELIMIT:
        dispatch(deleteHistoryItem());
        return notification;
      default:
        console.error(response);
        return {
          appearance: NotificationType.DANGER,
          title: "Unknown message type. See console.",
          timestamp
        };
    }
  };

  const notif = getNotif();
  dispatch(addNotification(notif));
  displayNotification(notif);
};
