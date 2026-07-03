import { useRef, useState } from "react";
import type { RefObject } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
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
import {
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  TMDB_POSTER_SIZE,
  WATCH_ROOM_SEEK_STEP_SEC,
} from "@/lib/constants";
import { formatTimeSeconds } from "@/lib/format";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import {
  movieTechnicalDetailsQueryOpts,
  watchRoomQueryOpts,
} from "@/lib/query-opts";
import { buildMovieSubtitleTrackInfo } from "@/lib/movie-playback";
import { watchRoomStreamUrl } from "@/lib/watch-room";
import { cn } from "@/lib/utils";
import DeleteWatchRoomDialog from "@/components/DeleteWatchRoomDialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import VideoPlayer from "@/components/VideoPlayer";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { useWatchRoomConnection } from "./useWatchRoomConnection";
import type { WatchRoomDetailType, WatchRoomMemberType } from "@/types";

type WatchRoomPageProps = {
  roomId: number;
};

export function WatchRoomPage({ roomId }: WatchRoomPageProps) {
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const deleteButtonRef = useRef<HTMLButtonElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode: playerFullscreenMode,
    toggleFullscreen,
    exitFullscreenIfActive,
  } = useVideoFullscreen({ containerRef, videoRef });

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

  const {
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
  } = useWatchRoomConnection({
    currentRoomId,
    movieTitle,
    streamUrl,
    videoRef,
    onCurrentTimeChange: setCurrentTime,
    onPlayingChange: setPlaying,
    onRoomDeleted: () => navigate({ to: "/", replace: true }),
  });

  const subtitleTrackIndex = room?.subtitle_track ?? null;
  const techDetailsOpts = movieTechnicalDetailsQueryOpts(room?.movie_id ?? 0);
  const { data: techData } = useQuery({
    ...techDetailsOpts,
    enabled: (room?.movie_id ?? 0) > 0 && subtitleTrackIndex !== null,
  });
  const techLoaded = techData?.data != null;
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const subtitleTrack = room
    ? buildMovieSubtitleTrackInfo({
        movieId: room.movie_id,
        resolvedSubtitleTrack: subtitleTrackIndex,
        techLoaded,
        subtitleStreams,
      })
    : null;

  const playVideo = async () => {
    const video = videoRef.current;
    if (!video) return;

    syncToPendingPlayback();

    try {
      await video.play();
      setPlaybackError(null);
      clearPendingPlayback();
      sendPlaybackEvent("play", video.currentTime);
    } catch {
      setPlaybackError(
        "Playback failed - the browser could not play this stream.",
      );
    }
  };

  const pauseVideo = () => {
    const video = videoRef.current;
    if (!video) return;

    video.pause();
    clearPendingPlayback();
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

    const safeDuration =
      Number.isFinite(video.duration) && video.duration > 0
        ? video.duration
        : duration;
    const nextTime = Math.max(0, Math.min(newTime, safeDuration || newTime));
    video.currentTime = nextTime;
    setCurrentTime(nextTime);
    clearPendingPlayback();
    sendPlaybackEvent("seek", nextTime);
  };

  const seekBackward = () => seek(currentTime - WATCH_ROOM_SEEK_STEP_SEC);
  const seekForward = () => seek(currentTime + WATCH_ROOM_SEEK_STEP_SEC);

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

  useVideoPlaybackKeyboard({
    containerRef,
    videoRef,
    enabled: !!room,
    onTogglePlay: () => void togglePlay(),
    onSeekBackward: seekBackward,
    onSeekForward: seekForward,
    onSeekToStart: () => seek(0),
    onToggleFullscreen: () => void toggleFullscreen(),
    onEscape: exitFullscreenIfActive,
  });

  const handleOwnerDeleteSuccess = () => {
    closeRoomConnection();
    navigate({ to: "/", replace: true });
  };

  if (isPending) {
    return <WatchRoomLoading />;
  }

  if (isError || (data && data.error) || !room) {
    return (
      <WatchRoomUnavailable
        message={
          data?.message ||
          "This watch room could not be loaded or you do not have access to it."
        }
        onBackHome={() => navigate({ to: "/" })}
      />
    );
  }

  const connectedMembers = room.members.filter(member =>
    connectedUserIds.includes(member.id),
  );

  return (
    <div
      ref={containerRef}
      className={cn(
        "min-h-screen bg-background text-foreground [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen",
        isImmersiveViewport &&
          "fixed inset-0 z-50 min-h-dvh w-full overflow-auto bg-background",
      )}
    >
      <title>{room.movie_title} Watch Room - Igloo</title>
      <meta
        name="description"
        content={`Watch ${room.movie_title} together in a shared synchronized room.`}
      />

      <LiveAnnouncer message={syncAnnouncement} />
      <p className="sr-only">
        Keyboard shortcuts: Space or K to play or pause, J or Left Arrow to
        rewind, L or Right Arrow to fast-forward, Home or 0 to restart, F for
        fullscreen, M to mute, Up or Down Arrow to adjust volume, and Escape to
        exit fullscreen.
      </p>

      <div
        className={cn(
          "mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8",
          playerFullscreenMode && "max-w-none",
        )}
      >
          <WatchRoomHeader
            room={room}
            connectedCount={connectedMembers.length}
            connectionReady={connectionReady}
            deleteButtonRef={deleteButtonRef}
            onDelete={() => setDeleteDialogOpen(true)}
            onLeave={() => navigate({ to: "/" })}
          />

        <div
          className={cn(
            "grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]",
            playerFullscreenMode && "grid-cols-1",
          )}
        >
          <WatchRoomPlayerPanel
            room={room}
            streamUrl={streamUrl}
            subtitleTrack={subtitleTrack}
            videoRef={videoRef}
            playing={playing}
            currentTime={currentTime}
            duration={duration}
            playbackError={playbackError}
            playerFullscreenMode={playerFullscreenMode}
            isFullscreen={isFullscreen}
            isImmersiveViewport={isImmersiveViewport}
            onError={setPlaybackError}
            onPlayingChange={setPlaying}
            onTimeUpdate={setCurrentTime}
            onDurationChange={setDuration}
            onEnded={() => {
              setPlaying(false);
              const video = videoRef.current;
              sendPlaybackEvent("pause", video ? video.currentTime : duration);
            }}
            onSeek={seek}
            onSeekBackward={seekBackward}
            onSeekForward={seekForward}
            onTogglePlay={() => void togglePlay()}
            onToggleFullscreen={() => void toggleFullscreen()}
          />

          {!playerFullscreenMode ? (
            <WatchRoomMembersPanel
              members={room.members}
              ownerId={room.owner.id}
              posterUrl={posterUrl}
              connectedUserIds={connectedUserIds}
            />
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
            markRoomDeletionHandled(true);
          }}
          onDeleteError={() => {
            markRoomDeletionHandled(false);
          }}
          onDeleted={handleOwnerDeleteSuccess}
          restoreFocusRef={deleteButtonRef}
        />
      ) : null}
    </div>
  );
}

