import { useEffect, useEffectEvent, useRef, useState } from "react";
import type { RefObject } from "react";
import { joinWatchRoom } from "@/lib/api";
import {
  WATCH_ROOM_CLIENT_EVENT_TYPES,
  WATCH_ROOM_ECHO_POSITION_TOLERANCE_SEC,
  WATCH_ROOM_ECHO_TIMEOUT_MS,
  WATCH_ROOM_EVENT_TYPES,
  WATCH_ROOM_HEARTBEAT_INTERVAL_MS,
  WATCH_ROOM_PAUSED_SYNC_TOLERANCE_SEC,
  WATCH_ROOM_SEEK_COALESCE_MS,
  WATCH_ROOM_STABLE_CONNECTION_MS,
  WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS,
  WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC,
} from "@/lib/constants";
import {
  watchRoomAnnouncement,
  watchRoomWebSocketUrl,
} from "@/lib/watch-room";
import {
  isInterruptedPlayError,
  isSourceFailurePlayError,
} from "@/lib/video-playback";
import type {
  WatchRoomMemberType,
  WatchRoomPlaybackStateType,
  WatchRoomServerEventType,
} from "@/types";
import { clearTimeoutRef } from "@/lib/utils";

const MAX_RECONNECT_DELAY_MS = 16_000;
// Echoes still waiting past this many are dropped oldest first.
const MAX_PENDING_ECHOES = 8;

type WatchRoomPlaybackEventType =
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE
  | typeof WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK;

type RoomPlaybackSnapshot = {
  state: WatchRoomPlaybackStateType;
  receivedAt: number;
  /**
   * A local Play overtook this paused state before the media loaded: only its
   * position still applies, since that play is pending and starts the media.
   */
  overtakenByPlay?: boolean;
};

/** A command this viewer sent, waiting for the server to send it back. */
type PendingEcho = {
  type: WatchRoomPlaybackEventType;
  positionSec: number;
  sentAt: number;
};

/**
 * - `connecting`: the first join and socket are on their way.
 * - `live`: the socket is open and the room's state has arrived.
 * - `reconnecting`: the link dropped; this viewer's presses move only their
 *   own player until it is back, and the room's state wins then.
 */
export type WatchRoomConnectionState = "connecting" | "live" | "reconnecting";

/**
 * Why the room ended for this viewer: the owner closed it, or this viewer is
 * no longer a member (removed, left on another device, or gone while away).
 */
type WatchRoomEndReason = "deleted" | "revoked";

type UseWatchRoomConnectionOptions = {
  currentRoomId: number | null;
  /** The signed-in user, so a room the server forgot can be put back. */
  selfUserId: number | null;
  movieTitle: string;
  streamUrl: string;
  videoRef: RefObject<HTMLVideoElement | null>;
  onCurrentTimeChange: (time: number) => void;
  onPlayingChange: (playing: boolean) => void;
  onMembersChanged: (members: WatchRoomMemberType[]) => void;
  /**
   * The socket is back after a drop: anything the room said while it was
   * down (a member change) never reached this viewer, so the room is re-read.
   */
  onReconnected: () => void;
  onRoomEnded: (reason: WatchRoomEndReason) => void;
};

/** Where a snapshot puts the room now: a playing room moved on since it came. */
function projectedPosition({ state, receivedAt }: RoomPlaybackSnapshot) {
  return state.paused
    ? state.position_sec
    : state.position_sec + (Date.now() - receivedAt) / 1000;
}

function sameIds(a: number[], b: number[]) {
  return a.length === b.length && a.every((id, index) => id === b[index]);
}

function isEchoOf(echo: PendingEcho, playback: WatchRoomPlaybackStateType) {
  const pausedMatches =
    echo.type === WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY
      ? !playback.paused
      : echo.type === WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE
        ? playback.paused
        : true;
  return (
    pausedMatches &&
    Math.abs(playback.position_sec - echo.positionSec) <
      WATCH_ROOM_ECHO_POSITION_TOLERANCE_SEC
  );
}

