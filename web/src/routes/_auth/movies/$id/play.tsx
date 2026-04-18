import { useRef, useEffect, useState } from "react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { zodSearchValidator } from "@tanstack/router-zod-adapter";
import { useQuery } from "@tanstack/react-query";
import {
  AlertCircle,
  ArrowLeft,
  Film,
  Rewind,
  FastForward,
  Pause,
  Play,
  Maximize,
  Minimize,
  RotateCcw,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import VideoPlayer from "@/components/VideoPlayer";
import ResumeDialog from "@/components/ResumeDialog";
import ChapterMenu from "@/components/ChapterMenu";
import {
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
  movieWatchProgressQueryOpts,
} from "@/lib/query-opts";
import { formatTimeSeconds } from "@/lib/format";
import {
  deleteMovieWatchProgress,
  updateMovieWatchProgress,
} from "@/lib/api";
import {
  HLS_RESUME_REWIND_BUFFER_SEC,
  HLS_SESSION_LOST_MAX_ATTEMPTS,
  HLS_SESSION_LOST_MIN_INTERVAL_MS,
} from "@/lib/constants";
import {
  STREAM_MODES,
  formatSubtitleLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  resolvePlaybackSettings,
  type StreamModeId,
} from "@/lib/playback";
import { showActionFailed } from "@/lib/toast-helpers";
import { toast } from "sonner";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  requestElementFullscreen,
  tryWebKitVideoEnterFullscreen,
  tryWebKitVideoExitFullscreen,
} from "@/lib/fullscreen";
import { cn } from "@/lib/utils";
import { unwrapFloatOrUndefined, unwrapStringOrUndefined } from "@/lib/nullable";
import type { PlaySearchParams } from "@/types";
import { playSearchSchema } from "@/types/movie-play";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";

export const Route = createFileRoute("/_auth/movies/$id/play")({
  validateSearch: zodSearchValidator(playSearchSchema),
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

const SEEK_STEP_SEC = 10;
const VOLUME_STEP = 0.1;
const CONTROLS_IDLE_MS = 3000;
const WATCH_PROGRESS_SAVE_INTERVAL_MS = 15_000;
const WATCH_PROGRESS_MIN_SECONDS = 180;
const WATCH_PROGRESS_COMPLETION_THRESHOLD = 0.98;
const HLS_FORWARD_REBASE_THRESHOLD_SEC = 120;

type ChapterAnnouncement = {
  key: number;
  text: string;
};

function buildStreamUrl(
  movieId: number,
  mode: StreamModeId,
  audioTrack: number,
  hlsStartSec: number,
  reloadKey: number,
): string {
  if (mode === "direct") return `/api/movies/${movieId}/stream`;
  const params = new URLSearchParams({
    audio_track: String(audioTrack),
  });
  if (hlsStartSec > 0) {
    params.set("start", String(Math.floor(hlsStartSec)));
  }
  if (reloadKey > 0) {
    params.set("reload", String(reloadKey));
  }
  return `/api/movies/${movieId}/hls/${mode}/playlist.m3u8?${params}`;
}

function shouldPersistWatchProgress(progressSec: number, durationSec: number) {
  if (!(durationSec > 0)) return false;
  const clampedProgress = Math.max(0, Math.min(progressSec, durationSec));
  const completionRatio = clampedProgress / durationSec;

  return (
    completionRatio >= WATCH_PROGRESS_COMPLETION_THRESHOLD ||
    clampedProgress >= WATCH_PROGRESS_MIN_SECONDS
  );
}

async function persistMovieWatchProgress(
  movieId: number,
  progressSec: number,
  durationSec: number,
  options?: { keepalive?: boolean },
) {
  if (!(durationSec > 0)) return;

  const clampedProgress = Math.max(0, Math.min(progressSec, durationSec));
  if (!shouldPersistWatchProgress(clampedProgress, durationSec)) return;

  if (options?.keepalive) {
    try {
      await fetch(`/api/movies/${movieId}/watch-progress`, {
        method: "PUT",
        credentials: "include",
        keepalive: true,
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          progress_sec: clampedProgress,
          duration_sec: durationSec,
        }),
      });
    } catch {
      // Best-effort pagehide save; ignore network failures.
    }
    return;
  }

  const res = await updateMovieWatchProgress(movieId, clampedProgress, durationSec);
  if (res.error) {
    throw new Error(res.message);
  }
}

