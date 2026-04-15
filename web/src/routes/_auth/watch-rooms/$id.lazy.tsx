import { useEffect, useMemo, useRef, useState } from "react";
import { createLazyFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  AlertCircle,
  ArrowLeft,
  FastForward,
  Pause,
  Play,
  Radio,
  Rewind,
  Trash2,
  Users,
} from "lucide-react";
import { joinWatchRoom } from "@/lib/api";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { formatTimeSeconds } from "@/lib/format";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import { showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import DeleteWatchRoomDialog from "@/components/DeleteWatchRoomDialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import VideoPlayer from "@/components/VideoPlayer";
import type { WatchRoomMemberType } from "@/types";

export const Route = createLazyFileRoute("/_auth/watch-rooms/$id")({
  component: WatchRoomPage,
});

const SEEK_STEP_SEC = 10;
const ROOM_SYNC_DRIFT_THRESHOLD_SEC = 1.5;
const ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS = 1200;

type RoomPlaybackState = {
  paused: boolean;
  position_sec: number;
  updated_at: string;
};

type RoomServerEvent = {
  type: "room_snapshot" | "playback_changed" | "member_joined" | "member_left" | "room_deleted" | "pong";
  room_id: number;
  playback?: RoomPlaybackState;
  member?: WatchRoomMemberType;
  connected_user_ids?: number[];
};

function roomStreamUrl(roomId: number, playbackMode: string) {
  if (playbackMode === "direct") {
    return `/api/watch-rooms/${roomId}/stream`;
  }
  return `/api/watch-rooms/${roomId}/hls/playlist.m3u8`;
}

function roomWebSocketUrl(roomId: number) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/api/watch-rooms/${roomId}/ws`;
}

function watchRoomAnnouncement(
  event: RoomServerEvent,
  title: string,
): string | undefined {
  switch (event.type) {
    case "playback_changed":
      if (!event.playback) return undefined;
      if (event.playback.paused) {
        return `Paused ${title}`;
      }
      return `Playing ${title}`;
    case "member_joined":
      return event.member ? `${event.member.name} joined the room` : undefined;
    case "member_left":
      return event.member ? `${event.member.name} left the room` : undefined;
    default:
      return undefined;
  }
}

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
  const videoRef = useRef<HTMLVideoElement>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const announcementTimeoutRef = useRef<number | null>(null);
  const roomDeletionHandledRef = useRef(false);

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [syncAnnouncement, setSyncAnnouncement] = useState<string | undefined>(
    undefined,
  );
  const [connectedUserIds, setConnectedUserIds] = useState<number[]>([]);
  const [connectionReady, setConnectionReady] = useState(false);
  const [joinReady, setJoinReady] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const { data, isPending, isError } = useQuery(watchRoomQueryOpts(roomId));
  const room = data?.error === false ? data.data.room : null;
  const streamUrl = room ? roomStreamUrl(room.id, room.playback_mode) : "";
  const posterUrl =
    room?.movie_poster != null
      ? buildTmdbImageUrl(room.movie_poster, TMDB_POSTER_SIZE)
      : null;

  const subtitleTrack = useMemo(() => {
    if (!room || room.subtitle_track === null) return null;
    return {
      url: `/api/movies/${room.movie_id}/subtitles/${room.subtitle_track}/web.vtt`,
      label: `Subtitle track ${room.subtitle_track + 1}`,
      srclang: "",
    };
  }, [room]);

  const closeRoomConnection = () => {
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

  useEffect(() => {
    if (!room || joinReady) return;

    let cancelled = false;
    const joinRoom = async () => {
      const res = await joinWatchRoom(room.id);
      if (cancelled) return;

      if (res.error) {
        setPlaybackError(res.message || "Unable to join this watch room.");
        return;
      }

      setJoinReady(true);
    };

    void joinRoom();
    return () => {
      cancelled = true;
    };
  }, [joinReady, room]);

  useEffect(() => {
    if (!room || !joinReady) return;

    const socket = new WebSocket(roomWebSocketUrl(room.id));
    socketRef.current = socket;

    socket.addEventListener("open", () => {
      setConnectionReady(true);
      socket.send(JSON.stringify({ type: "join" }));
      heartbeatRef.current = window.setInterval(() => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify({ type: "ping" }));
        }
      }, 25_000);
    });

    socket.addEventListener("message", async rawEvent => {
      let event: RoomServerEvent;
      try {
        event = JSON.parse(rawEvent.data) as RoomServerEvent;
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
          `"${room.movie_title}" is no longer available.`,
        );
        navigate({ to: "/", replace: true });
        return;
      }

      if (event.connected_user_ids) {
        setConnectedUserIds(event.connected_user_ids);
      }

      const announcement = watchRoomAnnouncement(event, room.movie_title);
      if (announcement) {
        if (announcementTimeoutRef.current !== null) {
          window.clearTimeout(announcementTimeoutRef.current);
        }
        setSyncAnnouncement(announcement);
        announcementTimeoutRef.current = window.setTimeout(() => {
          setSyncAnnouncement(undefined);
          announcementTimeoutRef.current = null;
        }, ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS);
      }

      if (!event.playback) return;

      const video = videoRef.current;
      if (!video) return;

      const targetTime = event.playback.position_sec;
      const drift = Math.abs(video.currentTime - targetTime);
      if (drift > ROOM_SYNC_DRIFT_THRESHOLD_SEC) {
        video.currentTime = targetTime;
        setCurrentTime(targetTime);
      }

      if (event.playback.paused) {
        if (!video.paused) {
          video.pause();
        }
        setPlaying(false);
      } else {
        if (video.paused) {
          try {
            await video.play();
          } catch {
            setPlaybackError(
              "Playback failed while syncing with the room. Try reloading the room.",
            );
            return;
          }
        }
        setPlaying(true);
      }
    });

    socket.addEventListener("close", () => {
      setConnectionReady(false);
      if (heartbeatRef.current !== null) {
        window.clearInterval(heartbeatRef.current);
        heartbeatRef.current = null;
      }
    });

    socket.addEventListener("error", () => {
      setPlaybackError("Realtime sync connection failed for this watch room.");
    });

    return () => {
      if (heartbeatRef.current !== null) {
        window.clearInterval(heartbeatRef.current);
        heartbeatRef.current = null;
      }
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      if (
        socket.readyState !== WebSocket.CLOSING &&
        socket.readyState !== WebSocket.CLOSED
      ) {
        socket.close();
      }
    };
  }, [joinReady, navigate, room]);

  useEffect(() => {
    return () => {
      if (announcementTimeoutRef.current !== null) {
        window.clearTimeout(announcementTimeoutRef.current);
      }
    };
  }, []);

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

  const togglePlay = async () => {
    const video = videoRef.current;
    if (!video) return;

    if (video.paused) {
      try {
        await video.play();
        sendPlaybackEvent("play", video.currentTime);
      } catch {
        setPlaybackError("Playback failed — the browser could not play this stream.");
      }
      return;
    }

    video.pause();
    sendPlaybackEvent("pause", video.currentTime);
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
    sendPlaybackEvent("seek", nextTime);
  };

  const seekBackward = () => seek(currentTime - SEEK_STEP_SEC);
  const seekForward = () => seek(currentTime + SEEK_STEP_SEC);
  const handleOwnerDeleteSuccess = () => {
    closeRoomConnection();
    navigate({ to: "/", replace: true });
  };

  const connectedMembers = room
    ? room.members.filter(member => connectedUserIds.includes(member.id))
    : [];

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
    <div className="min-h-screen bg-slate-950 text-white">
      <title>{room.movie_title} Watch Room - Igloo</title>
      <meta
        name="description"
        content={`Watch ${room.movie_title} together in a shared synchronized room.`}
      />

      <LiveAnnouncer message={syncAnnouncement} />

      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
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

        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <section className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90">
            <div className="flex min-h-[50vh] flex-col">
              <VideoPlayer
                videoRef={videoRef}
                src={streamUrl}
                title={room.movie_title}
                onError={setPlaybackError}
                onPlay={() => setPlaying(true)}
                onPause={() => setPlaying(false)}
                onEnded={() => {
                  setPlaying(false);
                  sendPlaybackEvent("pause", duration);
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