export function useWatchRoomConnection({
  currentRoomId,
  selfUserId,
  movieTitle,
  streamUrl,
  videoRef,
  onCurrentTimeChange,
  onPlayingChange,
  onMembersChanged,
  onReconnected,
  onRoomEnded,
}: UseWatchRoomConnectionOptions) {
  const socketRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const stableConnectionTimeoutRef = useRef<number | null>(null);
  const seekTimeoutRef = useRef<number | null>(null);
  const pendingSeekRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const intentionalCloseRef = useRef(false);
  const socketInstanceIdRef = useRef(0);
  const roomEndHandledRef = useRef(false);
  // Whether a socket has opened for this room yet: the next one is a reconnect.
  const openedOnceRef = useRef(false);
  const pendingPlaybackRef = useRef<RoomPlaybackSnapshot | null>(null);
  const pendingEchoesRef = useRef<PendingEcho[]>([]);
  // The room's latest known playback state: what the server last sent, or
  // what this viewer last pressed. A replaced stream source starts over at its
  // beginning, paused, so this is applied to it again.
  const roomPlaybackRef = useRef<RoomPlaybackSnapshot | null>(null);

  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [syncAnnouncementState, setSyncAnnouncementState] = useState<
    { text: string; token: number } | undefined
  >(undefined);
  const [connectedUserIds, setConnectedUserIds] = useState<number[]>([]);
  const [connectionState, setConnectionState] =
    useState<WatchRoomConnectionState>("connecting");
  const [reconnectKey, setReconnectKey] = useState(0);

  const syncAnnouncement = syncAnnouncementState?.text;
  // Keys the announcer, so the same words twice (two members pressing Play)
  // are spoken twice.
  const syncAnnouncementKey = syncAnnouncementState?.token;

  const announce = (text: string) => {
    setSyncAnnouncementState(currentAnnouncement => ({
      text,
      token: (currentAnnouncement?.token ?? 0) + 1,
    }));
  };

  const clearHeartbeat = () => {
    if (heartbeatRef.current !== null) {
      window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = null;
    }
  };

  const clearPendingSeek = () => {
    clearTimeoutRef(seekTimeoutRef);
    pendingSeekRef.current = null;
  };

  // Stops everything a socket keeps going and closes it; the caller has
  // already marked the close intentional.
  const stopSocket = (socket: WebSocket | null) => {
    clearTimeoutRef(reconnectTimeoutRef);
    clearTimeoutRef(stableConnectionTimeoutRef);
    clearPendingSeek();
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

  const closeRoomConnection = () => {
    intentionalCloseRef.current = true;
    stopSocket(socketRef.current);
  };

  const stopSocketOnCleanup = useEffectEvent(stopSocket);

  const endRoom = useEffectEvent((reason: WatchRoomEndReason) => {
    if (roomEndHandledRef.current) return;
    roomEndHandledRef.current = true;
    closeRoomConnection();
    onRoomEnded(reason);
  });

  const applyPlaybackState = useEffectEvent(
    async (playback: WatchRoomPlaybackStateType, overtakenByPlay: boolean) => {
      const video = videoRef.current;
      if (!video || video.readyState < 1) {
        return false;
      }

      const targetTime = playback.position_sec;
      const drift = Math.abs(video.currentTime - targetTime);
      const tolerance = playback.paused
        ? WATCH_ROOM_PAUSED_SYNC_TOLERANCE_SEC
        : WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC;
      if (drift > tolerance) {
        video.currentTime = targetTime;
      }

      if (playback.paused) {
        onCurrentTimeChange(targetTime);
        if (overtakenByPlay) {
          return true;
        }
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
      } catch (error) {
        // Not a refusal: a source swap or a pause cut the play short, and the
        // state stays pending for the next canplay; or the source failed,
        // which the player's error event reports.
        const refused =
          !isInterruptedPlayError(error) && !isSourceFailurePlayError(error);
        if (refused) {
          setPlaybackError(
            "Playback sync is waiting for browser permission. Press play to continue syncing with the room.",
          );
        }
        return false;
      }
    },
  );

  const flushPendingPlayback = useEffectEvent(async () => {
    const pending = pendingPlaybackRef.current;
    if (!pending) return;

    const adjustedPlayback: WatchRoomPlaybackStateType = {
      ...pending.state,
      position_sec: projectedPosition(pending),
    };

    const applied = await applyPlaybackState(
      adjustedPlayback,
      pending.overtakenByPlay === true,
    );
    if (applied && pendingPlaybackRef.current === pending) {
      pendingPlaybackRef.current = null;
    }
  });

  const followRoom = useEffectEvent(
    async (playback: WatchRoomPlaybackStateType) => {
      const snapshot = { state: playback, receivedAt: Date.now() };
      pendingPlaybackRef.current = snapshot;
      roomPlaybackRef.current = snapshot;
      await flushPendingPlayback();
    },
  );

  // The echoes that can still come back.
  const liveEchoes = (now: number) =>
    pendingEchoesRef.current.filter(
      echo => now - echo.sentAt < WATCH_ROOM_ECHO_TIMEOUT_MS,
    );

  const sendCommand = (type: WatchRoomPlaybackEventType, positionSec: number) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({ type, position_sec: positionSec }));

    const now = Date.now();
    pendingEchoesRef.current = [
      ...liveEchoes(now),
      { type, positionSec, sentAt: now },
    ].slice(-MAX_PENDING_ECHOES);
  };

  /**
   * The playback_changed the server sent: this viewer's own command coming
   * back, or another member's. Returns whether it was an echo.
   */
  const takeEcho = (playback: WatchRoomPlaybackStateType) => {
    const echoes = liveEchoes(Date.now());
    const head = echoes[0];
    if (head && isEchoOf(head, playback)) {
      pendingEchoesRef.current = echoes.slice(1);
      return { echo: true, newerPending: echoes.length > 1 };
    }
    pendingEchoesRef.current = echoes;
    return { echo: false, newerPending: false };
  };

  const handleSnapshot = useEffectEvent(async (
    playback: WatchRoomPlaybackStateType,
    /** Undefined when the snapshot did not say who is here. */
    connected: number[] | undefined,
  ) => {
    pendingEchoesRef.current = [];

    // An emptied room forgets where it was and starts again paused at 0: this
    // viewer's own drop was its last connection, or the server restarted.
    // Back in it alone, this viewer is the only memory of the room, so it
    // puts the room back where it was. IglooTV does the same.
    const known = roomPlaybackRef.current;
    const video = videoRef.current;
    const ownPosition =
      video && video.readyState >= 1 ? video.currentTime : null;
    const forgotten =
      playback.paused && playback.position_sec === 0 && (ownPosition ?? 0) > 0;
    // Only a snapshot that names who is here can say this viewer is alone;
    // without the list, overwriting the room could undo others' playback.
    const alone =
      connected !== undefined &&
      selfUserId !== null &&
      connected.every(id => id === selfUserId);
    if (known && forgotten && alone && ownPosition !== null) {
      const type = video?.paused
        ? WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE
        : WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY;
      sendCommand(type, ownPosition);
      roomPlaybackRef.current = {
        state: {
          paused: type === WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE,
          position_sec: ownPosition,
          updated_at: new Date().toISOString(),
        },
        receivedAt: Date.now(),
      };
      return;
    }

    await followRoom(playback);
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

    switch (event.type) {
      case WATCH_ROOM_EVENT_TYPES.ROOM_DELETED:
        endRoom("deleted");
        return;
      case WATCH_ROOM_EVENT_TYPES.ROOM_ACCESS_REVOKED:
        endRoom("revoked");
        return;
      case WATCH_ROOM_EVENT_TYPES.MEMBERS_CHANGED:
        if (event.members) onMembersChanged(event.members);
        return;
      case WATCH_ROOM_EVENT_TYPES.PONG:
        return;
      default:
        break;
    }

    const connectedIds = event.connected_user_ids;
    if (connectedIds) {
      setConnectedUserIds(previous =>
        sameIds(previous, connectedIds) ? previous : connectedIds,
      );
    }

    if (event.type === WATCH_ROOM_EVENT_TYPES.ROOM_SNAPSHOT) {
      if (event.playback) {
        await handleSnapshot(event.playback, connectedIds);
      }
      return;
    }

    if (event.type === WATCH_ROOM_EVENT_TYPES.PLAYBACK_CHANGED && event.playback) {
      const { echo, newerPending } = takeEcho(event.playback);
      // This viewer's own command, back from the server. With newer ones
      // still on their way it is out of date and changes nothing. The last
      // one is the server's word on the room and is followed: usually the
      // player is already there and the drift threshold makes it a no-op,
      // but another member's command can have landed in between.
      if (echo && newerPending) return;
      if (!echo) {
        const announcement = watchRoomAnnouncement(event, movieTitle);
        if (announcement) announce(announcement);
      }
      await followRoom(event.playback);
      return;
    }

    const announcement = watchRoomAnnouncement(event, movieTitle);
    if (announcement) announce(announcement);
  });

  const handleSocketOpen = useEffectEvent(() => {
    // The server sends the room's snapshot first thing, right behind this.
    setConnectionState("live");
    // The backoff resets only once the link has held: a server that accepts
    // the upgrade and drops it at once used to be retried, with a join, every
    // second for as long as it did.
    clearTimeoutRef(stableConnectionTimeoutRef);
    stableConnectionTimeoutRef.current = window.setTimeout(() => {
      stableConnectionTimeoutRef.current = null;
      reconnectAttemptsRef.current = 0;
    }, WATCH_ROOM_STABLE_CONNECTION_MS);
    if (openedOnceRef.current) onReconnected();
    openedOnceRef.current = true;
    const ws = socketRef.current;
    if (!ws) return;
    heartbeatRef.current = window.setInterval(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({ type: WATCH_ROOM_CLIENT_EVENT_TYPES.PING }),
        );
      }
    }, WATCH_ROOM_HEARTBEAT_INTERVAL_MS);
  });

  const scheduleReconnect = () => {
    const delay = Math.min(
      1000 * 2 ** Math.min(reconnectAttemptsRef.current, 10),
      MAX_RECONNECT_DELAY_MS,
    );
    reconnectAttemptsRef.current += 1;
    setConnectionState("reconnecting");
    reconnectTimeoutRef.current = window.setTimeout(() => {
      reconnectTimeoutRef.current = null;
      setReconnectKey(k => k + 1);
    }, delay);
  };

  const handleSocketClose = useEffectEvent((closedInstanceId: number) => {
    // Guard first: a replaced socket's close event can land after the new one
    // has opened, and clearing the heartbeat here would silence the live
    // connection until the server dropped it.
    if (closedInstanceId !== socketInstanceIdRef.current) return;
    clearHeartbeat();
    clearTimeoutRef(stableConnectionTimeoutRef);
    clearPendingSeek();
    // Nothing sent on this socket will come back.
    pendingEchoesRef.current = [];
    // A room this viewer is leaving or closing is not reconnected: its own
    // request ends it, or markRoomEndHandled(false) brings the socket back.
    if (!intentionalCloseRef.current && !roomEndHandledRef.current) {
      scheduleReconnect();
    }
  });

  // The cleanup closes the socket, which releases its listeners, and clears
  // the heartbeat interval and the reconnect and seek timeouts.
  // One room per mount: the route keys the page on the room id, so nothing
  // here carries over from another room.
  // react-doctor-disable-next-line react-doctor/effect-needs-cleanup
  useEffect(() => {
    intentionalCloseRef.current = false;
    if (!currentRoomId) return;

    let cancelled = false;
    let socket: WebSocket | null = null;
    socketInstanceIdRef.current += 1;
    const instanceId = socketInstanceIdRef.current;

    const initRoomConnection = async () => {
      let res: Awaited<ReturnType<typeof joinWatchRoom>> | null = null;
      try {
        res = await joinWatchRoom(currentRoomId);
      } catch {
        res = null;
      }

      if (cancelled) return;

      if (!res || res.error) {
        const status = res?.error ? res.status : undefined;
        // Gone or no longer ours: the room ended for this viewer.
        if (status === 403 || status === 404) {
          endRoom(status === 404 ? "deleted" : "revoked");
          return;
        }
        // The session ended; apiRequest has already signed the viewer out.
        if (status === 401) return;
        // A network failure or a server restart: try again later.
        scheduleReconnect();
        return;
      }

      socket = new WebSocket(watchRoomWebSocketUrl(currentRoomId));
      socketRef.current = socket;

      socket.addEventListener("open", handleSocketOpen);
      socket.addEventListener("message", event => {
        void handleSocketMessage(event);
      });
      socket.addEventListener("close", () => handleSocketClose(instanceId));
      // An error is always followed by close, which reconnects.
    };

    void initRoomConnection();

    return () => {
      cancelled = true;
      intentionalCloseRef.current = true;
      stopSocketOnCleanup(socket);
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

    // A new stream URL for the same room is a recovery or capacity reload,
    // which rebuilds the player at the start of the stream, paused. Without
    // the room's state re-applied the viewer sat there, and their next Play
    // broadcast that position and pulled the whole room back.
    if (!pendingPlaybackRef.current && roomPlaybackRef.current) {
      pendingPlaybackRef.current = roomPlaybackRef.current;
    }

    const handleReady = () => {
      void flushPendingPlayback();
    };

    video.addEventListener("loadedmetadata", handleReady);
    video.addEventListener("canplay", handleReady);
    return () => {
      video.removeEventListener("loadedmetadata", handleReady);
      video.removeEventListener("canplay", handleReady);
    };
  }, [streamUrl, videoRef]);

  const rememberPress = (
    type: WatchRoomPlaybackEventType,
    positionSec: number,
  ) => {
    // A seek keeps the room playing or paused as it was.
    let paused = roomPlaybackRef.current?.state.paused ?? true;
    if (type === WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY) paused = false;
    if (type === WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE) paused = true;
    roomPlaybackRef.current = {
      state: {
        paused,
        position_sec: positionSec,
        updated_at: new Date().toISOString(),
      },
      receivedAt: Date.now(),
    };
  };

  /**
   * Sends a play, pause or seek to the room. A seek waits
   * WATCH_ROOM_SEEK_COALESCE_MS for the next one, so a scrub or a held key
   * reaches the room as its final position; a play or pause carries its own
   * position and replaces a seek still waiting.
   */
  const sendPlaybackEvent = (
    type: WatchRoomPlaybackEventType,
    positionSec: number,
  ) => {
    rememberPress(type, positionSec);

    if (type !== WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK) {
      clearPendingSeek();
      sendCommand(type, positionSec);
      return;
    }

    pendingSeekRef.current = positionSec;
    clearTimeoutRef(seekTimeoutRef);
    seekTimeoutRef.current = window.setTimeout(() => {
      seekTimeoutRef.current = null;
      const position = pendingSeekRef.current;
      pendingSeekRef.current = null;
      if (position !== null) {
        sendCommand(WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK, position);
      }
    }, WATCH_ROOM_SEEK_COALESCE_MS);
  };

  // Called by a local Play before it plays, so the press starts from the
  // room's position rather than wherever the element happens to be.
  const syncToPendingPlayback = () => {
    const video = videoRef.current;
    const pending = pendingPlaybackRef.current;
    if (!video || !pending) return;

    if (video.readyState < 1) {
      // Nothing has loaded, so the position can only be applied by the flush
      // once it has. A paused state would then pause this press too, but the
      // press is newer: only the room's position carries over.
      if (pending.state.paused) {
        pendingPlaybackRef.current = { ...pending, overtakenByPlay: true };
      }
      return;
    }

    const adjustedPos = projectedPosition(pending);
    const drift = Math.abs(video.currentTime - adjustedPos);
    if (drift > WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC) {
      video.currentTime = adjustedPos;
      onCurrentTimeChange(adjustedPos);
    }
  };

  const clearPendingPlayback = () => {
    pendingPlaybackRef.current = null;
  };

  /** Where the room is now, as far as this viewer knows; null before any state. */
  const roomPosition = () => {
    const snapshot = pendingPlaybackRef.current ?? roomPlaybackRef.current;
    return snapshot ? projectedPosition(snapshot) : null;
  };

  // A pause sent before the source has loaded: the room's state (just
  // recorded as paused) is what the source takes on once it can, even if the
  // echo never comes back.
  const holdPausedRoomPlayback = () => {
    pendingPlaybackRef.current = roomPlaybackRef.current;
  };

  /**
   * This viewer ended the room themselves (closed or left it): the socket's
   * own room_deleted or room_access_revoked must not end it a second time.
   * `false` undoes that after the request failed.
   */
  const markRoomEndHandled = (handled: boolean) => {
    roomEndHandledRef.current = handled;
    // The request failed and the room goes on: if its socket closed meanwhile,
    // connect again.
    const socket = socketRef.current;
    const socketGone = !socket || socket.readyState === WebSocket.CLOSED;
    if (!handled && socketGone && !intentionalCloseRef.current) {
      setReconnectKey(k => k + 1);
    }
  };

  return {
    playbackError,
    setPlaybackError,
    syncAnnouncement,
    syncAnnouncementKey,
    connectedUserIds,
    connectionState,
    closeRoomConnection,
    sendPlaybackEvent,
    syncToPendingPlayback,
    clearPendingPlayback,
    roomPosition,
    holdPausedRoomPlayback,
    markRoomEndHandled,
  };
}
