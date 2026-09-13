import { useRef, useEffect, useState, type ComponentType } from "react";
import { useBlocker, useRouter } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, type LucideProps } from "lucide-react";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Spinner } from "@/components/ui/spinner";
import VideoPlayer from "@/components/playback/VideoPlayer";
import ResumeDialog from "@/components/playback/ResumeDialog";
import PlayerControls from "@/components/playback/PlayerControls";
import PlaybackStatusView from "@/components/playback/PlaybackStatus";
import { effectiveModeLabel } from "@/lib/playback";
import { deleteMediaWatchProgress } from "@/lib/api";
import { mediaKey } from "@/lib/media-ref";
import { mediaWatchProgressQueryKey } from "@/lib/query-opts";
import {
  clampPlaybackTime,
  getOrCreateHlsPlaybackSessionId,
  stopHlsPlaybackSession,
  derivePlaybackStatus,
  displayedMediaDuration,
  shouldRebaseHlsSession,
  toAbsoluteDuration,
  toAbsolutePlaybackTime,
  toMediaPlaybackTime,
} from "@/lib/video-playback";
import {
  refreshWatchQueries,
  staysOnCurrentPlayback,
  synchronizePlaybackExit,
} from "@/lib/video-playback-exit";
import {
  CONTINUE_WATCHING_KEY,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  MOVIE_CONTROLS_IDLE_MS,
  MOVIE_SEEK_STEP_SEC,
  MOVIE_VOLUME_STEP,
  SHOW_SEASON_EPISODES_KEY,
  STREAM_MODES,
} from "@/lib/constants";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import {
  playbackSettingsToPlaySearch,
  type PlaySearchParams,
} from "@/lib/route-search";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { useIdleControls } from "@/hooks/useIdleControls";
import { useWatchProgressSaver } from "@/hooks/useWatchProgressSaver";
import { useDirectPlayFallback } from "@/hooks/useDirectPlayFallback";
import { useHlsCapacityRetry } from "@/hooks/useHlsCapacityRetry";
import { useHlsSessionKeepalive } from "@/hooks/useHlsSessionKeepalive";
import { useHlsSessionRecovery } from "@/hooks/useHlsSessionRecovery";
import { useVideoPlaybackData } from "@/hooks/useVideoPlaybackData";
import { useResumeDecision } from "@/hooks/useResumeDecision";
import type { PlaybackMediaRef } from "@/types/playback";

type ChapterAnnouncement = {
  key: number;
  text: string;
};

type VideoPlaybackPageProps = {
  /** What is playing; every playback URL and progress row is scoped by it. */
  media: PlaybackMediaRef;
  search: PlaySearchParams;
  /** Replaces the route's search params; the URL is the player's state. */
  onNavigateSearch: (
    update: (prev: PlaySearchParams) => PlaySearchParams,
  ) => void;
  /** Where "Back" goes when there is no history to return to. */
  onBackFallback: () => void;
  title: string;
  artworkUrl: string | null;
  headerIcon: ComponentType<LucideProps>;
  /** The media's own header query is still loading. */
  detailsPending: boolean;
  /** The media's header query failed or answered with no media. */
  notFound: boolean;
  /** Duration known before technical details resolve, when the header has one. */
  fallbackDurationSec?: number;
};

/**
 * The video player page shared by movies and TV episodes. The route owns the
 * media's header query (title, artwork, not-found state) and the URL; this
 * component owns everything playback: stream selection, HLS session
 * lifecycle, watch progress, resume, keyboard and fullscreen chrome.
 */
