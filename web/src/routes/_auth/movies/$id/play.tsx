import { useRef, useEffect, useState } from "react";
import {
  createFileRoute,
  redirect,
  useBlocker,
  useRouter,
} from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Film } from "lucide-react";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Spinner } from "@/components/ui/spinner";
import VideoPlayer from "@/components/playback/VideoPlayer";
import ResumeDialog from "@/components/movies/ResumeDialog";
import MoviePlayerControls from "@/components/movies/MoviePlayerControls";
import PlaybackStatusView from "@/components/movies/MoviePlaybackStatus";
import {
  authUserQueryOpts,
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
  playbackSettingsQueryOpts,
} from "@/lib/query-opts";
import {
  effectiveModeLabel,
  getAvailableModes,
  getDefaultPlaybackSettings,
  getPrimaryVideoStream,
  playbackDefaultsInput,
} from "@/lib/playback";
import { getDevicePlaybackPreferences } from "@/lib/playback-preferences";
import { deleteMovieWatchProgress } from "@/lib/api";
import {
  clampMoviePlaybackTime,
  getOrCreateMovieHlsPlaybackSessionId,
  stopMovieHlsPlaybackSession,
  deriveMoviePlaybackStatus,
  displayedMovieDuration,
  shouldRebaseHlsMovieSession,
  toAbsoluteDuration,
  toAbsolutePlaybackTime,
  toMediaPlaybackTime,
} from "@/lib/movie-playback";
import {
  refreshMovieWatchQueries,
  staysOnCurrentMoviePlayback,
  synchronizeMoviePlaybackExit,
} from "@/lib/movie-playback-exit";
import {
  CONTINUE_WATCHING_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  MOVIE_CONTROLS_IDLE_MS,
  MOVIE_SEEK_STEP_SEC,
  MOVIE_VOLUME_STEP,
  STREAM_MODES,
} from "@/lib/constants";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import {
  playbackSettingsToPlaySearch,
  playSearchSchema,
  type PlaySearchParams,
} from "@/lib/route-search";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { useIdleControls } from "@/hooks/useIdleControls";
import { useMovieWatchProgressSaver } from "@/hooks/useMovieWatchProgressSaver";
import { useDirectPlayFallback } from "@/hooks/useDirectPlayFallback";
import { useHlsCapacityRetry } from "@/hooks/useHlsCapacityRetry";
import { useHlsSessionKeepalive } from "@/hooks/useHlsSessionKeepalive";
import { useHlsSessionRecovery } from "@/hooks/useHlsSessionRecovery";
import { useMoviePlaybackData } from "@/hooks/useMoviePlaybackData";
import { useMovieResumeDecision } from "@/hooks/useMovieResumeDecision";

export const Route = createFileRoute("/_auth/movies/$id/play")({
  validateSearch: playSearchSchema,
  loaderDeps: ({ search }) => ({
    mode: search.mode,
    audio_track: search.audio_track,
    subtitle_track: search.subtitle_track,
    start: search.start,
  }),
  loader: async ({ context, params, deps }) => {
    const movieId = parseInt(params.id, 10);
    if (Number.isNaN(movieId) || movieId <= 0) return;

    if (deps.mode !== undefined) {
      // The player waits for technical details before requesting media
      // (audit D16), so warm the caches for URLs that already carry a mode —
      // but without blocking navigation, or the player skeleton would not
      // render until every query resolves. The component's own queries join
      // these in-flight fetches; errors surface through them.
      void (async () => {
        const authRes =
          await context.queryClient.ensureQueryData(authUserQueryOpts());
        if (authRes.error) return;
        await Promise.all([
          context.queryClient.ensureQueryData(
            libraryMovieDetailsQueryOpts(movieId),
          ),
          context.queryClient.ensureQueryData(
            movieTechnicalDetailsQueryOpts(movieId),
          ),
          context.queryClient.ensureQueryData(playbackSettingsQueryOpts()),
        ]);
      })().catch(() => {});
      return;
    }

    const authRes =
      await context.queryClient.ensureQueryData(authUserQueryOpts());
    if (authRes.error) return;

    const [, techRes, playbackRes] = await Promise.all([
      context.queryClient.ensureQueryData(
        libraryMovieDetailsQueryOpts(movieId),
      ),
      context.queryClient.ensureQueryData(
        movieTechnicalDetailsQueryOpts(movieId),
      ),
      context.queryClient.ensureQueryData(playbackSettingsQueryOpts()),
    ]);

    const techData = techRes.error === false ? techRes.data : null;
    if (!techData) return;

    const videoStreams = techData.video_streams ?? [];
    const audioStreams = techData.audio_streams ?? [];
    const subtitleStreams = techData.subtitles ?? [];
    const primaryVideo = getPrimaryVideoStream(videoStreams);
    const availableModes = getAvailableModes({
      video: primaryVideo,
      videoStreamsLoaded: true,
      audioStreams,
      mimeType: techData.movie?.mime_type,
    });
    if (availableModes.length === 0) return;

    const serverSettings =
      playbackRes.error === false ? (playbackRes.data?.settings ?? null) : null;
    const resolved = getDefaultPlaybackSettings(
      availableModes,
      playbackDefaultsInput(
        getDevicePlaybackPreferences(authRes.data.user.id),
        serverSettings,
      ),
      audioStreams,
      subtitleStreams,
    );

    throw redirect({
      to: "/movies/$id/play",
      params: { id: params.id },
      search: {
        ...playbackSettingsToPlaySearch(resolved),
        start: deps.start ?? 0,
      },
      replace: true,
    });
  },
  component: PlayMoviePage,
});

type ChapterAnnouncement = {
  key: number;
  text: string;
};