function WatchRoomLoading() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-card px-4">
      <div className="text-center">
        <Spinner className="mx-auto mb-4 size-10 text-primary" />
        <p className="text-lg font-medium text-foreground">Loading watch room...</p>
      </div>
    </div>
  );
}

type WatchRoomUnavailableProps = {
  message: string;
  onBackHome: () => void;
};

export function WatchRoomUnavailable({
  message,
  onBackHome,
}: WatchRoomUnavailableProps) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-card px-4">
      <div className="max-w-md text-center">
        <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-destructive/10">
          <AlertCircle className="size-10 text-destructive" aria-hidden="true" />
        </div>
        <h1 className="mb-2 text-xl font-semibold text-foreground">
          Watch room unavailable
        </h1>
        <p className="mb-6 text-muted-foreground">{message}</p>
        <button
          type="button"
          onClick={onBackHome}
          className="inline-flex items-center gap-2 rounded-full bg-primary px-6 py-3 font-semibold text-primary-foreground transition-colors hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none"
        >
          <ArrowLeft className="size-5" aria-hidden="true" />
          Back home
        </button>
      </div>
    </div>
  );
}

type WatchRoomHeaderProps = {
  room: WatchRoomDetailType;
  connectedCount: number;
  connectionReady: boolean;
  deleteButtonRef: RefObject<HTMLButtonElement | null>;
  onDelete: () => void;
  onLeave: () => void;
};

function WatchRoomHeader({
  room,
  connectedCount,
  connectionReady,
  deleteButtonRef,
  onDelete,
  onLeave,
}: WatchRoomHeaderProps) {
  return (
    <header className="flex flex-col gap-4 rounded-2xl border border-border bg-card/90 p-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="flex items-center gap-2 text-xs font-semibold tracking-[0.2em] text-primary uppercase">
            <Radio className="size-3.5" aria-hidden="true" />
            Shared Watch Room
          </p>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
            {room.movie_title}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Hosted by {room.owner.name}. Everyone in this room shares the same
            playback.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          {room.is_owner ? (
            <Button
              ref={deleteButtonRef}
              type="button"
              variant="destructive"
              size="sm"
              aria-label={`Close watch room for ${room.movie_title}`}
              onClick={onDelete}
            >
              <Trash2 className="size-4" aria-hidden="true" />
              Close room
            </Button>
          ) : null}

          <button
            type="button"
            onClick={onLeave}
            className="inline-flex items-center gap-2 rounded-full border border-border px-4 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            Leave room
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
        <span className="rounded-full border border-border bg-background/60 px-3 py-1">
          {connectionReady
            ? "Realtime sync connected"
            : "Connecting realtime sync..."}
        </span>
        <span className="rounded-full border border-border bg-background/60 px-3 py-1">
          {connectedCount} connected now
        </span>
      </div>
    </header>
  );
}

