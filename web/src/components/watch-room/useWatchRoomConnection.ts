import { useEffect, useEffectEvent, useRef, useState } from "react";
import type { RefObject } from "react";
import { joinWatchRoom } from "@/lib/api";
import {
  WATCH_ROOM_CLIENT_EVENT_TYPES,
  WATCH_ROOM_EVENT_TYPES,
  WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS,
  WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC,
} from "@/lib/constants";
import {
  watchRoomAnnouncement,
  watchRoomWebSocketUrl,
} from "@/lib/watch-room";
import { showInfo } from "@/lib/toast-helpers";
import type {
  WatchRoomPlaybackStateType,
  WatchRoomServerEventType,
} from "@/types";

const MAX_RECONNECT_DELAY_MS = 16_000;

type WatchRoomPlaybackEventType =
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK;

type UseWatchRoomConnectionOptions = {
  currentRoomId: number | null;
  movieTitle: string;
  streamUrl: string;
  videoRef: RefObject<HTMLVideoElement | null>;
  onCurrentTimeChange: (time: number) => void;
  onPlayingChange: (playing: boolean) => void;
  onRoomDeleted: () => void;
};

export function useWatchRoomConnection({
  currentRoomId,
  movieTitle,
  streamUrl,
  videoRef,
  onCurrentTimeChange,
  onPlayingChange,
  onRoomDeleted,
}: UseWatchRoomConnectionOptions) {
  const socketRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const intentionalCloseRef = useRef(false);
  const socketInstanceIdRef = useRef(0);
  const prevRoomIdRef = useRef<number | null>(null);
  const roomDeletionHandledRef = useRef(false);
  const pendingPlaybackRef = useRef<{
    state: WatchRoomPlaybackStateType;
    receivedAt: number;
  } | null>(null);

  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [syncAnnouncementState, setSyncAnnouncementState] = useState<
    { text: string; token: number } | undefined
  >(undefined);
  const [connectedUserIds, setConnectedUserIds] = useState<number[]>([]);
  const [connectionReady, setConnectionReady] = useState(false);
  const [reconnectKey, setReconnectKey] = useState(0);

  const syncAnnouncement = syncAnnouncementState?.text;

  const clearHeartbeat = () => {
    if (heartbeatRef.current !== null) {
      window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = null;
    }
  };

  const closeRoomConnection = () => {
    intentionalCloseRef.current = true;

    if (reconnectTimeoutRef.current !== null) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    clearHeartbeat();

    const socket = socketRef.current;
    socketRef.current = null;

    if (
      socket &&
      socket.readyState !== WebSocket.CLOSING &&
      socket.readyState !== WebSocket.CLOSED
    ) {
      socket.close();
    }

    setConnectionReady(false);
  };

  const applyPlaybackState = useEffectEvent(
    async (playback: WatchRoomPlaybackStateType) => {
      const video = videoRef.current;
      if (!video || video.readyState < 1) {
        return false;
      }

      const targetTime = playback.position_sec;
      const drift = Math.abs(video.currentTime - targetTime);
      if (drift > WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC) {
        video.currentTime = targetTime;
      }

      if (playback.paused) {
        onCurrentTimeChange(targetTime);
        if (!video.paused) {
          video.pause();
        }
        onPlayingChange(false);
        setPlaybackError(null);
        return true;
      }

      if (video.readyState < 3) {
        return false;
      }

      try {
        if (video.paused) {
          await video.play();
        }
        setPlaybackError(null);
        onPlayingChange(true);
        return true;
      } catch {
        setPlaybackError(
          "Playback sync is waiting for browser permission. Press play to continue syncing with the room.",
        );
        return false;
      }
    },
  );

  const flushPendingPlayback = useEffectEvent(async () => {
    const pending = pendingPlaybackRef.current;
    if (!pending) return;

    const elapsed = !pending.state.paused
      ? (Date.now() - pending.receivedAt) / 1000
      : 0;
    const adjustedPlayback: WatchRoomPlaybackStateType = {
      ...pending.state,
      position_sec: pending.state.position_sec + elapsed,
    };

    const applied = await applyPlaybackState(adjustedPlayback);
    if (applied && pendingPlaybackRef.current === pending) {
      pendingPlaybackRef.current = null;
    }
  });

  const handleSocketMessage = useEffectEvent(async (rawEvent: MessageEvent) => {
    let event: WatchRoomServerEventType;
    try {
      if (typeof rawEvent.data !== "string") {
        return;
      }
      event = JSON.parse(rawEvent.data) as WatchRoomServerEventType;
    } catch {
      return;
    }

    if (event.type === WATCH_ROOM_EVENT_TYPES.ROOM_DELETED) {
      if (roomDeletionHandledRef.current) {
        return;
      }

      roomDeletionHandledRef.current = true;
      closeRoomConnection();
      showInfo("Watch room closed", `"${movieTitle}" is no longer available.`);
      onRoomDeleted();
      return;
    }

    if (event.connected_user_ids) {
      setConnectedUserIds(event.connected_user_ids);
    }

    const announcement = watchRoomAnnouncement(event, movieTitle);
    if (announcement) {
      setSyncAnnouncementState(currentAnnouncement => ({
        text: announcement,
        token: (currentAnnouncement?.token ?? 0) + 1,
      }));
    }

    if (!event.playback) return;

    pendingPlaybackRef.current = {
      state: event.playback,
      receivedAt: Date.now(),
    };
    await flushPendingPlayback();
  });

  const handleSocketOpen = useEffectEvent(() => {
    setConnectionReady(true);
    reconnectAttemptsRef.current = 0;
    const ws = socketRef.current;
    if (!ws) return;
    heartbeatRef.current = window.setInterval(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({ type: WATCH_ROOM_CLIENT_EVENT_TYPES.PING }),
        );
      }
    }, 25_000);
  });

  const handleSocketClose = useEffectEvent((closedInstanceId: number) => {
    setConnectionReady(false);
    clearHeartbeat();
    if (closedInstanceId !== socketInstanceIdRef.current) return;
    if (!intentionalCloseRef.current) {
      const delay = Math.min(
        1000 * 2 ** Math.min(reconnectAttemptsRef.current, 10),
        MAX_RECONNECT_DELAY_MS,
      );
      reconnectAttemptsRef.current += 1;
      setPlaybackError(null);
      reconnectTimeoutRef.current = window.setTimeout(() => {
        reconnectTimeoutRef.current = null;
        setReconnectKey(k => k + 1);
      }, delay);
    }
  });

  const handleSocketError = useEffectEvent(() => {
    setPlaybackError("Realtime sync connection failed for this watch room.");
  });

  useEffect(() => {
    roomDeletionHandledRef.current = false;
    intentionalCloseRef.current = false;
    if (prevRoomIdRef.current !== currentRoomId) {
      prevRoomIdRef.current = currentRoomId;
      reconnectAttemptsRef.current = 0;
      pendingPlaybackRef.current = null;
    }
    if (!currentRoomId) return;

    let cancelled = false;
    let socket: WebSocket | null = null;
    socketInstanceIdRef.current += 1;
    const instanceId = socketInstanceIdRef.current;

    const initRoomConnection = async () => {
      let res: Awaited<ReturnType<typeof joinWatchRoom>>;
      try {
        res = await joinWatchRoom(currentRoomId);
      } catch (error) {
        if (cancelled) return;
        setPlaybackError(
          error instanceof Error
            ? error.message
            : "Unable to join this watch room.",
        );
        return;
      }

      if (cancelled) return;

      if (res.error) {
        setPlaybackError(res.message || "Unable to join this watch room.");
        return;
      }

      socket = new WebSocket(watchRoomWebSocketUrl(currentRoomId));
      socketRef.current = socket;

      socket.addEventListener("open", handleSocketOpen);
      socket.addEventListener("message", event => {
        void handleSocketMessage(event);
      });
      socket.addEventListener("close", () => handleSocketClose(instanceId));
      socket.addEventListener("error", handleSocketError);
    };

    void initRoomConnection();

    return () => {
      cancelled = true;
      intentionalCloseRef.current = true;

      if (reconnectTimeoutRef.current !== null) {
        window.clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      clearHeartbeat();

      if (socketRef.current === socket) {
        socketRef.current = null;
      }

      if (
        socket &&
        socket.readyState !== WebSocket.CLOSING &&
        socket.readyState !== WebSocket.CLOSED
      ) {
        socket.close();
      }
    };
  }, [currentRoomId, reconnectKey]);

  useEffect(() => {
    if (!syncAnnouncementState) return;

    const timeoutId = window.setTimeout(() => {
      setSyncAnnouncementState(currentAnnouncement => (
        currentAnnouncement === syncAnnouncementState
          ? undefined
          : currentAnnouncement
      ));
    }, WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [syncAnnouncementState]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const roomIdAtEffectTime = currentRoomId;
    const handleReady = () => {
      if (prevRoomIdRef.current === roomIdAtEffectTime) {
        void flushPendingPlayback();
      }
    };

    video.addEventListener("loadedmetadata", handleReady);
    video.addEventListener("canplay", handleReady);
    return () => {
      video.removeEventListener("loadedmetadata", handleReady);
      video.removeEventListener("canplay", handleReady);
    };
  }, [streamUrl, currentRoomId, videoRef]);

  const sendPlaybackEvent = (
    type: WatchRoomPlaybackEventType,
    positionSec: number,
  ) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(
      JSON.stringify({
        type,
        position_sec: positionSec,
      }),
    );
  };

  const syncToPendingPlayback = () => {
    const video = videoRef.current;
    const pending = pendingPlaybackRef.current;
    if (!video || !pending || pending.state.paused) return;

    const elapsed = (Date.now() - pending.receivedAt) / 1000;
    const adjustedPos = pending.state.position_sec + elapsed;
    const drift = Math.abs(video.currentTime - adjustedPos);
    if (video.readyState >= 1 && drift > WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC) {
      video.currentTime = adjustedPos;
      onCurrentTimeChange(adjustedPos);
    }
  };

  const clearPendingPlayback = () => {
    pendingPlaybackRef.current = null;
  };

  const markRoomDeletionHandled = (handled: boolean) => {
    roomDeletionHandledRef.current = handled;
  };

  return {
    playbackError,
    setPlaybackError,
    syncAnnouncement,
    connectedUserIds,
    connectionReady,
    closeRoomConnection,
    sendPlaybackEvent,
    syncToPendingPlayback,
    clearPendingPlayback,
    markRoomDeletionHandled,
  };
}
