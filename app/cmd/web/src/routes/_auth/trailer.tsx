import { useRef, useEffect } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import {
  AlertCircle,
  ArrowLeft,
  Film,
  X,
  Rewind,
  FastForward,
  Pause,
  Play,
  VolumeX,
  Volume1,
  Volume2,
  Maximize,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";

// Search params schema with Zod validation
const trailerSearchSchema = z.object({
  mediaType: z.enum(["movie", "tv"]),
  mediaId: z.coerce.number().int().positive(),
  returnTo: z.string().optional(),
});

export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  component: TrailerPage,
});

function TrailerPage() {
  const { mediaType, mediaId, returnTo } = Route.useSearch();
  const navigate = useNavigate();

  const containerRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Fetch media details based on type
  // TODO: Add tvDetailsQueryOpts when TV shows are implemented
  const query =
    mediaType === "movie"
      ? movieDetailsQueryOpts(mediaId)
      : movieDetailsQueryOpts(mediaId);

  const { data } = useQuery(query);

  // TODO: Handle TV shows when implemented
  const media = data?.data?.movie;
  const trailer = media?.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube"
  );

  const title = media?.title ? `${media.title} - Trailer` : "Trailer";

  // Navigate back to origin page
  const handleClose = () => {
    if (returnTo) {
      navigate({ to: returnTo, params: { id: String(mediaId) } });
    } else {
      navigate({ to: "/" });
    }
  };

  // YouTube player hook
  const {
    containerRef: playerContainerRef,
    isReady,
    isPlaying,
    currentTime,
    duration,
    volume,
    isMuted,
    error,
    togglePlay,
    seekForward,
    seekBackward,
    setVolume,
    toggleMute,
  } = useYouTubePlayer({
    videoId: trailer?.key ?? null,
    autoplay: true,
    controls: true,
    onEnd: handleClose,
  });

  // Screen reader announcement
  const announcement = isPlaying ? `Playing: ${title}` : `Paused: ${title}`;

  // Progress percentage for the progress bar
  const progress = duration > 0 ? (currentTime / duration) * 100 : 0;

  // Handle clicking on the progress bar to seek
  const handleProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!duration) return;

    const target = e.currentTarget;
    const rect = target.getBoundingClientRect();
    const clickX = e.clientX - rect.left;
    const percentage = clickX / rect.width;
    const newTime = percentage * duration;

    // Use the hook's seekTo through seekBackward/Forward logic
    const diff = newTime - currentTime;
    if (diff > 0) {
      seekForward(diff);
    } else {
      seekBackward(-diff);
    }
  };

  // Handle keyboard navigation on progress bar
  const handleProgressKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      seekBackward(5);
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      seekForward(5);
    }
  };

  // Fullscreen toggle
  const toggleFullscreen = () => {
    if (!containerRef.current) return;

    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      containerRef.current.requestFullscreen();
    }
  };

  // Focus the close button when component mounts
  useEffect(() => {
    const timer = setTimeout(() => {
      closeButtonRef.current?.focus();
    }, 50);
    return () => clearTimeout(timer);
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Skip when user is typing in an input field
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      // Skip when modifier keys are pressed (allow browser shortcuts)
      if (e.ctrlKey || e.metaKey || e.altKey) {
        return;
      }

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
          seekBackward(10);
          break;
        case "ArrowRight":
        case "l":
        case "L":
          e.preventDefault();
          seekForward(10);
          break;
        case "ArrowUp":
          e.preventDefault();
          setVolume(Math.min(100, volume + 10));
          break;
        case "ArrowDown":
          e.preventDefault();
          setVolume(Math.max(0, volume - 10));
          break;
        case "m":
        case "M":
          e.preventDefault();
          toggleMute();
          break;
        case "f":
        case "F":
          e.preventDefault();
          toggleFullscreen();
          break;
        case "Home":
        case "0":
          e.preventDefault();
          seekBackward(currentTime); // Seek to start
          break;
        case "Escape":
          e.preventDefault();
          handleClose();
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  });

  // Focus trap within dialog
  const handleContainerKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Tab" && containerRef.current) {
      const focusableElements =
        containerRef.current.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );

      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];

      if (e.shiftKey && document.activeElement === firstElement) {
        e.preventDefault();
        lastElement?.focus();
      } else if (!e.shiftKey && document.activeElement === lastElement) {
        e.preventDefault();
        firstElement?.focus();
      }
    }
  };

  // Error state
  if (error) {
    return (
      <div
        ref={containerRef}
        role='dialog'
        aria-modal='true'
        aria-label='Error playing trailer'
        className='fixed inset-0 z-50 flex items-center justify-center bg-linear-to-b from-slate-900 via-slate-950 to-slate-900'
      >
        <div className='max-w-md px-4 text-center'>
          <div className='mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-red-500/10'>
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
          <h2 className='mb-2 text-xl font-semibold text-white'>
            Unable to Play Trailer
          </h2>
          <p className='mb-6 text-slate-400'>{error}</p>
          <button
            ref={closeButtonRef}
            onClick={handleClose}
            className='rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
          >
            <ArrowLeft className="mr-2 size-4" aria-hidden="true" />
            Go Back
          </button>
        </div>
      </div>
    );
  }

  // No trailer available
  if (!trailer && data) {
    return (
      <div
        ref={containerRef}
        role='dialog'
        aria-modal='true'
        aria-label='No trailer available'
        className='fixed inset-0 z-50 flex items-center justify-center bg-linear-to-b from-slate-900 via-slate-950 to-slate-900'
      >
        <div className='max-w-md px-4 text-center'>
          <div className='mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-slate-800'>
            <Film className="size-10 text-slate-400" aria-hidden="true" />
          </div>
          <h2 className='mb-2 text-xl font-semibold text-white'>
            No Trailer Available
          </h2>
          <p className='mb-6 text-slate-400'>
            This movie doesn't have a trailer yet.
          </p>
          <button
            ref={closeButtonRef}
            onClick={handleClose}
            className='rounded-full bg-amber-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
          >
            <ArrowLeft className="mr-2 size-4" aria-hidden="true" />
            Go Back
          </button>
        </div>
      </div>
    );
  }

  // Show loading while fetching movie data (before we know if there's a trailer)
  if (!data) {
    return (
      <div
        role='dialog'
        aria-modal='true'
        aria-label='Loading trailer'
        className='fixed inset-0 z-50 flex items-center justify-center bg-linear-to-b from-slate-900 via-slate-950 to-slate-900'
      >
        <div className='text-center'>
          <div className='mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-amber-500/10'>
            <Spinner className="size-10 text-amber-400" />
          </div>
          <p className='text-lg font-medium text-white'>Loading trailer...</p>
          <p className='mt-2 text-sm text-slate-400'>Please wait</p>
        </div>
      </div>
    );
  }

  // Loading state for player - render container but show overlay
  const isLoading = trailer && !isReady;

  return (
    <div
      ref={containerRef}
      role='dialog'
      aria-modal='true'
      aria-label={title}
      onKeyDown={handleContainerKeyDown}
      className='fixed inset-0 z-50 flex flex-col bg-linear-to-b from-slate-900 via-slate-950 to-slate-900'
    >
      {/* Screen reader live region for playback status */}
      <div className='sr-only' aria-live='polite' aria-atomic='true'>
        {announcement}
      </div>

      {/* Screen reader instructions */}
      <p className='sr-only'>
        Keyboard shortcuts: Space or K to play/pause, J or left arrow to rewind
        10 seconds, L or right arrow to forward 10 seconds, up/down arrows for
        volume, M to mute, F for fullscreen, Escape to close.
      </p>

      {/* Header with close button */}
      <header className='flex items-center justify-between border-b border-slate-700/50 bg-slate-900/95 px-4 py-3 backdrop-blur-lg'>
        <div className='flex items-center gap-3'>
          <Film className="size-5 text-amber-400" aria-hidden="true" />
          <div>
            <h1 className='truncate text-base font-semibold text-white'>
              {title}
            </h1>
            <p className='text-xs text-slate-400'>Now Playing</p>
          </div>
        </div>
        <button
          ref={closeButtonRef}
          onClick={handleClose}
          className='flex h-10 w-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
          aria-label='Close trailer (Escape)'
        >
          <X className="size-5" aria-hidden="true" />
        </button>
      </header>

      {/* Video container */}
      <div className='relative flex flex-1 items-center justify-center p-4'>
        <div className='aspect-video w-full max-w-6xl'>
          <div ref={playerContainerRef} className='h-full w-full' />
        </div>

        {/* Loading overlay while player initializes */}
        {isLoading && (
          <div className='absolute inset-0 flex items-center justify-center bg-slate-950/90 backdrop-blur-sm'>
            <div className='text-center'>
              <div className='mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-amber-500/10'>
                <Spinner className="size-8 text-amber-400" />
              </div>
              <p className='font-medium text-white'>Loading trailer...</p>
            </div>
          </div>
        )}
      </div>

      {/* Control bar */}
      <footer className='border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg'>
        <div className='mx-auto max-w-4xl'>
          {/* Progress bar */}
          <div className='mb-4' role='group' aria-label='Playback progress'>
            <div
              onClick={handleProgressClick}
              onKeyDown={handleProgressKeyDown}
              tabIndex={0}
              className='group relative h-1.5 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:outline-none'
              role='slider'
              aria-label='Seek through trailer'
              aria-valuenow={Math.round(currentTime)}
              aria-valuemin={0}
              aria-valuemax={Math.round(duration)}
              aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
            >
              <div
                className='absolute inset-y-0 left-0 rounded-full bg-amber-400 transition-all'
                style={{ width: `${progress}%` }}
              />
              <div
                className='absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md transition-opacity group-hover:opacity-100 group-focus:opacity-100'
                style={{ left: `calc(${progress}% - 6px)` }}
              />
            </div>
          </div>

          {/* Controls row */}
          <div className='flex items-center justify-between'>
            {/* Time display */}
            <div className='flex min-w-[100px] items-center gap-2'>
              <span className='text-sm text-slate-400 tabular-nums'>
                {formatTime(currentTime)}
              </span>
              <span className='text-slate-600'>/</span>
              <span className='text-sm text-slate-400 tabular-nums'>
                {formatTime(duration)}
              </span>
            </div>

            {/* Playback controls */}
            <div
              className='flex items-center gap-2'
              role='group'
              aria-label='Playback controls'
            >
              {/* Rewind 10s */}
              <button
                onClick={() => seekBackward(10)}
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                aria-label='Rewind 10 seconds (J or Left Arrow)'
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>

              {/* Play/Pause */}
              <button
                onClick={togglePlay}
                className='flex h-14 w-14 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
                aria-label={
                  isPlaying ? "Pause (Space or K)" : "Play (Space or K)"
                }
              >
                {isPlaying ? (
                  <Pause className="size-6 fill-current" aria-hidden="true" />
                ) : (
                  <Play className="size-6 fill-current" aria-hidden="true" />
                )}
              </button>

              {/* Forward 10s */}
              <button
                onClick={() => seekForward(10)}
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                aria-label='Forward 10 seconds (L or Right Arrow)'
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            {/* Volume and fullscreen controls */}
            <div className='flex min-w-[100px] items-center justify-end gap-2'>
              {/* Volume button */}
              <button
                onClick={toggleMute}
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                aria-label={isMuted ? "Unmute (M)" : "Mute (M)"}
              >
                {isMuted || volume === 0 ? (
                  <VolumeX className="size-5" aria-hidden="true" />
                ) : volume < 50 ? (
                  <Volume1 className="size-5" aria-hidden="true" />
                ) : (
                  <Volume2 className="size-5" aria-hidden="true" />
                )}
              </button>

              {/* Fullscreen button */}
              <button
                onClick={toggleFullscreen}
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                aria-label='Toggle fullscreen (F)'
              >
                <Maximize className="size-5" aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}

// Helper function to format time in mm:ss
function formatTime(seconds: number): string {
  if (!isFinite(seconds) || isNaN(seconds)) return "0:00";
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}