export default function VideoPlaybackPage({
  media,
  search,
  onNavigateSearch,
  onBackFallback,
  title,
  artworkUrl,
  headerIcon: HeaderIcon,
  detailsPending,
  notFound,
  fallbackDurationSec,
}: VideoPlaybackPageProps) {
  const { start } = search;
  const mode = search.mode ?? "direct";
  const { kind, id } = media;
  const currentMediaKey = mediaKey(media);
  const router = useRouter();
  const queryClient = useQueryClient();
  const { pause, suspendKeyboard, resumeKeyboard } = useAudioPlayerActions();

  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const backButtonRef = useRef<HTMLButtonElement>(null);
  const currentTimeRef = useRef(0);
  const durationRef = useRef(0);
  const hlsStopCleanupTimerRef = useRef<number | null>(null);
  const pendingAutoPlayOnLoadRef = useRef(false);
  // VideoPlayer calls onNativeError then synchronously onError; when a
  // fallback consumed the native error, the paired onError must not raise
  // the error screen.
  const fallbackConsumedErrorRef = useRef(false);

  useEffect(() => {
    pause();
    suspendKeyboard();
    return () => resumeKeyboard();
  }, [pause, suspendKeyboard, resumeKeyboard]);

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [resumeActionPending, setResumeActionPending] = useState(false);
  const [streamReloadKey, setStreamReloadKey] = useState(0);
  // Tagged with the session it describes rather than cleared by an effect, so
  // a rebase or mode change invalidates it by derivation.
  const [reportedProfile, setReportedProfile] = useState<{
    streamWindowKey: string;
    profile: string;
  } | null>(null);
  // State rather than useMemo: the getter writes sessionStorage and mints a
  // fresh random id when storage is unavailable, so a discarded memo cache
  // could change the session id mid-playback. State guarantees identity;
  // the render-phase reset re-seeds it when navigating to another item.
  const [playbackSession, setPlaybackSession] = useState(() => ({
    mediaKey: currentMediaKey,
    id: getOrCreateHlsPlaybackSessionId(media),
  }));
  if (playbackSession.mediaKey !== currentMediaKey) {
    setPlaybackSession({
      mediaKey: currentMediaKey,
      id: getOrCreateHlsPlaybackSessionId(media),
    });
  }
  const playbackSessionId = playbackSession.id;
  const [chapterAnnouncement, setChapterAnnouncement] =
    useState<ChapterAnnouncement>({
      key: 0,
      text: "",
    });
  const [fallbackAnnouncement, setFallbackAnnouncement] =
    useState<ChapterAnnouncement>({
      key: 0,
      text: "",
    });

  const {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode,
    toggleFullscreen,
    exitFullscreenIfActive,
  } = useVideoFullscreen({ containerRef, videoRef });

  const { visible: controlsVisible, showAndReset: showControlsAndResetIdle } =
    useIdleControls({
      active: chromeFullscreenMode,
      idleMs: MOVIE_CONTROLS_IDLE_MS,
    });

  const {
    techPending,
    techLoaded,
    directPlayAvailable,
    playbackPreferencesReady,
    watchProgressData,
    watchProgressPending,
    modeLabel,
    chapters,
    modeUnavailable,
    resolvedMode,
    resolvedAudioTrack,
    resolvedSubtitleTrack,
    isHlsPlayback,
    playbackStartSec,
    requestedHlsStartSec,
    actualHlsStartSec,
    hlsPlaybackOffset,
    streamUrl,
    subtitleInfo,
    playbackTiming,
    mediaDurationSec,
    sessionWindowKey,
    handleActualHlsStart,
  } = useVideoPlaybackData({
    media,
    search,
    streamReloadKey,
    playbackSessionId,
    fallbackDurationSec,
    onSyncSearch: (settings) => {
      onNavigateSearch((prev) => ({
        ...prev,
        ...playbackSettingsToPlaySearch(settings),
      }));
    },
  });

  const status = derivePlaybackStatus({
    mediaNoun: kind,
    notFound,
    detailsPending,
    hasDetails: !notFound && !detailsPending,
    requestedMode: mode,
    techPending,
    playbackPreferencesReady,
    modeUnavailable,
    playbackError,
  });

  useEffect(() => {
    if (!isHlsPlayback) return;

    if (hlsStopCleanupTimerRef.current !== null) {
      window.clearTimeout(hlsStopCleanupTimerRef.current);
      hlsStopCleanupTimerRef.current = null;
    }

    let stopped = false;
    const stopSession = (keepalive: boolean) => {
      if (stopped) return;
      stopped = true;
      void stopHlsPlaybackSession({ kind, id }, playbackSessionId, {
        keepalive,
      });
    };

    const scheduleStopSession = () => {
      if (stopped || hlsStopCleanupTimerRef.current !== null) return;
      hlsStopCleanupTimerRef.current = window.setTimeout(() => {
        hlsStopCleanupTimerRef.current = null;
        stopSession(false);
      }, 0);
    };

    const handlePageHide = (event: PageTransitionEvent) => {
      if (event.persisted) return;
      stopSession(true);
    };

    window.addEventListener("pagehide", handlePageHide);
    return () => {
      window.removeEventListener("pagehide", handlePageHide);
      scheduleStopSession();
    };
  }, [isHlsPlayback, kind, id, playbackSessionId]);

  const displayedDuration = displayedMediaDuration(duration, playbackTiming);

  const savedProgress =
    watchProgressData?.error === false ? watchProgressData.data : null;
  const savedProgressSec = savedProgress?.progress_sec ?? null;
  const savedDurationSec = savedProgress?.duration_sec ?? null;
  const { resumeDialogOpen, resumeTargetSec, dismissResumeDecision } =
    useResumeDecision({
      mediaKey: currentMediaKey,
      start,
      playing,
      watchProgressPending,
      savedProgressSec,
      savedDurationSec,
    });

  const handleBack = () => {
    if (router.history.length > 1) {
      router.history.back();
    } else {
      onBackFallback();
    }
  };

  const clampTime = (value: number) => {
    return clampPlaybackTime(value, durationRef.current, duration);
  };

  const navigateToPlaybackPosition = (
    targetTimeSec: number,
    options?: { forceReload?: boolean },
  ) => {
    const clampedTargetTime = clampTime(targetTimeSec);
    const video = videoRef.current;
    const shouldResumePlayback = !!video && !video.paused && !video.ended;

    if (shouldResumePlayback) {
      pendingAutoPlayOnLoadRef.current = true;
    }

    if (options?.forceReload) {
      setStreamReloadKey((prev) => prev + 1);
    }

    onNavigateSearch((prev) => ({
      ...prev,
      ...playbackSettingsToPlaySearch({
        mode: resolvedMode,
        audioTrack: resolvedAudioTrack,
        subtitleTrack: resolvedSubtitleTrack,
      }),
      start: Math.floor(clampedTargetTime),
    }));
  };

  const { handleSessionLost, recoveryAttempt } = useHlsSessionRecovery({
    streamWindowKey: sessionWindowKey,
    onRecover: (currentTimeSec) =>
      navigateToPlaybackPosition(currentTimeSec, { forceReload: true }),
    onMaxAttempts: setPlaybackError,
  });

  const { waitingForCapacity, handleCapacityBusy, notifyManifestLoaded } =
    useHlsCapacityRetry({
      streamWindowKey: sessionWindowKey,
      onRetry: () => setStreamReloadKey((prev) => prev + 1),
      onMaxAttempts: setPlaybackError,
    });

  const { handleNativeError: handleDirectPlayFallbackError } =
    useDirectPlayFallback({
      streamWindowKey: sessionWindowKey,
      videoRef,
      isHlsPlayback,
      resolvedMode,
      techLoaded,
      directAvailable: directPlayAvailable,
      playerMounted: status.kind === "ready",
      onFallback: () => {
        const remuxLabel =
          STREAM_MODES.find((m) => m.id === "remux")?.label ?? "remux";
        const notice = `This file can't be played directly by your browser. Switched to ${remuxLabel}.`;
        setFallbackAnnouncement((prev) => ({
          key: prev.key + 1,
          text: notice,
        }));
        showInfo("Switched playback mode", notice);
        // Resume playing once the remux stream is ready; the auto-resume
        // effect is keyed on the stream window and re-fires after this
        // navigation.
        pendingAutoPlayOnLoadRef.current = true;
        onNavigateSearch((prev) => ({
          ...prev,
          mode: "remux",
          // A failure before metadata leaves currentTime at 0; keep the
          // requested start instead of resetting the position.
          start:
            currentTimeRef.current > 0
              ? Math.floor(clampTime(currentTimeRef.current))
              : (prev.start ?? 0),
        }));
      },
    });

  const playVideo = async () => {
    const video = videoRef.current;
    if (!video) return;

    try {
      await video.play();
      setPlaybackError(null);
    } catch {
      setPlaybackError(
        "Playback failed — the browser could not play this stream.",
      );
    }
  };

  const pauseVideo = () => {
    videoRef.current?.pause();
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

  const handlePlaybackSurfaceClick = async (
    event: React.MouseEvent<HTMLDivElement>,
  ) => {
    if (!chromeFullscreenMode) return;
    const target = event.target as HTMLElement;
    const interactiveAncestor = target.closest(
      "button,a,input,select,textarea,[role='button'],[role='slider']",
    );
    if (interactiveAncestor && interactiveAncestor !== event.currentTarget) {
      return;
    }

    await togglePlay();
  };

  const seek = (newTime: number) => {
    const video = videoRef.current;
    if (!video) return;
    const t = clampTime(newTime);
    const currentVideoTime = toAbsolutePlaybackTime(
      video.currentTime,
      playbackTiming,
    );
    const rebase = shouldRebaseHlsSession({
      isHlsPlayback,
      targetTimeSec: t,
      actualHlsStartSec,
      currentVideoTimeSec: currentVideoTime,
    });

    if (rebase) {
      navigateToPlaybackPosition(t);
      return;
    }

    video.currentTime = toMediaPlaybackTime(t, playbackTiming);
    setCurrentTime(t);
  };

  const seekForward = () => seek(currentTime + MOVIE_SEEK_STEP_SEC);
  const seekBackward = () => seek(currentTime - MOVIE_SEEK_STEP_SEC);

  const handleChapterSelect = (startTimeSec: number, title: string) => {
    seek(startTimeSec);
    setChapterAnnouncement((prev) => ({
      key: prev.key + 1,
      text: `Jumped to chapter: ${title}`,
    }));
  };

  // durationRef is written where the duration is learned — onDurationChange
  // below, and the HLS seed further down — not copied back out of state here.
  // Round-tripping it meant every timeupdate restamped the ref with whatever
  // `duration` held, which is 0 until the media reports one, wiping the seed.
  useEffect(() => {
    currentTimeRef.current = currentTime;
  }, [currentTime]);

  const { handlePauseSave, handleEndedSave, flushProgress } =
    useWatchProgressSaver({
      media,
      playing,
      currentTimeRef,
      durationRef,
      fallbackDurationSec: mediaDurationSec,
    });

  useHlsSessionKeepalive({
    enabled: isHlsPlayback && status.kind === "ready",
    streamUrl,
  });

  useBlocker({
    enableBeforeUnload: false,
    shouldBlockFn: async ({ current, next }) => {
      if (staysOnCurrentPlayback(current, next)) return false;

      await synchronizePlaybackExit({
        pausePlayback: () => videoRef.current?.pause(),
        flushProgress,
        refreshWatchQueries: () => refreshWatchQueries(queryClient, media),
        onSaveError: () =>
          showActionFailed(
            "save watch progress",
            "Unable to save your latest playback position.",
          ),
      });

      return false;
    },
  });

  useEffect(() => {
    if (!pendingAutoPlayOnLoadRef.current) return;
    const video = videoRef.current;
    if (!video) return;

    const resumePlayback = async () => {
      try {
        await video.play();
      } catch {
        // Best-effort playback resume after rebasing the HLS session.
      }

      pendingAutoPlayOnLoadRef.current = false;
    };

    if (video.readyState >= 2) {
      void resumePlayback();
      return;
    }

    video.addEventListener("canplay", resumePlayback, { once: true });
    return () => {
      video.removeEventListener("canplay", resumePlayback);
    };
    // Keyed on the stream window, not streamUrl: the direct-play URL is a
    // constant, so a fallback navigation would never re-fire this otherwise
    // (audit D12).
  }, [sessionWindowKey]);

  useEffect(() => {
    if (!isHlsPlayback || !(mediaDurationSec && mediaDurationSec > 0)) return;
    durationRef.current = mediaDurationSec;
  }, [isHlsPlayback, mediaDurationSec]);

  // The effective profile belongs to one session, so an answer carried over
  // from a previous one is ignored rather than used to describe this one.
  const effectiveProfile =
    reportedProfile?.streamWindowKey === sessionWindowKey
      ? reportedProfile.profile
      : null;

  // The player calls this before its own onError, which reports a message for
  // every MediaError code. Writing one here too only produced a value that the
  // next line of the same handler overwrote, so this decides one thing: whether
  // the error was consumed by a direct-play fallback and must be swallowed.
  const handleNativePlaybackError = (code: number | null | undefined) => {
    if (handleDirectPlayFallbackError(code)) {
      fallbackConsumedErrorRef.current = true;
    }
  };

  const keyboardShortcutsEnabled = status.kind === "ready" && !resumeDialogOpen;

  useVideoPlaybackKeyboard({
    containerRef,
    videoRef,
    enabled: keyboardShortcutsEnabled,
    fullscreenActive: chromeFullscreenMode,
    volumeStep: MOVIE_VOLUME_STEP,
    onShowControls: showControlsAndResetIdle,
    onTogglePlay: () => void togglePlay(),
    onSeekBackward: seekBackward,
    onSeekForward: seekForward,
    onSeekToStart: () => seek(0),
    onToggleFullscreen: () => void toggleFullscreen(),
    onEscape: exitFullscreenIfActive,
  });

  const announcement = playing ? `Playing: ${title}` : `Paused: ${title}`;

  const handleResume = () => {
    if (resumeTargetSec === null) return;

    dismissResumeDecision();
    navigateToPlaybackPosition(resumeTargetSec);
  };

  const handleStartFromBeginning = async () => {
    setResumeActionPending(true);
    const res = await deleteMediaWatchProgress(media);
    // deleteMediaWatchProgress resolves an envelope and never rejects, so this
    // clears on both outcomes; `finally` would bail out the React Compiler.
    // react-doctor-disable-next-line react-doctor/no-loading-flag-reset-outside-finally
    setResumeActionPending(false);

    if (res.error) {
      showActionFailed("clear watch progress", res.message);
      return;
    }

    queryClient.removeQueries({
      queryKey:
        media.kind === "movie"
          ? [CONTINUE_WATCHING_KEY]
          : [SHOW_SEASON_EPISODES_KEY],
    });
    queryClient.removeQueries({
      queryKey: mediaWatchProgressQueryKey(media),
    });
    dismissResumeDecision();
  };

  useVideoMediaSession({
    videoRef,
    title,
    artworkUrl,
    currentTime,
    duration: displayedDuration,
    playing,
    seekStepSec: MOVIE_SEEK_STEP_SEC,
    onPlay: playVideo,
    onPause: pauseVideo,
    onSeek: seek,
    enabled: !notFound && !detailsPending && !playbackError && !modeUnavailable,
  });

  const videoPlayer = (
    <VideoPlayer
      videoRef={videoRef}
      src={streamUrl}
      isHlsSource={isHlsPlayback}
      title={title}
      isFullscreen={chromeFullscreenMode}
      onError={(msg) => {
        if (fallbackConsumedErrorRef.current) {
          fallbackConsumedErrorRef.current = false;
          return;
        }
        setPlaybackError(msg);
      }}
      onPlay={() => setPlaying(true)}
      onPause={() => {
        setPlaying(false);
        void handlePauseSave();
      }}
      onEnded={() => {
        setPlaying(false);
        void handleEndedSave();
      }}
      onTimeUpdate={(time) => {
        const absoluteTime = toAbsolutePlaybackTime(time, playbackTiming);
        currentTimeRef.current = absoluteTime;
        setCurrentTime(absoluteTime);
      }}
      onDurationChange={(nextDuration) => {
        const absoluteDuration = toAbsoluteDuration(
          nextDuration,
          playbackTiming,
        );
        durationRef.current = absoluteDuration;
        setDuration(absoluteDuration);
      }}
      onNativeError={handleNativePlaybackError}
      subtitleTrack={subtitleInfo}
      startSec={isHlsPlayback ? hlsPlaybackOffset : playbackStartSec}
      requestedStartSec={requestedHlsStartSec}
      onStartApplied={(time) => {
        const absoluteTime = toAbsolutePlaybackTime(time, playbackTiming);
        currentTimeRef.current = absoluteTime;
        setCurrentTime(absoluteTime);
      }}
      onSessionLost={(time) =>
        handleSessionLost(toAbsolutePlaybackTime(time, playbackTiming))
      }
      onCapacityBusy={handleCapacityBusy}
      onManifestLoaded={notifyManifestLoaded}
      onEffectiveProfile={(profile) =>
        setReportedProfile({ streamWindowKey: sessionWindowKey, profile })
      }
      onActualStart={handleActualHlsStart}
    />
  );

  const capacityOverlay = waitingForCapacity ? (
    <div
      className={cn(
        MOTION_MEDIA_OVERLAY_ENTER_CLASS,
        "pointer-events-none absolute inset-0 z-10 flex items-center justify-center",
      )}
    >
      <div className="flex items-center gap-3 rounded-full bg-background/80 px-5 py-3 backdrop-blur-sm">
        <Spinner className="size-5 text-primary" aria-hidden="true" />
        <p className="text-sm font-medium text-foreground">
          Waiting for server capacity…
        </p>
      </div>
    </div>
  ) : null;

  if (status.kind !== "ready") {
    return (
      <PlaybackStatusView
        status={status}
        mediaNoun={kind}
        onBack={handleBack}
        onRetry={() => {
          setPlaybackError(null);
          setPlaying(false);
          setCurrentTime(0);
          setDuration(0);
        }}
        backButtonRef={backButtonRef}
        containerRef={containerRef}
      />
    );
  }

  return (
    <div
      ref={containerRef}
      onMouseMove={chromeFullscreenMode ? showControlsAndResetIdle : undefined}
      onTouchStart={chromeFullscreenMode ? showControlsAndResetIdle : undefined}
      className={cn(
        "flex min-h-0 flex-1 flex-col bg-background [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen",
        isImmersiveViewport &&
          "fixed inset-0 z-50 min-h-dvh w-full max-w-none overflow-hidden",
      )}
      role="region"
      aria-label={`Video player for ${title}`}
      tabIndex={-1}
    >
      <LiveAnnouncer message={announcement} politeness="polite" />
      <LiveAnnouncer
        message={waitingForCapacity ? "Waiting for server capacity…" : ""}
        politeness="polite"
      />
      <LiveAnnouncer
        message={chapterAnnouncement.text}
        announcementKey={chapterAnnouncement.key}
        politeness="assertive"
      />
      <LiveAnnouncer
        message={fallbackAnnouncement.text}
        announcementKey={fallbackAnnouncement.key}
        politeness="assertive"
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
      <ResumeDialog
        open={resumeDialogOpen}
        resumeTargetSec={resumeTargetSec}
        pending={resumeActionPending}
        onResume={handleResume}
        onStartFromBeginning={() => void handleStartFromBeginning()}
        restoreFocusRef={containerRef}
      />

      <p className="sr-only">
        Keyboard shortcuts: Space or K to play/pause, J or Left arrow to rewind{" "}
        {MOVIE_SEEK_STEP_SEC} seconds, L or Right arrow to forward{" "}
        {MOVIE_SEEK_STEP_SEC} seconds, Up/Down for volume, M to mute, F for
        fullscreen, Escape to exit fullscreen, Back button to go back.
      </p>

      <header
        className={
          chromeFullscreenMode
            ? cn(
                MOTION_PLAYER_CHROME_PANEL_CLASS,
                "absolute inset-x-0 top-0 z-10 flex items-center justify-between border-b border-border bg-background/95 px-4 py-3 backdrop-blur-lg",
                controlsVisible
                  ? "translate-y-0 opacity-100"
                  : "pointer-events-none -translate-y-full opacity-0",
              )
            : "flex shrink-0 items-center justify-between border-b border-border bg-background/95 px-4 py-3 backdrop-blur-lg"
        }
      >
        <div className="flex items-center gap-3">
          <HeaderIcon className="size-5 text-primary" aria-hidden="true" />
          <h1 className="truncate text-base font-semibold text-foreground">
            {title}
          </h1>
        </div>
        <button
          type="button"
          ref={backButtonRef}
          onClick={handleBack}
          className={cn(
            MOTION_PLAYER_CHROME_BUTTON_CLASS,
            "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
          )}
          aria-label="Back to previous page"
        >
          <ArrowLeft className="size-5" aria-hidden="true" />
        </button>
      </header>

      {chromeFullscreenMode ? (
        // Click-to-toggle is a pointer convenience only; the same toggle is
        // reachable from the footer play button and Space/K, so the surface
        // carries no button role (audit D14).
        // react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions
        <div
          className="relative flex min-h-0 flex-1 flex-col"
          onClick={handlePlaybackSurfaceClick}
        >
          {videoPlayer}
          {capacityOverlay}
        </div>
      ) : (
        <div className="relative flex min-h-0 flex-1 flex-col">
          {videoPlayer}
          {capacityOverlay}
        </div>
      )}

      <PlayerControls
        chromeFullscreenMode={chromeFullscreenMode}
        controlsVisible={controlsVisible}
        isFullscreen={isFullscreen}
        isImmersiveViewport={isImmersiveViewport}
        currentTime={currentTime}
        duration={duration}
        displayedDuration={displayedDuration}
        playing={playing}
        modeLabel={
          isHlsPlayback
            ? effectiveModeLabel(resolvedMode, effectiveProfile)
            : modeLabel
        }
        chapters={chapters}
        videoRef={videoRef}
        onSeek={seek}
        onSeekBackward={seekBackward}
        onSeekForward={seekForward}
        onTogglePlay={() => void togglePlay()}
        onToggleFullscreen={() => void toggleFullscreen()}
        onSelectChapter={handleChapterSelect}
      />
    </div>
  );
}
