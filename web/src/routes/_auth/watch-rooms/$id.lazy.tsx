import { useEffect, useEffectEvent, useRef, useState } from "react";
import { createLazyFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  AlertCircle,
  ArrowLeft,
  FastForward,
  Maximize,
  Minimize,
  Pause,
  Play,
  Radio,
  Rewind,
  Trash2,
  Users,
} from "lucide-react";
import { joinWatchRoom } from "@/lib/api";
import {
  TMDB_POSTER_SIZE,
  WATCH_ROOM_SEEK_STEP_SEC,
  WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS,
  WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC,
} from "@/lib/constants";
import { formatTimeSeconds } from "@/lib/format";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  requestElementFullscreen,
  tryWebKitVideoEnterFullscreen,
  tryWebKitVideoExitFullscreen,
} from "@/lib/fullscreen";
import {
  watchRoomAnnouncement,
  watchRoomStreamUrl,
  watchRoomWebSocketUrl,
} from "@/lib/watch-room";
import { showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import DeleteWatchRoomDialog from "@/components/DeleteWatchRoomDialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import VideoPlayer from "@/components/VideoPlayer";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import type {
  WatchRoomPlaybackStateType,
  WatchRoomServerEventType,
} from "@/types";

export const Route = createLazyFileRoute("/_auth/watch-rooms/$id")({
  component: WatchRoomPage,
});

export function WatchRoomPage() {
  const { id } = Route.useParams();
  const roomId = Number.parseInt(id, 10);
  return <WatchRoomPageContent roomId={roomId} />;
}

type WatchRoomPageContentProps = {
  roomId: number;
};

