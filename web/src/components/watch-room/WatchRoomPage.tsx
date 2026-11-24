import { useRef, useState } from "react";
import type { RefObject } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
  ArrowLeft,
  FastForward,
  LogOut,
  Maximize,
  Minimize,
  Pause,
  Play,
  Radio,
  Rewind,
  Trash2,
  UserPen,
  Users,
} from "lucide-react";
import {
  MOVIE_SEEK_STEP_SEC,
  PLAYBACK_FAILED_MESSAGE,
  TMDB_POSTER_SIZE,
  WATCH_ROOM_CLIENT_EVENT_TYPES,
} from "@/lib/constants";
import { formatTimecode } from "@/lib/format";
import { apiErrorMessage } from "@/lib/is-api-failure";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import {
  authUserFrom,
  authUserQueryOpts,
  forgetWatchRoom,
  movieTechnicalDetailsQueryOpts,
  setWatchRoomMembers,
  watchRoomQueryOpts,
} from "@/lib/query-opts";
import { movieMediaRef } from "@/lib/media-ref";
import {
  buildSubtitleTrackInfo,
  clampPlaybackTime,
  isInterruptedPlayError,
  isSourceFailurePlayError,
} from "@/lib/video-playback";
import {
  watchRoomStreamConflicted,
  watchRoomStreamUrl,
} from "@/lib/watch-room";
import { showInfo } from "@/lib/toast-helpers";
import { getWatchRoom } from "@/lib/api";
import { sessionEndedSince, sessionStamp } from "@/lib/session";
import { useHlsCapacityRetry } from "@/hooks/useHlsCapacityRetry";
import { useHlsSessionKeepalive } from "@/hooks/useHlsSessionKeepalive";
import { useHlsSessionRecovery } from "@/hooks/useHlsSessionRecovery";
import { cn } from "@/lib/utils";
import DeleteWatchRoomDialog from "@/components/watch-room/DeleteWatchRoomDialog";
import EditWatchRoomMembersDialog from "@/components/watch-room/EditWatchRoomMembersDialog";
import LeaveWatchRoomDialog from "@/components/watch-room/LeaveWatchRoomDialog";
import { Button } from "@/components/ui/button";
import {
  PlayerIconButton,
  PlayerPlayButton,
} from "@/components/playback/PlayerButtons";
import ProgressBar from "@/components/playback/ProgressBar";
import VolumeControl from "@/components/playback/VolumeControl";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import VideoPlayer, {
  type SubtitleTrackInfo,
} from "@/components/playback/VideoPlayer";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { useShortcutHints } from "@/hooks/useShortcutHints";
import { useSuspendMusicPlayback } from "@/hooks/useSuspendMusicPlayback";
import {
  useWatchRoomConnection,
  type WatchRoomConnectionState,
} from "./useWatchRoomConnection";
import { WatchRoomLoading, WatchRoomUnavailable } from "./WatchRoomStatus";
import type { WatchRoomDetailType, WatchRoomMemberType } from "@/types";

const WATCH_ROOM_TRACKS_CHANGED_MESSAGE =
  "This movie's file changed since the room was made, so its audio or subtitles no longer match. The host needs to close this room and start a new one.";

type WatchRoomPageProps = {
  roomId: number;
};

