import { useRef, useEffect, useState } from "react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { ArrowLeft, Film } from "lucide-react";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import VideoPlayer from "@/components/VideoPlayer";
import ResumeDialog from "@/components/ResumeDialog";
import MoviePlayerControls from "@/components/MoviePlayerControls";
import PlaybackStatusView from "@/components/MoviePlaybackStatus";
import {
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
import { deleteMovieWatchProgress } from "@/lib/api";
import {
  MOVIE_CONTROLS_IDLE_MS,
  MOVIE_SEEK_STEP_SEC,
  MOVIE_VOLUME_STEP,
  clampMoviePlaybackTime,
  deriveMoviePlaybackStatus,
  displayedMovieDuration,
  hasEligibleMovieResumeProgress,
  nativeMoviePlaybackErrorMessage,
  shouldRebaseHlsMovieSession,
  toAbsoluteDuration,
  toAbsolutePlaybackTime,
  toMediaPlaybackTime,
} from "@/lib/movie-playback";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { PlaySearchParams } from "@/types";
import { playSearchSchema } from "@/types/movie-play";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { useIdleControls } from "@/hooks/useIdleControls";
import { useMovieWatchProgressSaver } from "@/hooks/useMovieWatchProgressSaver";
import { useHlsSessionRecovery } from "@/hooks/useHlsSessionRecovery";
import { useMoviePlaybackData } from "@/hooks/useMoviePlaybackData";

export const Route = createFileRoute("/_auth/movies/$id/play")({
  validateSearch: playSearchSchema,
  loader: async ({ context, params }) => {
    const movieId = parseInt(params.id, 10);
    if (!Number.isNaN(movieId) && movieId > 0) {
      await Promise.all([
        context.queryClient.ensureQueryData(
          libraryMovieDetailsQueryOpts(movieId),
        ),
        context.queryClient.ensureQueryData(
          movieTechnicalDetailsQueryOpts(movieId),
        ),
      ]);
    }
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
  const { mode, start } = search;
  const movieId = parseInt(id, 10);
  const navigate = Route.useNavigate();
  const router = useRouter();
  const { pause, suspendKeyboard, resumeKeyboard } = useAudioPlayerActions();

  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const backButtonRef = useRef<HTMLButtonElement>(null);
  const currentTimeRef = useRef(0);
  const durationRef = useRef(0);

  useEffect(() => {
    pause();
    suspendKeyboard();
    return () => resumeKeyboard();
  }, [pause, suspendKeyboard, resumeKeyboard]);

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [resumeDismissed, setResumeDismissed] = useState(start > 0);
  const [resumeActionPending, setResumeActionPending] = useState(false);
  const [streamReloadKey, setStreamReloadKey] = useState(0);
  const [pendingAutoPlayOnLoad, setPendingAutoPlayOnLoad] = useState(false);
  const [chapterAnnouncement, setChapterAnnouncement] =
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
    watchProgressData,
    watchProgressPending,
    title,
    posterUrl,
    qualityLabel,
    chapters,
    modeUnavailable,
    resolvedMode,
    resolvedAudioTrack,
    resolvedSubtitleTrack,
    isHlsPlayback,
    hlsStartSec,
    hlsPlaybackOffset,
    streamUrl,
    subtitleInfo,
    playbackTiming,
    movieDurationSec,
    sessionWindowKey,
  } = useMoviePlaybackData({
    movieId,
    search,
    streamReloadKey,
    onSyncSearch: ({ mode, audioTrack, subtitleTrack }) => {
      navigate({
        search: (prev: PlaySearchParams) => ({
          ...prev,
          mode,
          audio_track: audioTrack,
          subtitle_track: subtitleTrack ?? undefined,
        }),
        replace: true,
      });
    },
  });

  const displayedDuration = displayedMovieDuration(duration, playbackTiming);

  const savedProgress =
    watchProgressData?.error === false ? watchProgressData.data : null;
  const savedProgressSec = savedProgress?.progress_sec ?? null;
  const savedDurationSec = savedProgress?.duration_sec ?? null;
  const hasEligibleResumeProgress = hasEligibleMovieResumeProgress(
    savedProgressSec,
    savedDurationSec,
  );
  const resumeDialogOpen =
    !resumeDismissed &&
    start === 0 &&
    !watchProgressPending &&
    hasEligibleResumeProgress;

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
      setPendingAutoPlayOnLoad(true);
    }

    if (options?.forceReload) {
      setStreamReloadKey((prev) => prev + 1);
    }

    navigate({
      search: (prev: PlaySearchParams) => ({
        ...prev,
        mode: resolvedMode,
        audio_track: resolvedAudioTrack,
        subtitle_track: resolvedSubtitleTrack ?? undefined,
        start: Math.floor(clampedTargetTime),
      }),
      replace: true,
    });
  };

  const { handleSessionLost } = useHlsSessionRecovery({
    streamWindowKey: sessionWindowKey,
    onRecover: (currentTimeSec) =>
      navigateToPlaybackPosition(currentTimeSec, { forceReload: true }),
    onMaxAttempts: setPlaybackError,
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
      hlsStartSec,
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

  useEffect(() => {
    currentTimeRef.current = currentTime;
    durationRef.current = duration;
  }, [currentTime, duration]);

  const { handlePauseSave, handleEndedSave } = useMovieWatchProgressSaver({
    movieId,
    playing,
    currentTimeRef,
    durationRef,
  });

  useEffect(() => {
    if (!pendingAutoPlayOnLoad) return;
    const video = videoRef.current;
    if (!video) return;

    const resumePlayback = async () => {
      try {
        await video.play();
      } catch {
        // Best-effort playback resume after rebasing the HLS session.
      } finally {
        setPendingAutoPlayOnLoad(false);
      }
    };

    if (video.readyState >= 2) {
      void resumePlayback();
      return;
    }

    video.addEventListener("canplay", resumePlayback, { once: true });
    return () => {
      video.removeEventListener("canplay", resumePlayback);
    };
  }, [pendingAutoPlayOnLoad, streamUrl]);

  useEffect(() => {
    if (!isHlsPlayback || !(movieDurationSec && movieDurationSec > 0)) return;
    durationRef.current = movieDurationSec;
  }, [isHlsPlayback, movieDurationSec]);

  const handleNativePlaybackError = (code: number | null | undefined) => {
    setPlaybackError(nativeMoviePlaybackErrorMessage(code));
  };

  useVideoPlaybackKeyboard({
    containerRef,
    videoRef,
    enabled: !resumeDialogOpen,
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
    if (savedProgressSec === null) return;

    setResumeDismissed(true);
    navigateToPlaybackPosition(savedProgressSec);
  };

  const handleStartFromBeginning = async () => {
    setResumeActionPending(true);
    const res = await deleteMovieWatchProgress(movieId);
    setResumeActionPending(false);

    if (res.error) {
      showActionFailed("clear watch progress", res.message);
      return;
    }

    setResumeDismissed(true);
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

  const status = deriveMoviePlaybackStatus({
    movieNotFound,
    movieIsPending,
    hasMovie: !!movie,
    requestedMode: mode,
    techPending,
    modeUnavailable,
    playbackError,
  });

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
      onMouseMove={
        chromeFullscreenMode ? showControlsAndResetIdle : undefined
      }
      onTouchStart={
        chromeFullscreenMode ? showControlsAndResetIdle : undefined
      }
      className={cn(
        "flex min-h-0 flex-1 flex-col bg-slate-900 [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen",
        isImmersiveViewport &&
          "fixed inset-0 z-50 min-h-dvh w-full max-w-none overflow-hidden",
      )}
      role="region"
      aria-label={`Video player for ${title}`}
    >
      <LiveAnnouncer message={announcement} politeness="polite" />
      <LiveAnnouncer
        message={chapterAnnouncement.text}
        announcementKey={chapterAnnouncement.key}
        politeness="assertive"
      />
      <ResumeDialog
        open={resumeDialogOpen}
        savedProgressSec={savedProgressSec}
        pending={resumeActionPending}
        onResume={handleResume}
        onStartFromBeginning={() => void handleStartFromBeginning()}
      />

      <p className="sr-only">
        Keyboard shortcuts: Space or K to play/pause, J or Left arrow to rewind
        10 seconds, L or Right arrow to forward 10 seconds, Up/Down for volume,
        M to mute, F for fullscreen, Escape to exit fullscreen, Back button to
        go back.
      </p>

      <header
        className={
          chromeFullscreenMode
            ? `absolute inset-x-0 top-0 z-10 flex items-center justify-between border-b border-slate-700/50 bg-slate-900/95 px-4 py-3 backdrop-blur-lg transition-all duration-300 ease-out ${
                controlsVisible
                  ? "translate-y-0 opacity-100"
                  : "pointer-events-none -translate-y-full opacity-0"
              }`
            : "flex shrink-0 items-center justify-between border-b border-slate-700/50 bg-slate-900/95 px-4 py-3 backdrop-blur-lg"
        }
      >
        <div className="flex items-center gap-3">
          <Film className="size-5 text-cyan-400" aria-hidden="true" />
          <h1 className="truncate text-base font-semibold text-white">
            {title}
          </h1>
        </div>
        <button
          ref={backButtonRef}
          onClick={handleBack}
          className="flex size-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
          aria-label="Back to previous page"
        >
          <ArrowLeft className="size-5" aria-hidden="true" />
        </button>
      </header>

      <div
        className="flex min-h-0 flex-1 flex-col"
        onClick={chromeFullscreenMode ? togglePlay : undefined}
      >
        <VideoPlayer
          videoRef={videoRef}
          src={streamUrl}
          title={title}
          isFullscreen={chromeFullscreenMode}
          onError={(msg) => setPlaybackError(msg)}
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
          startSec={isHlsPlayback ? hlsPlaybackOffset : start}
          onStartApplied={(time) => {
            const absoluteTime = toAbsolutePlaybackTime(time, playbackTiming);
            currentTimeRef.current = absoluteTime;
            setCurrentTime(absoluteTime);
          }}
          onSessionLost={(time) =>
            handleSessionLost(toAbsolutePlaybackTime(time, playbackTiming))
          }
        />
      </div>

      <MoviePlayerControls
        chromeFullscreenMode={chromeFullscreenMode}
        controlsVisible={controlsVisible}
        isFullscreen={isFullscreen}
        isImmersiveViewport={isImmersiveViewport}
        currentTime={currentTime}
        duration={duration}
        displayedDuration={displayedDuration}
        playing={playing}
        qualityLabel={qualityLabel}
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