export function WatchRoomPageContent({
  roomId,
}: WatchRoomPageContentProps) {
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const announcementTimeoutRef = useRef<number | null>(null);
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
  const fullscreenSourceRef = useRef<"none" | "document" | "webkitVideo">(
    "none",
  );

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [syncAnnouncement, setSyncAnnouncement] = useState<string | undefined>(
    undefined,
  );
  const [connectedUserIds, setConnectedUserIds] = useState<number[]>([]);
  const [connectionReady, setConnectionReady] = useState(false);
  const [reconnectKey, setReconnectKey] = useState(0);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isImmersiveViewport, setIsImmersiveViewport] = useState(false);

  const { data, isPending, isError } = useQuery(watchRoomQueryOpts(roomId));
  const room = data?.error === false ? data.data.room : null;
  const currentRoomId = room?.id ?? null;
  const movieTitle = room?.movie_title ?? "";
  const streamUrl = room
    ? watchRoomStreamUrl(room.id, room.playback_mode)
    : "";
  const posterUrl =
    room?.movie_poster != null
      ? buildTmdbImageUrl(room.movie_poster, TMDB_POSTER_SIZE)
      : null;

  const subtitleTrack =
    room && room.subtitle_track !== null
      ? {
          url: `/api/movies/${room.movie_id}/subtitles/${room.subtitle_track}/web.vtt`,
          label: `Subtitle track ${room.subtitle_track + 1}`,
          srclang: "",
        }
      : null;

  const closeRoomConnection = () => {
    intentionalCloseRef.current = true;

    if (reconnectTimeoutRef.current !== null) {
      window.clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (heartbeatRef.current !== null) {
      window.clearInterval(heartbeatRef.current);
      heartbeatRef.current = null;
    }

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
        setCurrentTime(targetTime);
        if (!video.paused) {
          video.pause();
        }
        setPlaying(false);
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
        setPlaying(true);
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

  const handleSocketMessage = useEffectEvent(
    async (rawEvent: MessageEvent) => {
      let event: WatchRoomServerEventType;
      try {
        event = JSON.parse(rawEvent.data) as WatchRoomServerEventType;
      } catch {
        return;
      }

      if (event.type === "room_deleted") {
        if (roomDeletionHandledRef.current) {
          return;
        }

        roomDeletionHandledRef.current = true;
        closeRoomConnection();
        showInfo(
          "Watch room closed",
          `"${movieTitle}" is no longer available.`,
        );
        navigate({ to: "/", replace: true });
        return;
      }

      if (event.connected_user_ids) {
        setConnectedUserIds(event.connected_user_ids);
      }

      const announcement = watchRoomAnnouncement(event, movieTitle);
      if (announcement) {
        if (announcementTimeoutRef.current !== null) {
          window.clearTimeout(announcementTimeoutRef.current);
        }
        setSyncAnnouncement(announcement);
        announcementTimeoutRef.current = window.setTimeout(() => {
          setSyncAnnouncement(undefined);
          announcementTimeoutRef.current = null;
        }, WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS);
      }

      if (!event.playback) return;

      pendingPlaybackRef.current = { state: event.playback, receivedAt: Date.now() };
      await flushPendingPlayback();
    },
  );

  const MAX_RECONNECT_ATTEMPTS = 5;

  const handleSocketOpen = useEffectEvent(() => {
    setConnectionReady(true);
    reconnectAttemptsRef.current = 0;
    const ws = socketRef.current;
    if (!ws) return;
    heartbeatRef.current = window.setInterval(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "ping" }));
      }
    }, 25_000);
  });

  const handleSocketClose = useEffectEvent((closedInstanceId: number) => {
    setConnectionReady(false);
    if (closedInstanceId !== socketInstanceIdRef.current) return;
    if (
      !intentionalCloseRef.current &&
      reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS
    ) {
      const delay = Math.min(1000 * 2 ** reconnectAttemptsRef.current, 16_000);
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

      if (heartbeatRef.current !== null) {
        window.clearInterval(heartbeatRef.current);
        heartbeatRef.current = null;
      }

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
    return () => {
      if (announcementTimeoutRef.current !== null) {
        window.clearTimeout(announcementTimeoutRef.current);
      }
    };
  }, []);

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
  }, [streamUrl, currentRoomId]);

  useEffect(() => {
    const onFullscreenChange = () => {
      const entering = !!getFullscreenElement();
      if (entering) {
        fullscreenSourceRef.current = "document";
        setIsFullscreen(true);
        setIsImmersiveViewport(false);
      } else {
        if (fullscreenSourceRef.current === "document") {
          fullscreenSourceRef.current = "none";
        }
        setIsFullscreen(false);
      }
    };

    document.addEventListener("fullscreenchange", onFullscreenChange);
    document.addEventListener("webkitfullscreenchange", onFullscreenChange);

    return () => {
      document.removeEventListener("fullscreenchange", onFullscreenChange);
      document.removeEventListener(
        "webkitfullscreenchange",
        onFullscreenChange,
      );
    };
  }, []);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onWebKitBegin = () => {
      fullscreenSourceRef.current = "webkitVideo";
      setIsFullscreen(true);
      setIsImmersiveViewport(false);
    };
    const onWebKitEnd = () => {
      fullscreenSourceRef.current = "none";
      setIsFullscreen(false);
    };

    video.addEventListener("webkitbeginfullscreen", onWebKitBegin);
    video.addEventListener("webkitendfullscreen", onWebKitEnd);

    return () => {
      video.removeEventListener("webkitbeginfullscreen", onWebKitBegin);
      video.removeEventListener("webkitendfullscreen", onWebKitEnd);
    };
  }, []);

  useEffect(() => {
    if (!isImmersiveViewport) return;

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [isImmersiveViewport]);

  const sendPlaybackEvent = (type: "play" | "pause" | "seek", positionSec: number) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(
      JSON.stringify({
        type,
        position_sec: positionSec,
      }),
    );
  };

  const playVideo = async () => {
    const video = videoRef.current;
    if (!video) return;

    const pending = pendingPlaybackRef.current;
    if (pending && !pending.state.paused) {
      const elapsed = (Date.now() - pending.receivedAt) / 1000;
      const adjustedPos = pending.state.position_sec + elapsed;
      const drift = Math.abs(video.currentTime - adjustedPos);
      if (video.readyState >= 1 && drift > WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC) {
        video.currentTime = adjustedPos;
        setCurrentTime(adjustedPos);
      }
    }

    try {
      await video.play();
      setPlaybackError(null);
      pendingPlaybackRef.current = null;
      sendPlaybackEvent("play", video.currentTime);
    } catch {
      setPlaybackError("Playback failed — the browser could not play this stream.");
    }
  };

  const pauseVideo = () => {
    const video = videoRef.current;
    if (!video) return;

    video.pause();
    pendingPlaybackRef.current = null;
    sendPlaybackEvent("pause", video.currentTime);
  };

  const togglePlay = async () => {
    const video = videoRef.current;
    if (!video) return;

    if (video.paused) {
      await playVideo();
      return;
    }

    pauseVideo();
  };

  const seek = (newTime: number) => {
    const video = videoRef.current;
    if (!video) return;

    const safeDuration = Number.isFinite(video.duration) && video.duration > 0
      ? video.duration
      : duration;
    const nextTime = Math.max(0, Math.min(newTime, safeDuration || newTime));
    video.currentTime = nextTime;
    setCurrentTime(nextTime);
    pendingPlaybackRef.current = null;
    sendPlaybackEvent("seek", nextTime);
  };

  const seekBackward = () =>
    seek(currentTime - WATCH_ROOM_SEEK_STEP_SEC);
  const seekForward = () =>
    seek(currentTime + WATCH_ROOM_SEEK_STEP_SEC);

  useVideoMediaSession({
    videoRef,
    title: movieTitle || "Watch room",
    artworkUrl: posterUrl,
    currentTime,
    duration,
    playing,
    seekStepSec: WATCH_ROOM_SEEK_STEP_SEC,
    onPlay: playVideo,
    onPause: pauseVideo,
    onSeek: seek,
    enabled: !!room && !playbackError,
  });

  const toggleFullscreen = async () => {
    const container = containerRef.current;
    const video = videoRef.current;
    if (!container || !video) return;

    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return;
    }

    if (fullscreenSourceRef.current === "webkitVideo") {
      tryWebKitVideoExitFullscreen(video);
      return;
    }

    if (isImmersiveViewport) {
      setIsImmersiveViewport(false);
      return;
    }

    const enterFallback = () => {
      if (tryWebKitVideoEnterFullscreen(video)) {
        return;
      }

      setIsImmersiveViewport(true);
    };

    if (
      !canRequestElementFullscreen(container) ||
      !isDocumentFullscreenEntryLikely()
    ) {
      enterFallback();
      return;
    }

    try {
      await requestElementFullscreen(container);
    } catch {
      enterFallback();
    }
  };

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      const container = containerRef.current;
      const targetInsidePlayer = container?.contains(target) ?? false;
      const targetIsPageBody =
        target === document.body || target === document.documentElement;

      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.tagName === "SELECT" ||
        target.isContentEditable
      ) {
        return;
      }

      if (!targetInsidePlayer && !targetIsPageBody) {
        return;
      }

      if (event.ctrlKey || event.metaKey || event.altKey) {
        return;
      }

      switch (event.key) {
        case "f":
        case "F":
          event.preventDefault();
          event.stopPropagation();
          void toggleFullscreen();
          break;
        case "Escape":
          if (getFullscreenElement()) {
            event.preventDefault();
            event.stopPropagation();
            void exitDocumentFullscreen();
            break;
          }

          if (
            fullscreenSourceRef.current === "webkitVideo" &&
            tryWebKitVideoExitFullscreen(videoRef.current)
          ) {
            event.preventDefault();
            event.stopPropagation();
            break;
          }

          if (isImmersiveViewport) {
            event.preventDefault();
            event.stopPropagation();
            setIsImmersiveViewport(false);
          }
          break;
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
    // React Compiler memoizes toggleFullscreen; ESLint cannot see that, so suppress.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isImmersiveViewport]);

  const handleOwnerDeleteSuccess = () => {
    closeRoomConnection();
    navigate({ to: "/", replace: true });
  };

  const connectedMembers = room
    ? room.members.filter(member => connectedUserIds.includes(member.id))
    : [];
  const playerFullscreenMode = isFullscreen || isImmersiveViewport;

  if (isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-900 px-4">
        <div className="text-center">
          <Spinner className="mx-auto mb-4 size-10 text-amber-400" />
          <p className="text-lg font-medium text-white">Loading watch room...</p>
        </div>
      </div>
    );
  }

  if (isError || (data && data.error) || !room) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-900 px-4">
        <div className="max-w-md text-center">
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">Watch room unavailable</h1>
          <p className="mb-6 text-slate-400">
            {data?.message || "This watch room could not be loaded or you do not have access to it."}
          </p>
          <button
            type="button"
            onClick={() => navigate({ to: "/" })}
            className="inline-flex items-center gap-2 rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
          >
            <ArrowLeft className="size-5" aria-hidden="true" />
            Back home
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className={cn(
        "min-h-screen bg-slate-950 text-white [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen",
        isImmersiveViewport &&
          "fixed inset-0 z-50 min-h-dvh w-full overflow-auto bg-slate-950",
      )}
    >
      <title>{room.movie_title} Watch Room - Igloo</title>
      <meta
        name="description"
        content={`Watch ${room.movie_title} together in a shared synchronized room.`}
      />

      <LiveAnnouncer message={syncAnnouncement} />
      <p className="sr-only">
        Keyboard shortcuts: F for fullscreen and Escape to exit fullscreen.
      </p>

      <div
        className={cn(
          "mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8",
          playerFullscreenMode && "max-w-none",
        )}
      >
        <header className="flex flex-col gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <p className="flex items-center gap-2 text-xs font-semibold tracking-[0.2em] text-amber-300 uppercase">
                <Radio className="size-3.5" aria-hidden="true" />
                Shared Watch Room
              </p>
              <h1 className="mt-2 text-2xl font-semibold tracking-tight text-white md:text-3xl">
                {room.movie_title}
              </h1>
              <p className="mt-2 text-sm text-slate-400">
                Hosted by {room.owner.name}. Everyone in this room shares the same playback.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              {room.is_owner ? (
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  className="bg-red-600 text-white hover:bg-red-700"
                  aria-label={`Close watch room for ${room.movie_title}`}
                  onClick={() => setDeleteDialogOpen(true)}
                >
                  <Trash2 className="size-4" aria-hidden="true" />
                  Close room
                </Button>
              ) : null}

              <button
                type="button"
                onClick={() => navigate({ to: "/" })}
                className="inline-flex items-center gap-2 rounded-full border border-slate-700 px-4 py-2 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
              >
                <ArrowLeft className="size-4" aria-hidden="true" />
                Leave room
              </button>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3 text-sm text-slate-300">
            <span className="rounded-full border border-slate-700 bg-slate-950/60 px-3 py-1">
              {connectionReady ? "Realtime sync connected" : "Connecting realtime sync..."}
            </span>
            <span className="rounded-full border border-slate-700 bg-slate-950/60 px-3 py-1">
              {connectedMembers.length} connected now
            </span>
          </div>
        </header>

        <div
          className={cn(
            "grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]",
            playerFullscreenMode && "grid-cols-1",
          )}
        >
          <section className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90">
            <div
              className={cn(
                "flex min-h-[50vh] flex-col",
                playerFullscreenMode && "min-h-dvh",
              )}
            >
              <VideoPlayer
                videoRef={videoRef}
                src={streamUrl}
                title={room.movie_title}
                isFullscreen={playerFullscreenMode}
                onError={setPlaybackError}
                onPlay={() => setPlaying(true)}
                onPause={() => setPlaying(false)}
                onEnded={() => {
                  setPlaying(false);
                  const video = videoRef.current;
                  sendPlaybackEvent("pause", video ? video.currentTime : duration);
                }}
                onTimeUpdate={setCurrentTime}
                onDurationChange={setDuration}
                subtitleTrack={subtitleTrack}
              />

              <div className="border-t border-slate-800 p-4 sm:p-5">
                <ProgressBar
                  currentTime={currentTime}
                  duration={duration}
                  onSeek={seek}
                  variant="video"
                />

                <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={seekBackward}
                      className="inline-flex size-11 items-center justify-center rounded-full border border-slate-700 bg-slate-950/60 text-slate-200 transition-colors hover:bg-slate-800 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                      aria-label="Rewind 10 seconds"
                    >
                      <Rewind className="size-5" aria-hidden="true" />
                    </button>

                    <button
                      type="button"
                      onClick={() => void togglePlay()}
                      className="inline-flex size-13 items-center justify-center rounded-full bg-amber-500 text-slate-900 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                      aria-label={playing ? "Pause playback" : "Play playback"}
                    >
                      {playing ? (
                        <Pause className="size-6" aria-hidden="true" />
                      ) : (
                        <Play className="size-6 fill-current" aria-hidden="true" />
                      )}
                    </button>

                    <button
                      type="button"
                      onClick={seekForward}
                      className="inline-flex size-11 items-center justify-center rounded-full border border-slate-700 bg-slate-950/60 text-slate-200 transition-colors hover:bg-slate-800 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                      aria-label="Fast-forward 10 seconds"
                    >
                      <FastForward className="size-5" aria-hidden="true" />
                    </button>
                  </div>

                  <div className="flex items-center gap-4">
                    <p className="text-sm font-medium text-slate-300">
                      {formatTimeSeconds(currentTime)} / {formatTimeSeconds(duration)}
                    </p>
                    <VolumeControl mediaRef={videoRef} />
                    <button
                      type="button"
                      onClick={() => void toggleFullscreen()}
                      className="inline-flex size-11 items-center justify-center rounded-full border border-slate-700 bg-slate-950/60 text-slate-200 transition-colors hover:bg-slate-800 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                      aria-label={
                        playerFullscreenMode
                          ? isImmersiveViewport && !isFullscreen
                            ? "Exit expanded view"
                            : "Exit fullscreen"
                          : "Fullscreen"
                      }
                      aria-pressed={playerFullscreenMode}
                    >
                      {playerFullscreenMode ? (
                        <Minimize className="size-5" aria-hidden="true" />
                      ) : (
                        <Maximize className="size-5" aria-hidden="true" />
                      )}
                    </button>
                  </div>
                </div>

                {playbackError && (
                  <div className="mt-4 rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                    {playbackError}
                  </div>
                )}
              </div>
            </div>
          </section>

          {!playerFullscreenMode ? (
            <aside className="rounded-2xl border border-slate-800 bg-slate-900/90 p-4 sm:p-5">
            <h2 className="flex items-center gap-2 text-lg font-semibold text-white">
              <Users className="size-5 text-amber-400" aria-hidden="true" />
              People in this room
            </h2>

            {posterUrl && (
              <img
                src={posterUrl}
                alt=""
                className="mt-4 aspect-2/3 w-28 rounded-xl border border-slate-800 object-cover"
              />
            )}

            <ul className="mt-4 space-y-3">
              {room.members.map(member => {
                const isConnected = connectedUserIds.includes(member.id);
                return (
                  <li
                    key={member.id}
                    className="flex items-center justify-between gap-3 rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-slate-100">
                        {member.name}
                        {member.id === room.owner.id ? (
                          <span className="ml-2 text-xs text-amber-300">Host</span>
                        ) : null}
                      </p>
                    </div>
                    <span
                      className={cn(
                        "rounded-full px-2 py-1 text-xs font-medium",
                        isConnected
                          ? "bg-emerald-500/15 text-emerald-300"
                          : "bg-slate-800 text-slate-400",
                      )}
                    >
                      {isConnected ? "Connected" : "Away"}
                    </span>
                  </li>
                );
              })}
            </ul>
            </aside>
          ) : null}
        </div>
      </div>

      {room.is_owner ? (
        <DeleteWatchRoomDialog
          roomId={room.id}
          movieTitle={room.movie_title}
          open={deleteDialogOpen}
          onOpenChange={setDeleteDialogOpen}
          onDeleteStart={() => {
            roomDeletionHandledRef.current = true;
          }}
          onDeleteError={() => {
            roomDeletionHandledRef.current = false;
          }}
          onDeleted={handleOwnerDeleteSuccess}
        />
      ) : null}
    </div>
  );
}