export function WatchRoomPage({ roomId }: WatchRoomPageProps) {
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  // The player card, not the page, is what goes fullscreen: the header and
  // the members column are page chrome a viewer leaves behind.
  const playerRef = useRef<HTMLElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const deleteButtonRef = useRef<HTMLButtonElement>(null);
  const editButtonRef = useRef<HTMLButtonElement>(null);
  const leaveButtonRef = useRef<HTMLButtonElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [leaveDialogOpen, setLeaveDialogOpen] = useState(false);
  const [streamReloadKey, setStreamReloadKey] = useState(0);
  // Joining a room is choosing to watch it: music steps aside from the
  // start, through the room's loading and error states too.
  useSuspendMusicPlayback();

  const {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode: playerFullscreenMode,
    toggleFullscreen,
    exitFullscreenIfActive,
  } = useVideoFullscreen({ containerRef: playerRef, videoRef });
  // Never the video's own WebKit fullscreen: it would hide the room's
  // transport row and its reconnecting warning, so a browser without element
  // fullscreen gets the expanded player card instead.
  const togglePlayerFullscreen = () =>
    void toggleFullscreen({ videoFallback: false });

  const queryClient = useQueryClient();
  // The query keeps the loaded room through a failed refetch (see
  // watchRoomQueryOpts), so the page never tears down a room in progress.
  const { data, isPending, isError } = useQuery(watchRoomQueryOpts(roomId));
  const { data: authUserData } = useQuery(authUserQueryOpts());
  const selfUserId = authUserFrom(authUserData)?.id ?? null;
  const room = data?.error === false ? data.data.room : null;
  const currentRoomId = room?.id ?? null;
  const movieTitle = room?.movie_title ?? "";
  const isHlsRoom = room ? room.playback_mode !== "direct" : false;
  const streamUrl = room
    ? watchRoomStreamUrl(room.id, room.playback_mode, streamReloadKey)
    : "";
  // Reload changes only the source URL. Keeping it out of this key prevents a
  // player rebuild from replenishing either recovery budget.
  const streamWindowKey = room ? `${room.id}:${room.playback_mode}` : "";
  const posterUrl =
    room?.movie_poster != null
      ? buildTmdbImageUrl(room.movie_poster, TMDB_POSTER_SIZE)
      : null;

  const {
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
  } = useWatchRoomConnection({
    currentRoomId,
    selfUserId,
    movieTitle,
    streamUrl,
    videoRef,
    onCurrentTimeChange: setCurrentTime,
    onPlayingChange: setPlaying,
    onMembersChanged: members =>
      setWatchRoomMembers(queryClient, roomId, members),
    // Only a re-read that loads replaces the members; one that fails leaves
    // the roster as it is.
    onReconnected: () => {
      const session = sessionStamp();
      void getWatchRoom(roomId).then(res => {
        if (sessionEndedSince(session) || res.error) return;
        setWatchRoomMembers(queryClient, roomId, res.data.room.members);
      });
    },
    // The owner closed the room, or this viewer is no longer in it: leave,
    // and drop it from this viewer's cache so Home's list and the room's
    // loader cannot offer it again.
    onRoomEnded: reason => {
      if (reason === "deleted") {
        showInfo("Watch room closed", `"${movieTitle}" is no longer available.`);
      } else {
        showInfo(
          "No longer in this watch room",
          `You are no longer invited to "${movieTitle}".`,
        );
      }
      void forgetWatchRoom(queryClient, roomId, () =>
        navigate({ to: "/", replace: true }),
      );
    },
  });

  // The player only says the stream failed. A room whose tracks no longer
  // match its file answers 409, which only reloading the room will not fix,
  // so the page asks the stream route itself.
  const handlePlayerError = (message: string) => {
    setPlaybackError(message);
    if (!room) return;
    void watchRoomStreamConflicted(streamUrl, room.playback_mode).then(
      conflicted => {
        if (conflicted) setPlaybackError(WATCH_ROOM_TRACKS_CHANGED_MESSAGE);
      },
    );
  };

  const subtitleTrackIndex = room?.subtitle_track ?? null;
  const techDetailsOpts = movieTechnicalDetailsQueryOpts(room?.movie_id ?? 0);
  const { data: techData } = useQuery({
    ...techDetailsOpts,
    enabled: (room?.movie_id ?? 0) > 0 && subtitleTrackIndex !== null,
  });
  const techLoaded = techData?.data != null;
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const subtitleTrack = room
    ? buildSubtitleTrackInfo({
        media: movieMediaRef(room.movie_id),
        resolvedSubtitleTrack: subtitleTrackIndex,
        techLoaded,
        subtitleStreams,
      })
    : null;

  // Room HLS sessions can be evicted or refused exactly like personal ones, and
  // until now a room had no recovery at all: a fragment 404 or a capacity 503
  // went straight to a dead "Stream error" (audit H9). Recovery rebuilds the
  // player; the connection then re-applies the room's last known state to the
  // new source, so participants are not thrown back to the start.
  const { waitingForCapacity, handleCapacityBusy, notifyManifestLoaded } =
    useHlsCapacityRetry({
      streamWindowKey,
      onRetry: () => setStreamReloadKey((prev) => prev + 1),
      onMaxAttempts: setPlaybackError,
    });

  const { handleSessionLost, recoveryAttempt } = useHlsSessionRecovery({
    onRecover: () => setStreamReloadKey((prev) => prev + 1),
    onMaxAttempts: () =>
      setPlaybackError(
        "The playback session expired. Reload to keep watching.",
      ),
  });

  // Rooms keep a long TTL but nothing refreshes it while every participant is
  // paused, so without this a paused room can be evicted mid-film.
  useHlsSessionKeepalive({
    enabled: isHlsRoom && !playbackError && !waitingForCapacity,
    streamUrl,
  });

  const playVideo = async () => {
    const video = videoRef.current;
    if (!video) return;

    syncToPendingPlayback();

    try {
      await video.play();
      setPlaybackError(null);
      clearPendingPlayback();
      sendPlaybackEvent(WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY, video.currentTime);
    } catch (error) {
      // Cut short, not failed: a Pause (this viewer's or the room's) or a
      // stream reload interrupted the pending play. Nothing played, so
      // nothing is broadcast. A failed source is reported by the player's
      // error event.
      if (isInterruptedPlayError(error) || isSourceFailurePlayError(error)) {
        return;
      }
      setPlaybackError(PLAYBACK_FAILED_MESSAGE);
    }
  };

  const pauseVideo = () => {
    const video = videoRef.current;
    if (!video) return;

    // A stream reload (recovery, capacity retry) leaves the element paused
    // without a `pause` event, so the controls would keep showing Pause.
    if (video.paused) setPlaying(false);
    video.pause();
    // Until the new source has metadata its playhead reads 0: the room is
    // paused where it was, and that state waits for the source to load.
    const unloaded = video.readyState < HTMLMediaElement.HAVE_METADATA;
    const position = unloaded
      ? (roomPosition() ?? video.currentTime)
      : video.currentTime;
    sendPlaybackEvent(WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE, position);
    if (unloaded) {
      holdPausedRoomPlayback();
    } else {
      clearPendingPlayback();
    }
  };

  // The viewer's intent, not just the element: a stream reload leaves the
  // element paused while the controls still show Pause, and a press there
  // must pause the room, not play it again.
  const togglePlay = async () => {
    const video = videoRef.current;
    if (!video) return;

    if (playing || !video.paused) {
      pauseVideo();
      return;
    }

    await playVideo();
  };

  const seek = (newTime: number) => {
    const video = videoRef.current;
    if (!video) return;

    const nextTime = clampPlaybackTime(
      newTime,
      Number.isFinite(video.duration) ? video.duration : 0,
      duration,
    );
    video.currentTime = nextTime;
    setCurrentTime(nextTime);
    clearPendingPlayback();
    sendPlaybackEvent(WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK, nextTime);
  };

  const seekBackward = () => seek(currentTime - MOVIE_SEEK_STEP_SEC);
  const seekForward = () => seek(currentTime + MOVIE_SEEK_STEP_SEC);

  useVideoMediaSession({
    videoRef,
    title: movieTitle || "Watch room",
    artworkUrl: posterUrl,
    currentTime,
    duration,
    playing,
    seekStepSec: MOVIE_SEEK_STEP_SEC,
    onPlay: playVideo,
    onPause: pauseVideo,
    onSeek: seek,
    enabled: !!room && !playbackError,
  });

  useVideoPlaybackKeyboard({
    containerRef,
    videoRef,
    enabled: !!room,
    // Escape leaves fullscreen or the expanded view even while the stream is
    // still loading or has failed, when the other shortcuts stand down.
    fullscreenActive: playerFullscreenMode,
    onTogglePlay: () => void togglePlay(),
    onSeekBackward: seekBackward,
    onSeekForward: seekForward,
    onSeekToStart: () => seek(0),
    onToggleFullscreen: togglePlayerFullscreen,
    onEscape: exitFullscreenIfActive,
  });
  const { showShortcutHints } = useShortcutHints();

  // Returned so the delete dialog waits for the page to go before it drops
  // the room's cache entry.
  const handleRoomExit = () => {
    closeRoomConnection();
    return navigate({ to: "/", replace: true });
  };

  if (isPending) {
    return <WatchRoomLoading />;
  }

  if (isError || data?.error || !room) {
    // A room the server refuses (403 "access denied") or cannot find (404)
    // reads the same to the viewer: it is gone or was never theirs.
    const unreachable =
      data?.error && (data.status === 403 || data.status === 404);
    return (
      <WatchRoomUnavailable
        message={
          unreachable
            ? "This room no longer exists or you were not invited."
            : apiErrorMessage(
                data,
                "This watch room could not be loaded or you do not have access to it.",
              )
        }
        onBackHome={() => navigate({ to: "/" })}
      />
    );
  }

  const connectedMembers = room.members.filter((member) =>
    // connectedUserIds is live presence for one watch room - a handful of people.
    // react-doctor-disable-next-line react-doctor/js-set-map-lookups
    connectedUserIds.includes(member.id),
  );

  return (
    <div ref={containerRef} className="bg-background text-foreground">
      <LiveAnnouncer
        message={syncAnnouncement}
        announcementKey={syncAnnouncementKey}
      />
      <LiveAnnouncer
        message={
          recoveryAttempt > 0
            ? "Playback interrupted, reloading the stream…"
            : ""
        }
        announcementKey={recoveryAttempt}
        politeness="assertive"
      />
      {showShortcutHints && (
        <p className="sr-only">
          Keyboard shortcuts: Space or K to play or pause, J or Left Arrow to
          rewind, L or Right Arrow to fast-forward, Home or 0 to restart, F for
          fullscreen, M to mute, Up or Down Arrow to adjust volume, and Escape
          to exit fullscreen.
        </p>
      )}

      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
        {/* Fullscreen and the expanded view cover the page without leaving
            it, so the header's buttons must not take focus (or open a dialog)
            behind the player. */}
        <WatchRoomHeader
          inert={playerFullscreenMode}
          room={room}
          connectedCount={connectedMembers.length}
          connectionState={connectionState}
          deleteButtonRef={deleteButtonRef}
          editButtonRef={editButtonRef}
          leaveButtonRef={leaveButtonRef}
          onDelete={() => setDeleteDialogOpen(true)}
          onEditMembers={() => setEditDialogOpen(true)}
          onLeave={() => setLeaveDialogOpen(true)}
          onBackHome={() => navigate({ to: "/" })}
        />

        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <WatchRoomPlayerPanel
            playerRef={playerRef}
            room={room}
            posterUrl={posterUrl}
            streamUrl={streamUrl}
            isHlsRoom={isHlsRoom}
            subtitleTrack={subtitleTrack}
            waitingForCapacity={waitingForCapacity}
            reconnecting={connectionState === "reconnecting"}
            onSessionLost={handleSessionLost}
            onCapacityBusy={handleCapacityBusy}
            onManifestLoaded={notifyManifestLoaded}
            videoRef={videoRef}
            playing={playing}
            currentTime={currentTime}
            duration={duration}
            playbackError={playbackError}
            playerFullscreenMode={playerFullscreenMode}
            isFullscreen={isFullscreen}
            isImmersiveViewport={isImmersiveViewport}
            onError={handlePlayerError}
            onPlayingChange={setPlaying}
            onTimeUpdate={setCurrentTime}
            onDurationChange={setDuration}
            onEnded={() => {
              // `ended` follows a `pause`, which already cleared `playing`.
              const video = videoRef.current;
              sendPlaybackEvent(
                WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE,
                video ? video.currentTime : duration,
              );
            }}
            onSeek={seek}
            onSeekBackward={seekBackward}
            onSeekForward={seekForward}
            onTogglePlay={() => void togglePlay()}
            onToggleFullscreen={togglePlayerFullscreen}
          />

          {!playerFullscreenMode ? (
            <WatchRoomMembersPanel
              members={room.members}
              ownerId={room.owner.id}
              connectedUserIds={connectedUserIds}
              presenceKnown={connectionState === "live"}
            />
          ) : null}
        </div>
      </div>

      {room.is_owner ? (
        <>
          <DeleteWatchRoomDialog
            roomId={room.id}
            movieTitle={room.movie_title}
            open={deleteDialogOpen}
            onOpenChange={setDeleteDialogOpen}
            onDeleteStart={() => markRoomEndHandled(true)}
            onDeleteError={() => markRoomEndHandled(false)}
            onDeleted={handleRoomExit}
            restoreFocusRef={deleteButtonRef}
          />
          <EditWatchRoomMembersDialog
            roomId={room.id}
            movieTitle={room.movie_title}
            inviteeIds={room.members
              .filter(member => member.id !== room.owner.id)
              .map(member => member.id)}
            open={editDialogOpen}
            onOpenChange={setEditDialogOpen}
            restoreFocusRef={editButtonRef}
          />
        </>
      ) : (
        <LeaveWatchRoomDialog
          roomId={room.id}
          movieTitle={room.movie_title}
          hostName={room.owner.name}
          open={leaveDialogOpen}
          onOpenChange={setLeaveDialogOpen}
          onLeaveStart={() => markRoomEndHandled(true)}
          onLeaveError={() => markRoomEndHandled(false)}
          onLeft={handleRoomExit}
          restoreFocusRef={leaveButtonRef}
        />
      )}
    </div>
  );
}

