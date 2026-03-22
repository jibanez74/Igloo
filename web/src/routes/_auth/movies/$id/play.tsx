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
import {
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
import { formatTimeSeconds } from "@/lib/format";
import {
  STREAM_MODES,
  getAvailableModes,
  type StreamModeId,
} from "@/lib/playback";
import { playSearchSchema } from "@/types/movie-play";

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

function buildStreamUrl(
  movieId: number,
  mode: StreamModeId,
  audioTrack: number,
): string {
  if (mode === "direct") return `/api/movies/${movieId}/stream`;
  return `/api/movies/${movieId}/hls/${mode}/playlist.m3u8?audio_track=${audioTrack}`;
}

function PlayMoviePage() {
  const { id } = Route.useParams();
  const { mode, audio_track: audioTrack } = Route.useSearch();
  const movieId = parseInt(id, 10);
  const navigate = Route.useNavigate();
  const router = useRouter();

  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const backButtonRef = useRef<HTMLButtonElement>(null);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [controlsVisible, setControlsVisible] = useState(true);
  const [playbackError, setPlaybackError] = useState<string | null>(null);

  const streamUrl = buildStreamUrl(movieId, mode, audioTrack);
  const qualityLabel = STREAM_MODES.find(m => m.id === mode)?.label ?? mode;

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
  const techLoaded = !techPending && techData?.data != null;
  const availableModes = techLoaded
    ? getAvailableModes(
        techData.data!.video_streams?.[0]?.height ?? 0,
        techData.data!.video_streams?.[0]?.codec,
        techData.data!.audio_streams?.[0]?.codec,
        techData.data!.movie?.mime_type,
      )
    : null;
  const modeUnavailable =
    availableModes !== null && !availableModes.some(m => m.id === mode);

  const handleBack = () => {
    if (router.history.length > 1) {
      router.history.back();
    } else {
      navigate({ to: "/movies" });
    }
  };

  const togglePlay = () => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      video.play().catch(() => {
        setPlaybackError(
          "Playback failed — the browser could not play this stream.",
        );
      });
    } else {
      video.pause();
    }
  };

  const seek = (newTime: number) => {
    const video = videoRef.current;
    if (!video) return;
    const t = Math.max(0, Math.min(newTime, duration || 0));
    video.currentTime = t;
    setCurrentTime(t);
  };

  const seekForward = () => seek(currentTime + SEEK_STEP_SEC);
  const seekBackward = () => seek(currentTime - SEEK_STEP_SEC);

  const toggleFullscreen = () => {
    const el = containerRef.current;
    if (!el) return;
    const isCurrentlyFullscreen = !!(
      document.fullscreenElement ??
      (document as Document & { webkitFullscreenElement?: Element })
        .webkitFullscreenElement
    );
    if (isCurrentlyFullscreen) {
      const exitFs =
        document.exitFullscreen ??
        (
          document as Document & {
            webkitExitFullscreen?: () => Promise<void>;
          }
        ).webkitExitFullscreen;
      exitFs?.call(document);
    } else {
      const requestFs =
        el.requestFullscreen ??
        (
          el as HTMLElement & {
            webkitRequestFullscreen?: () => Promise<void>;
          }
        ).webkitRequestFullscreen;
      requestFs?.call(el);
    }
  };

  // Video element event listeners (single source of truth — VideoPlayer renders a bare <video>)
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onTimeUpdate = () => setCurrentTime(video.currentTime);
    const onDurationChange = () => setDuration(video.duration);
    const onError = () => {
      const code = video.error?.code;
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
    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("durationchange", onDurationChange);
    video.addEventListener("error", onError);
    return () => {
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("durationchange", onDurationChange);
      video.removeEventListener("error", onError);
    };
  }, []);

  useEffect(() => {
    const onFullscreenChange = () => {
      const fullscreenEl =
        document.fullscreenElement ??
        (document as Document & { webkitFullscreenElement?: Element })
          .webkitFullscreenElement;
      const entering = !!fullscreenEl;
      setIsFullscreen(entering);
      if (entering) setControlsVisible(true);
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
    if (!isFullscreen) {
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
  }, [isFullscreen]);

  useEffect(() => {
    const timer = setTimeout(() => backButtonRef.current?.focus(), 50);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.tagName === "SELECT" ||
        target.isContentEditable
      ) {
        return;
      }
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      const fullscreenEl =
        document.fullscreenElement ??
        (document as Document & { webkitFullscreenElement?: Element })
          .webkitFullscreenElement;
      if (fullscreenEl) showControlsAndResetIdle();

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
          const fse =
            document.fullscreenElement ??
            (document as Document & { webkitFullscreenElement?: Element })
              .webkitFullscreenElement;
          if (fse) {
            e.preventDefault();
            e.stopPropagation();
            const exitFs =
              document.exitFullscreen ??
              (
                document as Document & {
                  webkitExitFullscreen?: () => Promise<void>;
                }
              ).webkitExitFullscreen;
            exitFs?.call(document);
          }
          break;
        }
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [currentTime, duration]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleContainerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Tab" && containerRef.current) {
      const focusable = containerRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    }
  };

  const announcement = playing ? `Playing: ${title}` : `Paused: ${title}`;

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
      onKeyDown={handleContainerKeyDown}
      onMouseMove={isFullscreen ? showControlsAndResetIdle : undefined}
      onTouchStart={isFullscreen ? showControlsAndResetIdle : undefined}
      className="flex min-h-0 flex-1 flex-col bg-slate-900 [&:-webkit-full-screen]:fixed [&:-webkit-full-screen]:inset-0 [&:-webkit-full-screen]:h-screen [&:-webkit-full-screen]:w-screen [&:fullscreen]:fixed [&:fullscreen]:inset-0 [&:fullscreen]:h-screen [&:fullscreen]:w-screen"
      role="region"
      aria-label={`Video player for ${title}`}
    >
      <LiveAnnouncer message={announcement} politeness="polite" />

      <p className="sr-only">
        Keyboard shortcuts: Space or K to play/pause, J or Left arrow to rewind
        10 seconds, L or Right arrow to forward 10 seconds, Up/Down for volume,
        M to mute, F for fullscreen, Escape to exit fullscreen, Back button to
        go back.
      </p>

      <header
        className={
          isFullscreen
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
        onClick={isFullscreen ? togglePlay : undefined}
        role={isFullscreen ? "button" : undefined}
        tabIndex={isFullscreen ? 0 : undefined}
        onKeyDown={
          isFullscreen
            ? e => {
                if (e.key === " " || e.key === "Enter") {
                  e.preventDefault();
                  togglePlay();
                  showControlsAndResetIdle();
                }
              }
            : undefined
        }
        aria-label={isFullscreen ? "Play or pause" : undefined}
      >
        <VideoPlayer
          videoRef={videoRef}
          src={streamUrl}
          title={title}
          isFullscreen={isFullscreen}
          onError={msg => setPlaybackError(msg)}
        />
      </div>

      <footer
        className={
          isFullscreen
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
              <VolumeControl
                mediaRef={videoRef}
                variant="minimized"
                accent="cyan"
              />
              <button
                onClick={toggleFullscreen}
                className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
                aria-pressed={isFullscreen}
              >
                {isFullscreen ? (
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
