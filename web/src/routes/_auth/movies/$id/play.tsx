import { useRef, useEffect, useState } from "react";
import {
  createFileRoute,
  useRouter,
} from "@tanstack/react-router";
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
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import MovieVideo from "@/components/VideoPlayer";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { formatTimeSeconds } from "@/lib/format";

export const Route = createFileRoute("/_auth/movies/$id/play")({
  component: PlayMoviePage,
});

const SEEK_STEP_SEC = 10;
const VOLUME_STEP = 0.1;
const CONTROLS_IDLE_MS = 3000;

function PlayMoviePage() {
  const { id } = Route.useParams();
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

  const streamUrl = `/api/movies/${movieId}/stream`;

  const { data, isPending, isError } = useQuery(
    libraryMovieDetailsQueryOpts(movieId),
  );
  const movie = data && !data.error ? data.data?.movie : null;
  const title = movie?.title ?? "Movie";
  const movieNotFound = isError || (data && data.error);

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
      video.play().catch(() => setPlaybackError("Playback failed"));
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

  const seekForward = () => {
    seek(currentTime + SEEK_STEP_SEC);
  };

  const seekBackward = () => {
    seek(currentTime - SEEK_STEP_SEC);
  };

  const toggleFullscreen = () => {
    const el = containerRef.current;
    if (!el) return;
    const isCurrentlyFullscreen = !!(
      document.fullscreenElement ?? (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement
    );
    if (isCurrentlyFullscreen) {
      const exitFs =
        document.exitFullscreen ??
        (document as Document & { webkitExitFullscreen?: () => Promise<void> }).webkitExitFullscreen;
      exitFs?.call(document);
    } else {
      const requestFs =
        el.requestFullscreen ??
        (el as HTMLElement & { webkitRequestFullscreen?: () => Promise<void> }).webkitRequestFullscreen;
      requestFs?.call(el);
    }
  };

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onTimeUpdate = () => setCurrentTime(video.currentTime);
    const onDurationChange = () => setDuration(video.duration);
    const onError = () => setPlaybackError("Playback failed");
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
        (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement;
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
        target.isContentEditable
      ) {
        return;
      }
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      const fullscreenEl =
        document.fullscreenElement ??
        (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement;
      if (fullscreenEl) showControlsAndResetIdle();

      switch (e.key) {
        case " ":
        case "k":
        case "K":
          e.preventDefault();
          togglePlay();
          break;
        case "ArrowLeft":
        case "j":
        case "J":
          e.preventDefault();
          seekBackward();
          break;
        case "ArrowRight":
        case "l":
        case "L":
          e.preventDefault();
          seekForward();
          break;
        case "ArrowUp": {
          e.preventDefault();
          const v = videoRef.current;
          if (v) {
            v.volume = Math.min(1, v.volume + VOLUME_STEP);
            v.muted = false;
          }
          break;
        }
        case "ArrowDown": {
          e.preventDefault();
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
          const v = videoRef.current;
          if (v) v.muted = !v.muted;
          break;
        }
        case "f":
        case "F":
          e.preventDefault();
          toggleFullscreen();
          break;
        case "Home":
        case "0":
          e.preventDefault();
          seek(0);
          break;
        case "Escape": {
          const fullscreenEl =
            document.fullscreenElement ??
            (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement;
          if (fullscreenEl) {
            e.preventDefault();
            const exitFs =
              document.exitFullscreen ??
              (document as Document & { webkitExitFullscreen?: () => Promise<void> }).webkitExitFullscreen;
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
        'button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])',
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
          <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">
            Movie not found
          </h1>
          <p className="mb-6 text-slate-400">
            The movie could not be found or you don't have access to it.
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

  if (playbackError) {
    return (
      <div
        ref={containerRef}
        className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4"
      >
        <div className="max-w-md text-center">
          <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-white">
            Playback failed
          </h1>
          <p className="mb-6 text-slate-400">
            The video could not be played. The file may be missing or in an
            unsupported format.
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
            ? `absolute top-0 right-0 left-0 z-10 flex items-center justify-between border-b border-slate-700/50 bg-slate-900/95 px-4 py-3 backdrop-blur-lg transition-all duration-300 ease-out ${
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
          className="flex h-10 w-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
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
            ? (e) => {
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
        <MovieVideo
        videoRef={videoRef}
        src={streamUrl}
        title={title}
        isFullscreen={isFullscreen}
        onTimeUpdate={() => setCurrentTime(videoRef.current?.currentTime ?? 0)}
        onDurationChange={() => setDuration(videoRef.current?.duration ?? 0)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onError={() => setPlaybackError("Playback failed")}
      />
      </div>

      <footer
        className={
          isFullscreen
            ? `absolute right-0 bottom-0 left-0 z-10 border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg transition-all duration-300 ease-out ${
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
                className="flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label="Seek backward 10 seconds"
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>
              <button
                onClick={togglePlay}
                className="flex h-14 w-14 items-center justify-center rounded-full bg-cyan-500 text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
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
                className="flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
                aria-label="Seek forward 10 seconds"
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            <div className="flex min-w-25 items-center justify-end gap-2">
              <VolumeControl
                mediaRef={videoRef}
                variant="minimized"
                accent="cyan"
              />
              <button
                onClick={toggleFullscreen}
                className="flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
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