function PlayMoviePage() {
  const { id } = Route.useParams();
  const { mode, audio_track: audioTrack, subtitle_track: subtitleTrack, start } = Route.useSearch();
  const movieId = parseInt(id, 10);
  const navigate = Route.useNavigate();
  const router = useRouter();
  const { pause, suspendKeyboard, resumeKeyboard } = useAudioPlayerActions();

  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const backButtonRef = useRef<HTMLButtonElement>(null);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const currentTimeRef = useRef(0);
  const durationRef = useRef(0);
  const sessionLostNavAttemptsRef = useRef(0);
  const lastSessionLostNavAtRef = useRef(0);
  const sessionLostStreamKeyRef = useRef("");

  useEffect(() => {
    pause();
    suspendKeyboard();
    return () => resumeKeyboard();
  }, [pause, suspendKeyboard, resumeKeyboard]);

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isImmersiveViewport, setIsImmersiveViewport] = useState(false);
  const fullscreenSourceRef = useRef<"none" | "document" | "webkitVideo">(
    "none",
  );
  const [controlsVisible, setControlsVisible] = useState(true);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [resumeDismissed, setResumeDismissed] = useState(start > 0);
  const [resumeActionPending, setResumeActionPending] = useState(false);
  const [streamReloadKey, setStreamReloadKey] = useState(0);
  const [pendingAutoPlayOnLoad, setPendingAutoPlayOnLoad] = useState(false);
  const [chapterAnnouncement, setChapterAnnouncement] = useState<ChapterAnnouncement>({
    key: 0,
    text: "",
  });

  const chromeFullscreenMode = isFullscreen || isImmersiveViewport;

  const scheduleHideControls = () => {
    if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
    idleTimerRef.current = setTimeout(() => {
      setControlsVisible(false);
      idleTimerRef.current = null;
    }, CONTROLS_IDLE_MS);
  };

  const showControlsAndResetIdle = () => {
    setControlsVisible(true);
    scheduleHideControls();
  };

  const { data, isPending, isError } = useQuery(
    libraryMovieDetailsQueryOpts(movieId),
  );
  const movie = data && !data.error ? data.data?.movie : null;
  const title = movie?.title ?? "Movie";
  const movieNotFound = isError || (data && data.error);

  const { data: techData, isPending: techPending } = useQuery(
    movieTechnicalDetailsQueryOpts(movieId),
  );
  const { data: watchProgressData, isPending: watchProgressPending } = useQuery(
    movieWatchProgressQueryOpts(movieId),
  );
  const techLoaded = !techPending && techData?.data != null;
  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const chapters = techData?.data?.chapters ?? [];
  const primaryVideo = techLoaded
    ? getPrimaryVideoStream(videoStreams)
    : undefined;
  const availableModes = techLoaded
    ? getAvailableModes(
        primaryVideo?.height ?? 0,
        primaryVideo?.codec,
        audioStreams[0]?.codec,
        techData.data!.movie?.mime_type,
      )
    : null;
  const resolvedPlaybackSettings =
    availableModes !== null
      ? resolvePlaybackSettings(
          {
            mode,
            audioTrack,
            subtitleTrack: subtitleTrack ?? null,
          },
          availableModes,
          audioStreams,
          subtitleStreams,
        )
      : {
          mode,
          audioTrack,
          subtitleTrack: subtitleTrack ?? null,
        };
  const resolvedMode = resolvedPlaybackSettings.mode;
  const resolvedAudioTrack = resolvedPlaybackSettings.audioTrack;
  const resolvedSubtitleTrack = resolvedPlaybackSettings.subtitleTrack;
  const isHlsPlayback = resolvedMode !== "direct";
  const hlsStartSec = isHlsPlayback
    ? Math.max(0, start - HLS_RESUME_REWIND_BUFFER_SEC)
    : 0;
  const hlsPlaybackOffsetSec = isHlsPlayback ? Math.max(0, start - hlsStartSec) : 0;
  const streamUrl = buildStreamUrl(
    movieId,
    resolvedMode,
    resolvedAudioTrack,
    hlsStartSec,
    streamReloadKey,
  );
  const qualityLabel =
    STREAM_MODES.find(m => m.id === resolvedMode)?.label ?? resolvedMode;
  const modeUnavailable = availableModes !== null && availableModes.length === 0;
  const movieDurationSec =
    unwrapFloatOrUndefined(techData?.data?.movie?.duration) ??
    unwrapFloatOrUndefined(movie?.duration);
  const toAbsolutePlaybackTime = (timeSec: number) =>
    isHlsPlayback ? hlsStartSec + timeSec : timeSec;
  const toMediaTime = (timeSec: number) =>
    isHlsPlayback ? Math.max(0, timeSec - hlsStartSec) : timeSec;
  const toAbsoluteDuration = (durationSec: number) => {
    if (!isHlsPlayback) return durationSec;
    if (movieDurationSec && movieDurationSec > 0) {
      return movieDurationSec;
    }
    return hlsStartSec + durationSec;
  };

  const subtitleInfo = (() => {
    if (resolvedSubtitleTrack === null || !techLoaded) return null;
    if (
      resolvedSubtitleTrack < 0 ||
      resolvedSubtitleTrack >= subtitleStreams.length
    ) {
      return null;
    }
    const sub = subtitleStreams[resolvedSubtitleTrack];
    return {
      url: `/api/movies/${movieId}/subtitles/${resolvedSubtitleTrack}/web.vtt`,
      label: formatSubtitleLabel(sub, resolvedSubtitleTrack),
      srclang: unwrapStringOrUndefined(sub.language) ?? "",
    };
  })();
  const sessionLostStreamKey = `${movieId}:${resolvedMode}:${resolvedAudioTrack}`;
  const sessionWindowKey = `${sessionLostStreamKey}:${Math.floor(hlsStartSec)}`;

  useEffect(() => {
    if (sessionLostStreamKeyRef.current !== sessionWindowKey) {
      sessionLostStreamKeyRef.current = sessionWindowKey;
      sessionLostNavAttemptsRef.current = 0;
      lastSessionLostNavAtRef.current = 0;
    }
  }, [sessionWindowKey]);

  useEffect(() => {
    if (availableModes === null) return;

    const resolvedSubtitleSearch = resolvedSubtitleTrack ?? undefined;
    if (
      mode === resolvedMode &&
      audioTrack === resolvedAudioTrack &&
      subtitleTrack === resolvedSubtitleSearch
    ) {
      return;
    }

    navigate({
      search: (prev: PlaySearchParams) => ({
        ...prev,
        mode: resolvedMode,
        audio_track: resolvedAudioTrack,
        subtitle_track: resolvedSubtitleSearch,
      }),
      replace: true,
    });
  }, [
    audioTrack,
    availableModes,
    mode,
    navigate,
    resolvedAudioTrack,
    resolvedMode,
    resolvedSubtitleTrack,
    subtitleTrack,
  ]);

  const savedProgress =
    watchProgressData?.error === false ? watchProgressData.data : null;
  const savedProgressSec = savedProgress?.progress_sec ?? null;
  const savedDurationSec = savedProgress?.duration_sec ?? null;
  const hasEligibleResumeProgress =
    savedProgressSec !== null &&
    savedDurationSec !== null &&
    savedDurationSec > 0 &&
    savedProgressSec >= WATCH_PROGRESS_MIN_SECONDS &&
    savedProgressSec / savedDurationSec < WATCH_PROGRESS_COMPLETION_THRESHOLD;
  const resumeDialogOpen =
    !resumeDismissed && start === 0 && !watchProgressPending && hasEligibleResumeProgress;

  const handleBack = () => {
    if (router.history.length > 1) {
      router.history.back();
    } else {
      navigate({ to: "/movies" });
    }
  };

  const clampPlaybackTime = (value: number) => {
    const knownDuration =
      durationRef.current > 0
        ? durationRef.current
        : duration > 0
          ? duration
          : value;
    return Math.max(0, Math.min(value, knownDuration));
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
      setStreamReloadKey(prev => prev + 1);
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

  const handleSessionLost = (currentTimeSec: number) => {
    const now = Date.now();
    if (sessionLostNavAttemptsRef.current >= HLS_SESSION_LOST_MAX_ATTEMPTS) {
      setPlaybackError(
        "Playback session could not be recovered. Try reloading the page or choosing another quality.",
      );
      return;
    }
    const tooSoon =
      sessionLostNavAttemptsRef.current > 0 &&
      now - lastSessionLostNavAtRef.current < HLS_SESSION_LOST_MIN_INTERVAL_MS;
    if (tooSoon) {
      return;
    }
    sessionLostNavAttemptsRef.current += 1;
    lastSessionLostNavAtRef.current = now;
    navigateToPlaybackPosition(currentTimeSec, { forceReload: true });
  };

  const togglePlay = async () => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      try {
        await video.play();
      } catch {
        setPlaybackError(
          "Playback failed — the browser could not play this stream.",
        );
      }
    } else {
      video.pause();
    }
  };

  const seek = (newTime: number) => {
    const video = videoRef.current;
    if (!video) return;
    const t = clampPlaybackTime(newTime);
    const currentVideoTime = toAbsolutePlaybackTime(video.currentTime);
    const shouldRebaseHlsSession =
      isHlsPlayback &&
      (t < hlsStartSec ||
        t > currentVideoTime + HLS_FORWARD_REBASE_THRESHOLD_SEC);

    if (shouldRebaseHlsSession) {
      navigateToPlaybackPosition(t);
      return;
    }

    video.currentTime = toMediaTime(t);
    setCurrentTime(t);
  };

  const seekForward = () => seek(currentTime + SEEK_STEP_SEC);
  const seekBackward = () => seek(currentTime - SEEK_STEP_SEC);

  const handleChapterSelect = (startTimeSec: number, title: string) => {
    seek(startTimeSec);
    setChapterAnnouncement((prev) => ({
      key: prev.key + 1,
      text: `Jumped to chapter: ${title}`,
    }));
  };

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
      toast.info(
        "Full screen isn't available in this browser. Using expanded view instead.",
      );
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

  // Video element event listeners (single source of truth — VideoPlayer renders a bare <video>)
  useEffect(() => {
    currentTimeRef.current = currentTime;
    durationRef.current = duration;
  }, [currentTime, duration]);

  useEffect(() => {
    if (!playing) return;
    const interval = window.setInterval(async () => {
      try {
        await persistMovieWatchProgress(
          movieId,
          currentTimeRef.current,
          durationRef.current,
        );
      } catch {
        // Silent background save failure; pause/end handlers surface failures when needed.
      }
    }, WATCH_PROGRESS_SAVE_INTERVAL_MS);
    return () => {
      window.clearInterval(interval);
    };
  }, [movieId, playing]);

  const handlePauseSave = async () => {
    try {
      await persistMovieWatchProgress(
        movieId,
        currentTimeRef.current,
        durationRef.current,
      );
    } catch {
      // Best effort on pause; avoid interrupting playback UI with repeated toasts.
    }
  };

  const handleEndedSave = async () => {
    try {
      await persistMovieWatchProgress(
        movieId,
        durationRef.current,
        durationRef.current,
      );
    } catch {
      showActionFailed(
        "save watch progress",
        "Unable to mark this movie as watched.",
      );
    }
  };

  useEffect(() => {
    const handlePageHide = () => {
      void persistMovieWatchProgress(
        movieId,
        currentTimeRef.current,
        durationRef.current,
        { keepalive: true },
      );
    };
    window.addEventListener("pagehide", handlePageHide);
    return () => {
      window.removeEventListener("pagehide", handlePageHide);
    };
  }, [movieId]);

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
    setDuration(movieDurationSec);
  }, [isHlsPlayback, movieDurationSec]);

  const handleNativePlaybackError = (code: number | null | undefined) => {
    if (code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
      setPlaybackError("This media format is not supported by the browser.");
    } else if (code === MediaError.MEDIA_ERR_DECODE) {
      setPlaybackError("The stream could not be decoded.");
    } else if (code === MediaError.MEDIA_ERR_NETWORK) {
      setPlaybackError("A network error interrupted playback.");
    } else {
      setPlaybackError("Playback failed.");
    }
  };

  useEffect(() => {
    const onFullscreenChange = () => {
      const entering = !!getFullscreenElement();
      if (entering) {
        fullscreenSourceRef.current = "document";
        setIsFullscreen(true);
        setIsImmersiveViewport(false);
        setControlsVisible(true);
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
      setControlsVisible(true);
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

  useEffect(() => {
    if (!chromeFullscreenMode) {
      if (idleTimerRef.current) {
        clearTimeout(idleTimerRef.current);
        idleTimerRef.current = null;
      }
      return;
    }
    scheduleHideControls();
    return () => {
      if (idleTimerRef.current) {
        clearTimeout(idleTimerRef.current);
        idleTimerRef.current = null;
      }
    };
  }, [chromeFullscreenMode]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const container = containerRef.current;
      const targetInsidePlayer = container?.contains(target) ?? false;
      const targetIsPageBody =
        target === document.body || target === document.documentElement;

      if (resumeDialogOpen) {
        return;
      }
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
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      if (getFullscreenElement() || isImmersiveViewport) {
        showControlsAndResetIdle();
      }

      switch (e.key) {
        case " ":
        case "k":
        case "K":
          e.preventDefault();
          e.stopPropagation();
          togglePlay();
          break;
        case "ArrowLeft":
        case "j":
        case "J":
          e.preventDefault();
          e.stopPropagation();
          seekBackward();
          break;
        case "ArrowRight":
        case "l":
        case "L":
          e.preventDefault();
          e.stopPropagation();
          seekForward();
          break;
        case "ArrowUp": {
          e.preventDefault();
          e.stopPropagation();
          const v = videoRef.current;
          if (v) {
            v.volume = Math.min(1, v.volume + VOLUME_STEP);
            v.muted = false;
          }
          break;
        }
        case "ArrowDown": {
          e.preventDefault();
          e.stopPropagation();
          const v = videoRef.current;
          if (v) {
            v.volume = Math.max(0, v.volume - VOLUME_STEP);
            v.muted = false;
          }
          break;
        }
        case "m":
        case "M": {
          e.preventDefault();
          e.stopPropagation();
          const v = videoRef.current;
          if (v) v.muted = !v.muted;
          break;
        }
        case "f":
        case "F":
          e.preventDefault();
          e.stopPropagation();
          toggleFullscreen();
          break;
        case "Home":
        case "0":
          e.preventDefault();
          e.stopPropagation();
          seek(0);
          break;
        case "Escape": {
          if (getFullscreenElement()) {
            e.preventDefault();
            e.stopPropagation();
            void exitDocumentFullscreen();
            break;
          }
          const v = videoRef.current;
          if (
            fullscreenSourceRef.current === "webkitVideo" &&
            v &&
            tryWebKitVideoExitFullscreen(v)
          ) {
            e.preventDefault();
            e.stopPropagation();
            break;
          }
          if (isImmersiveViewport) {
            e.preventDefault();
            e.stopPropagation();
            setIsImmersiveViewport(false);
          }
          break;
        }
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
    // Intentionally omit seekBackward, seekForward, togglePlay, and seek from deps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    currentTime,
    duration,
    isImmersiveViewport,
    resumeDialogOpen,
    toggleFullscreen,
  ]);

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

  if (movieNotFound) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4">
        <div className="max-w-md text-center">
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">
            Movie not found
          </h1>
          <p className="mb-6 text-slate-400">
            The movie could not be found or you don&apos;t have access to it.
          </p>
          <button
            ref={backButtonRef}
            onClick={handleBack}
            className="inline-flex items-center gap-2 rounded-full bg-cyan-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
            aria-label="Back to previous page"
          >
            <ArrowLeft className="size-5" aria-hidden="true" />
            Back
          </button>
        </div>
      </div>
    );
  }

  if (isPending || !movie) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4">
        <div className="text-center">
          <Spinner className="mx-auto mb-6 size-10 text-cyan-400" />
          <p className="text-lg font-medium text-white">Loading movie...</p>
        </div>
      </div>
    );
  }

  if (mode !== "direct" && techPending) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4">
        <div className="text-center">
          <Spinner className="mx-auto mb-6 size-10 text-cyan-400" />
          <p className="text-lg font-medium text-white">
            Preparing playback...
          </p>
        </div>
      </div>
    );
  }

  if (modeUnavailable) {
    const modeLabel = STREAM_MODES.find(m => m.id === mode)?.label ?? mode;
    return (
      <div
        ref={containerRef}
        className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4"
      >
        <div className="max-w-md text-center">
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">
            Quality not available
          </h1>
          <p className="mb-6 text-slate-400">
            <strong className="text-slate-200">{modeLabel}</strong> is not
            available for this movie. Go back and choose a different quality in
            Playback Settings.
          </p>
          <button
            ref={backButtonRef}
            onClick={handleBack}
            className="inline-flex items-center gap-2 rounded-full bg-cyan-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
            aria-label="Back to previous page"
          >
            <ArrowLeft className="size-5" aria-hidden="true" />
            Back
          </button>
        </div>
      </div>
    );
  }

  if (playbackError) {
    return (
      <div
        ref={containerRef}
        className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4"
      >
        <div className="max-w-md text-center">
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">
            Playback failed
          </h1>
          <p className="mb-6 text-slate-400">{playbackError}</p>
          <div className="flex items-center justify-center gap-3">
            <button
              onClick={() => {
                setPlaybackError(null);
                setPlaying(false);
                setCurrentTime(0);
                setDuration(0);
              }}
              className="inline-flex items-center gap-2 rounded-full bg-cyan-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
              aria-label="Try again"
            >
              <RotateCcw className="size-5" aria-hidden="true" />
              Try Again
            </button>
            <button
              ref={backButtonRef}
              onClick={handleBack}
              className="inline-flex items-center gap-2 rounded-full border border-slate-600 px-6 py-3 font-semibold text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
              aria-label="Back to previous page"
            >
              <ArrowLeft className="size-5" aria-hidden="true" />
              Back
            </button>
          </div>
        </div>
      </div>
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
          onError={msg => setPlaybackError(msg)}
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
            const absoluteTime = toAbsolutePlaybackTime(time);
            currentTimeRef.current = absoluteTime;
            setCurrentTime(absoluteTime);
          }}
          onDurationChange={(nextDuration) => {
            const absoluteDuration = toAbsoluteDuration(nextDuration);
            durationRef.current = absoluteDuration;
            setDuration(absoluteDuration);
          }}
          onNativeError={handleNativePlaybackError}
          subtitleTrack={subtitleInfo}
          startSec={isHlsPlayback ? hlsPlaybackOffsetSec : start}
          onStartApplied={(time) => {
            const absoluteTime = toAbsolutePlaybackTime(time);
            currentTimeRef.current = absoluteTime;
            setCurrentTime(absoluteTime);
          }}
          onSessionLost={(time) => handleSessionLost(toAbsolutePlaybackTime(time))}
        />
      </div>

      <footer
        className={
          chromeFullscreenMode
            ? `absolute inset-x-0 bottom-0 z-10 border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg transition-all duration-300 ease-out ${
                controlsVisible
                  ? "translate-y-0 opacity-100"
                  : "pointer-events-none translate-y-full opacity-0"
              }`
            : "shrink-0 border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg"
        }
      >
        <div className="mx-auto max-w-4xl">
          <div className="mb-4" role="group" aria-label="Playback progress">
            <ProgressBar
              variant="video"
              currentTime={currentTime}
              duration={duration}
              onSeek={seek}
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="flex min-w-25 items-center gap-2">
              <span className="text-sm text-slate-400 tabular-nums">
                {formatTimeSeconds(currentTime)}
              </span>
              <span className="text-slate-600">/</span>
              <span className="text-sm text-slate-400 tabular-nums">
                {formatTimeSeconds(duration)}
              </span>
            </div>

            <div
              className="flex items-center gap-2"
              role="group"
              aria-label="Playback controls"
            >
              <button
                onClick={seekBackward}
                className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label="Seek backward 10 seconds"
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>
              <button
                onClick={togglePlay}
                className="flex size-14 items-center justify-center rounded-full bg-cyan-500 text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                aria-label={playing ? "Pause" : "Play"}
              >
                {playing ? (
                  <Pause className="size-6 fill-current" aria-hidden="true" />
                ) : (
                  <Play className="size-6 fill-current" aria-hidden="true" />
                )}
              </button>
              <button
                onClick={seekForward}
                className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label="Seek forward 10 seconds"
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            <div className="flex min-w-25 items-center justify-end gap-2">
              <span
                className="rounded-sm bg-slate-800/80 px-2 py-1 text-xs text-slate-400"
                aria-label="Current stream quality"
              >
                {qualityLabel}
              </span>
              <ChapterMenu
                chapters={chapters}
                currentTimeSec={currentTime}
                onSelectChapter={handleChapterSelect}
                portalContainer={containerRef.current}
              />
              <VolumeControl
                mediaRef={videoRef}
                variant="minimized"
                accent="cyan"
              />
              <button
                onClick={toggleFullscreen}
                className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label={
                  chromeFullscreenMode
                    ? isImmersiveViewport && !isFullscreen
                      ? "Exit expanded view"
                      : "Exit fullscreen"
                    : "Fullscreen"
                }
                aria-pressed={chromeFullscreenMode}
              >
                {chromeFullscreenMode ? (
                  <Minimize className="size-5" aria-hidden="true" />
                ) : (
                  <Maximize className="size-5" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