type WatchRoomPlayerPanelProps = {
  room: WatchRoomDetailType;
  streamUrl: string;
  subtitleTrack: {
    url: string;
    label: string;
    srclang: string;
  } | null;
  videoRef: RefObject<HTMLVideoElement | null>;
  playing: boolean;
  currentTime: number;
  duration: number;
  playbackError: string | null;
  playerFullscreenMode: boolean;
  isFullscreen: boolean;
  isImmersiveViewport: boolean;
  onError: (error: string | null) => void;
  onPlayingChange: (playing: boolean) => void;
  onTimeUpdate: (time: number) => void;
  onDurationChange: (duration: number) => void;
  onEnded: () => void;
  onSeek: (time: number) => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onTogglePlay: () => void;
  onToggleFullscreen: () => void;
};

function WatchRoomPlayerPanel({
  room,
  streamUrl,
  subtitleTrack,
  videoRef,
  playing,
  currentTime,
  duration,
  playbackError,
  playerFullscreenMode,
  isFullscreen,
  isImmersiveViewport,
  onError,
  onPlayingChange,
  onTimeUpdate,
  onDurationChange,
  onEnded,
  onSeek,
  onSeekBackward,
  onSeekForward,
  onTogglePlay,
  onToggleFullscreen,
}: WatchRoomPlayerPanelProps) {
  return (
    <section className="overflow-hidden rounded-2xl border border-border bg-card/90">
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
          onError={onError}
          onPlay={() => onPlayingChange(true)}
          onPause={() => onPlayingChange(false)}
          onEnded={onEnded}
          onTimeUpdate={onTimeUpdate}
          onDurationChange={onDurationChange}
          subtitleTrack={subtitleTrack}
        />

        <div className="border-t border-border p-4 sm:p-5">
          <ProgressBar
            currentTime={currentTime}
            duration={duration}
            onSeek={onSeek}
            variant="video"
          />

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onSeekBackward}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex size-11 items-center justify-center rounded-full border border-border bg-background/60 text-foreground hover:bg-muted focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
                aria-label={`Rewind ${WATCH_ROOM_SEEK_STEP_SEC} seconds`}
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>

              <button
                type="button"
                onClick={onTogglePlay}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex size-13 items-center justify-center rounded-full bg-primary text-primary-foreground hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
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
                onClick={onSeekForward}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex size-11 items-center justify-center rounded-full border border-border bg-background/60 text-foreground hover:bg-muted focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
                aria-label={`Fast-forward ${WATCH_ROOM_SEEK_STEP_SEC} seconds`}
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            <div className="flex items-center gap-4">
              <p className="text-sm font-medium text-muted-foreground">
                {formatTimeSeconds(currentTime)} /{" "}
                {formatTimeSeconds(duration)}
              </p>
              <VolumeControl mediaRef={videoRef} />
              <button
                type="button"
                onClick={onToggleFullscreen}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex size-11 items-center justify-center rounded-full border border-border bg-background/60 text-foreground hover:bg-muted focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
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
            <div className="mt-4 rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {playbackError}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

type WatchRoomMembersPanelProps = {
  members: WatchRoomMemberType[];
  ownerId: number;
  posterUrl: string | null;
  connectedUserIds: number[];
};

function WatchRoomMembersPanel({
  members,
  ownerId,
  posterUrl,
  connectedUserIds,
}: WatchRoomMembersPanelProps) {
  return (
    <aside className="rounded-2xl border border-border bg-card/90 p-4 sm:p-5">
      <h2 className="flex items-center gap-2 text-lg font-semibold text-foreground">
        <Users className="size-5 text-primary" aria-hidden="true" />
        People in this room
      </h2>

      {posterUrl && (
        <img
          src={posterUrl}
          alt=""
          className="mt-4 aspect-2/3 w-28 rounded-xl border border-border object-cover"
        />
      )}

      <ul className="mt-4 space-y-3">
        {members.map(member => {
          const isConnected = connectedUserIds.includes(member.id);
          return (
            <li
              key={member.id}
              className="flex items-center justify-between gap-3 rounded-xl border border-border bg-background/40 px-3 py-2"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-foreground">
                  {member.name}
                  {member.id === ownerId ? (
                    <span className="ml-2 text-xs text-primary">Host</span>
                  ) : null}
                </p>
              </div>
              <span
                className={cn(
                  "rounded-full px-2 py-1 text-xs font-medium",
                  isConnected
                    ? "bg-success/15 text-success"
                    : "bg-muted text-muted-foreground",
                )}
              >
                {isConnected ? "Connected" : "Away"}
              </span>
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