function PlayMoviePage() {
  const { id } = Route.useParams();
  const search = Route.useSearch();
  const { start } = search;
  const mode = search.mode ?? "direct";
  const movieId = parseInt(id, 10);
  const navigate = Route.useNavigate();
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
  // the render-phase reset re-seeds it when navigating to another movie.
  const [playbackSession, setPlaybackSession] = useState(() => ({
    movieId,
    id: getOrCreateMovieHlsPlaybackSessionId(movieId),
  }));
  if (playbackSession.movieId !== movieId) {
    setPlaybackSession({
      movieId,
      id: getOrCreateMovieHlsPlaybackSessionId(movieId),
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
    movie,
    movieIsPending,
    movieNotFound,
    techPending,
    techLoaded,
    directPlayAvailable,
    playbackPreferencesReady,
    watchProgressData,
    watchProgressPending,
    title,
    posterUrl,
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
    movieDurationSec,
    sessionWindowKey,
    handleActualHlsStart,
  } = useMoviePlaybackData({
    movieId,
    search,
    streamReloadKey,
    playbackSessionId,
    onSyncSearch: (settings) => {
      navigate({
        search: (prev: PlaySearchParams) => ({
          ...prev,
          ...playbackSettingsToPlaySearch(settings),
        }),
        replace: true,
      });
    },
  });

  const status = deriveMoviePlaybackStatus({
    movieNotFound,
    movieIsPending,
    hasMovie: !!movie,
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
      void stopMovieHlsPlaybackSession(movieId, playbackSessionId, {
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
  }, [isHlsPlayback, movieId, playbackSessionId]);

  const displayedDuration = displayedMovieDuration(duration, playbackTiming);

  const savedProgress =
    watchProgressData?.error === false ? watchProgressData.data : null;
  const savedProgressSec = savedProgress?.progress_sec ?? null;
  const savedDurationSec = savedProgress?.duration_sec ?? null;
  const { resumeDialogOpen, resumeTargetSec, dismissResumeDecision } =
    useMovieResumeDecision({
      movieId,
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
      navigate({ to: "/movies" });
    }
  };

  const clampPlaybackTime = (value: number) => {
    return clampMoviePlaybackTime(value, durationRef.current, duration);
  };

  const navigateToPlaybackPosition = (
    targetTimeSec: number,
    options?: { forceReload?: boolean },
  ) => {
    const clampedTargetTime = clampPlaybackTime(targetTimeSec);
    const video = videoRef.current;
    const shouldResumePlayback = !!video && !video.paused && !video.ended;

    if (shouldResumePlayback) {
      pendingAutoPlayOnLoadRef.current = true;
    }

    if (options?.forceReload) {
      setStreamReloadKey((prev) => prev + 1);
    }

    navigate({
      search: (prev: PlaySearchParams) => ({
        ...prev,
        ...playbackSettingsToPlaySearch({
          mode: resolvedMode,
          audioTrack: resolvedAudioTrack,
          subtitleTrack: resolvedSubtitleTrack,
        }),
        start: Math.floor(clampedTargetTime),
      }),
      replace: true,
    });
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
        navigate({
          search: (prev: PlaySearchParams) => ({
            ...prev,
            mode: "remux",
            // A failure before metadata leaves currentTime at 0; keep the
            // requested start instead of resetting the position.
            start:
              currentTimeRef.current > 0
                ? Math.floor(clampPlaybackTime(currentTimeRef.current))
                : (prev.start ?? 0),
          }),
          replace: true,
        });
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
    const t = clampPlaybackTime(newTime);
    const currentVideoTime = toAbsolutePlaybackTime(
      video.currentTime,
      playbackTiming,
    );
    const shouldRebaseHlsSession = shouldRebaseHlsMovieSession({
      isHlsPlayback,
      targetTimeSec: t,
      actualHlsStartSec,
      currentVideoTimeSec: currentVideoTime,
    });

    if (shouldRebaseHlsSession) {
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
    useMovieWatchProgressSaver({
      movieId,
      playing,
      currentTimeRef,
      durationRef,
      fallbackDurationSec: movieDurationSec,
    });

  useHlsSessionKeepalive({
    enabled: isHlsPlayback && status.kind === "ready",
    streamUrl,
  });

  useBlocker({
    enableBeforeUnload: false,
    shouldBlockFn: async ({ current, next }) => {
      if (staysOnCurrentMoviePlayback(current, next)) return false;

      await synchronizeMoviePlaybackExit({
        pausePlayback: () => videoRef.current?.pause(),
        flushProgress,
        refreshWatchQueries: () =>
          refreshMovieWatchQueries(queryClient, movieId),
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
    if (!isHlsPlayback || !(movieDurationSec && movieDurationSec > 0)) return;
    durationRef.current = movieDurationSec;
  }, [isHlsPlayback, movieDurationSec]);

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
    const res = await deleteMovieWatchProgress(movieId);
    setResumeActionPending(false);

    if (res.error) {
      showActionFailed("clear watch progress", res.message);
      return;
    }

    queryClient.removeQueries({ queryKey: [CONTINUE_WATCHING_KEY] });
    queryClient.removeQueries({
      queryKey: [MOVIE_WATCH_PROGRESS_KEY, movieId],
    });
    dismissResumeDecision();
  };

  useVideoMediaSession({
    videoRef,
    title,
    artworkUrl: posterUrl,
    currentTime,
    duration: displayedDuration,
    playing,
    seekStepSec: MOVIE_SEEK_STEP_SEC,
    onPlay: playVideo,
    onPause: pauseVideo,
    onSeek: seek,
    enabled: !movieNotFound && !!movie && !playbackError && !modeUnavailable,
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
          <Film className="size-5 text-primary" aria-hidden="true" />
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

      <MoviePlayerControls
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