type WatchRoomHeaderProps = {
  /** True while the player card is fullscreen or expanded over the page. */
  inert: boolean;
  room: WatchRoomDetailType;
  connectedCount: number;
  connectionState: WatchRoomConnectionState;
  deleteButtonRef: RefObject<HTMLButtonElement | null>;
  editButtonRef: RefObject<HTMLButtonElement | null>;
  leaveButtonRef: RefObject<HTMLButtonElement | null>;
  onDelete: () => void;
  onEditMembers: () => void;
  onLeave: () => void;
  onBackHome: () => void;
};

const CONNECTION_STATE_LABELS: Record<WatchRoomConnectionState, string> = {
  connecting: "Connecting realtime sync...",
  live: "Realtime sync connected",
  reconnecting: "Reconnecting: your changes won't reach the room",
};

function WatchRoomHeader({
  inert,
  room,
  connectedCount,
  connectionState,
  deleteButtonRef,
  editButtonRef,
  leaveButtonRef,
  onDelete,
  onEditMembers,
  onLeave,
  onBackHome,
}: WatchRoomHeaderProps) {
  return (
    <header
      inert={inert}
      className="flex flex-col gap-4 rounded-2xl border border-border bg-card/90 p-4"
    >
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="flex items-center gap-2 text-xs font-semibold tracking-eyebrow text-primary uppercase">
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
            <>
              <Button
                ref={editButtonRef}
                variant="outline"
                size="sm"
                shape="pill"
                aria-label={`Edit invitees for ${room.movie_title}`}
                onClick={onEditMembers}
              >
                <UserPen className="size-4" aria-hidden="true" />
                Edit invitees
              </Button>
              <Button
                ref={deleteButtonRef}
                variant="destructive"
                size="sm"
                shape="pill"
                aria-label={`Close watch room for ${room.movie_title}`}
                onClick={onDelete}
              >
                <Trash2 className="size-4" aria-hidden="true" />
                Close room
              </Button>
            </>
          ) : (
            <Button
              ref={leaveButtonRef}
              variant="outline-destructive"
              size="sm"
              shape="pill"
              aria-label={`Leave watch room for ${room.movie_title}`}
              onClick={onLeave}
            >
              <LogOut className="size-4" aria-hidden="true" />
              Leave room
            </Button>
          )}

          <Button variant="outline" size="sm" shape="pill" onClick={onBackHome}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Back home
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
        <span
          role="status"
          className={cn(
            "rounded-full border px-3 py-1",
            connectionState === "reconnecting"
              ? "border-destructive/30 bg-destructive/10 text-destructive"
              : "border-border bg-background/60",
          )}
        >
          {CONNECTION_STATE_LABELS[connectionState]}
        </span>
        {/* Presence is only known while the socket is live. */}
        {connectionState === "live" ? (
          <span className="rounded-full border border-border bg-background/60 px-3 py-1">
            {connectedCount} connected now
          </span>
        ) : null}
      </div>
    </header>
  );
}

type WatchRoomPlayerPanelProps = {
  /** The element that goes fullscreen or expands. */
  playerRef: RefObject<HTMLElement | null>;
  room: WatchRoomDetailType;
  posterUrl: string | null;
  streamUrl: string;
  isHlsRoom: boolean;
  subtitleTrack: SubtitleTrackInfo | null;
  videoRef: RefObject<HTMLVideoElement | null>;
  playing: boolean;
  currentTime: number;
  duration: number;
  playbackError: string | null;
  playerFullscreenMode: boolean;
  isFullscreen: boolean;
  isImmersiveViewport: boolean;
  waitingForCapacity: boolean;
  /** The room's socket is reconnecting: local controls won't reach it. */
  reconnecting: boolean;
  onSessionLost: (currentTime: number) => void;
  onCapacityBusy: (retryAfterSec: number) => void;
  onManifestLoaded: () => void;
  onError: (error: string) => void;
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
  playerRef,
  room,
  posterUrl,
  streamUrl,
  isHlsRoom,
  subtitleTrack,
  videoRef,
  playing,
  currentTime,
  duration,
  playbackError,
  playerFullscreenMode,
  isFullscreen,
  isImmersiveViewport,
  waitingForCapacity,
  reconnecting,
  onSessionLost,
  onCapacityBusy,
  onManifestLoaded,
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
    <section
      ref={playerRef}
      className={cn(
        // Fullscreen, the card fills the screen exactly: a definite height
        // lets the video take what the transport row leaves, and keeps that
        // row (and its Exit button) on screen.
        "flex flex-col overflow-hidden rounded-2xl border border-border bg-card/90 [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen",
        // Opaque, on the theme like the movie player's chrome; the video
        // area brings its own black.
        playerFullscreenMode && "rounded-none border-0 bg-background",
        // Where the browser has no element fullscreen, the card covers the
        // window instead (the hook locks the page's scroll).
        isImmersiveViewport && "fixed inset-0 z-modal h-dvh w-full",
      )}
    >
      <div
        className={cn(
          "flex flex-col",
          playerFullscreenMode ? "min-h-0 flex-1" : "min-h-[50vh]",
        )}
      >
        <VideoPlayer
          videoRef={videoRef}
          src={streamUrl}
          isHlsSource={isHlsRoom}
          title={room.movie_title}
          posterUrl={posterUrl}
          isFullscreen={playerFullscreenMode}
          onError={onError}
          onPlay={() => onPlayingChange(true)}
          onPause={() => onPlayingChange(false)}
          onEnded={onEnded}
          onTimeUpdate={onTimeUpdate}
          onDurationChange={onDurationChange}
          subtitleTrack={subtitleTrack}
          onSessionLost={onSessionLost}
          onCapacityBusy={onCapacityBusy}
          onManifestLoaded={onManifestLoaded}
        />

        {waitingForCapacity && (
          <p
            className="border-t border-border px-4 py-3 text-sm text-muted-foreground sm:px-5"
            role="status"
          >
            Waiting for server capacity…
          </p>
        )}

        {/* The header's sync chip says this, but the header is behind the
            player while it is fullscreen. */}
        {playerFullscreenMode && reconnecting && (
          <p
            className="border-t border-border px-4 py-3 text-sm text-destructive sm:px-5"
            role="status"
          >
            {CONNECTION_STATE_LABELS.reconnecting}
          </p>
        )}

        <div
          className="border-t border-border p-4 sm:p-5"
          role="group"
          aria-label="Playback controls"
        >
          <ProgressBar
            currentTime={currentTime}
            duration={duration}
            onSeek={onSeek}
            variant="video"
          />

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <PlayerIconButton
                onClick={onSeekBackward}
                aria-label={`Rewind ${MOVIE_SEEK_STEP_SEC} seconds`}
              >
                <Rewind className="size-5" aria-hidden="true" />
              </PlayerIconButton>

              <PlayerPlayButton
                onClick={onTogglePlay}
                aria-label={playing ? "Pause playback" : "Play playback"}
              >
                {playing ? (
                  <Pause className="size-6 fill-current" aria-hidden="true" />
                ) : (
                  <Play className="size-6 fill-current" aria-hidden="true" />
                )}
              </PlayerPlayButton>

              <PlayerIconButton
                onClick={onSeekForward}
                aria-label={`Fast-forward ${MOVIE_SEEK_STEP_SEC} seconds`}
              >
                <FastForward className="size-5" aria-hidden="true" />
              </PlayerIconButton>
            </div>

            <div className="flex items-center gap-4">
              {/* Below sm the progress bar prints both times itself. */}
              <p className="hidden text-sm font-medium text-muted-foreground sm:block">
                {formatTimecode(currentTime, {
                  forceHours: duration >= 3600,
                })}{" "}
                / {formatTimecode(duration)}
              </p>
              <VolumeControl mediaRef={videoRef} />
              <PlayerIconButton
                onClick={onToggleFullscreen}
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
              </PlayerIconButton>
            </div>
          </div>

          {playbackError && (
            <div
              role="alert"
              className="mt-4 rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive"
            >
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
  connectedUserIds: number[];
  /** False while reconnecting: the last presence the room sent is stale. */
  presenceKnown: boolean;
};

function WatchRoomMembersPanel({
  members,
  ownerId,
  connectedUserIds,
  presenceKnown,
}: WatchRoomMembersPanelProps) {
  return (
    <aside className="rounded-2xl border border-border bg-card/90 p-4 sm:p-5">
      <h2 className="flex items-center gap-2 text-lg font-semibold text-foreground">
        <Users className="size-5 text-primary" aria-hidden="true" />
        People in this room
      </h2>

      <ul className="mt-4 space-y-3">
        {members.map((member) => {
          // Same room-sized presence list as above.
          // react-doctor-disable-next-line react-doctor/js-set-map-lookups
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
              {presenceKnown ? (
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
              ) : null}
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
