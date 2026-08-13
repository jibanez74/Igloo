import { useRef, useEffect, useEffectEvent, useState } from "react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
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
  Minimize,
  RotateCcw,
} from "lucide-react";
import ProgressBar from "@/components/playback/ProgressBar";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogDescription,
  DialogFullscreenContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import { trailerSearchSchema } from "@/lib/route-search";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { toast } from "sonner";
import { formatTimecode } from "@/lib/format";
import {
  MOVIE_SEEK_STEP_SEC,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  requestElementFullscreen,
} from "@/lib/fullscreen";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  loaderDeps: ({ search }) => ({
    mediaType: search.mediaType,
    mediaId: search.mediaId,
    videoKey: search.videoKey,
  }),
  loader: async ({ context, deps }) => {
    if (deps.mediaId && deps.mediaId > 0 && !deps.videoKey) {
      await context.queryClient.ensureQueryData(
        movieDetailsQueryOpts(deps.mediaId),
      );
    }
  },
  component: TrailerPage,
});

function focusMainAfterNavigation() {
  window.setTimeout(() => {
    document.getElementById("main")?.focus({ preventScroll: true });
  }, 0);
}

function handleDialogEscapeKeyDown(event: KeyboardEvent) {
  if (!getFullscreenElement()) {
    return;
  }

  event.preventDefault();
  void exitDocumentFullscreen();
}

function TrailerPage() {
  const { mediaType, mediaId, videoKey, returnTo } = Route.useSearch();
  const navigate = Route.useNavigate();
  const router = useRouter();
  const { pause, suspendKeyboard, resumeKeyboard } = useAudioPlayerActions();

  const containerRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const [isBrowserFullscreen, setIsBrowserFullscreen] = useState(
    () => !!getFullscreenElement(),
  );

  useEffect(() => {
    pause();
    suspendKeyboard();
    return () => resumeKeyboard();
  }, [pause, suspendKeyboard, resumeKeyboard]);

  const shouldFetchMovie =
    mediaType === "movie" && mediaId != null && mediaId > 0 && !videoKey;
  const {
    data,
    isPending: moviePending,
    isError: movieIsError,
    refetch: refetchMovie,
  } = useQuery({
    ...movieDetailsQueryOpts(mediaId ?? 0),
    enabled: shouldFetchMovie,
  });

  const media = data?.data?.movie;
  const trailerFromApi = media?.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube",
  );
  const trailerKey = videoKey ?? trailerFromApi?.key ?? null;

  const title = media?.title ? `${media.title} - Trailer` : "Trailer";

  const handleClose = () => {
    void router.navigate({ to: returnTo ?? "/" }).catch(() => {
      void navigate({ to: "/" });
    });

    focusMainAfterNavigation();
  };

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
    seekTo,
    seekForward,
    seekBackward,
    setVolume,
    toggleMute,
    retry,
  } = useYouTubePlayer({
    videoId: trailerKey,
    autoplay: true,
    controls: true,
    onEnd: handleClose,
  });

  const announcement = isPlaying ? `Playing: ${title}` : `Paused: ${title}`;

  useEffect(() => {
    const onFullscreenChange = () => {
      setIsBrowserFullscreen(!!getFullscreenElement());
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

  const toggleFullscreen = () => {
    const el = containerRef.current;
    if (!el) return;

    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return;
    }

    if (
      !canRequestElementFullscreen(el) ||
      !isDocumentFullscreenEntryLikely()
    ) {
      toast.info("Full screen isn't available in this browser.");
      return;
    }

    void requestElementFullscreen(el).catch(() => {
      toast.info("Full screen isn't available in this browser.");
    });
  };

  const handleKeyboardShortcut = useEffectEvent((e: KeyboardEvent) => {
    if (e.defaultPrevented) {
      return;
    }

    const target = e.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }

    // Text entry always owns the keyboard.
    if (
      target.isContentEditable ||
      target.closest('textarea, select, input:not([type="range"])')
    ) {
      return;
    }

    // Interactive controls keep the keys they consume natively; every other
    // key still works as a playback shortcut while dialog chrome is focused.
    const sliderKeys = [
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "ArrowDown",
      "Home",
      "End",
      "PageUp",
      "PageDown",
    ];
    if (
      target.closest('input[type="range"], [role="slider"]') &&
      sliderKeys.includes(e.key)
    ) {
      return;
    }

    if (
      target.closest("button, a[href]") &&
      (e.key === " " || e.key === "Enter")
    ) {
      return;
    }

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
        seekBackward(MOVIE_SEEK_STEP_SEC);
        break;
      case "ArrowRight":
      case "l":
      case "L":
        e.preventDefault();
        seekForward(MOVIE_SEEK_STEP_SEC);
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
        seekTo(0);
        break;
    }
  });

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      handleKeyboardShortcut(e);
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const handleDialogOpenChange = (open: boolean) => {
    if (!open) {
      handleClose();
    }
  };

  const handleDialogOpenAutoFocus = (event: Event) => {
    event.preventDefault();

    const focusTarget = closeButtonRef.current ?? containerRef.current;
    focusTarget?.focus({ preventScroll: true });
  };

  if (error) {
    return (
      <Dialog open onOpenChange={handleDialogOpenChange}>
        <DialogFullscreenContent
          ref={containerRef}
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "flex items-center justify-center bg-linear-to-b from-card via-background to-card",
          )}
          onOpenAutoFocus={handleDialogOpenAutoFocus}
          onEscapeKeyDown={handleDialogEscapeKeyDown}
        >
          <div className="max-w-md px-4 text-center">
            <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-destructive/10">
              <AlertCircle className="size-10 text-destructive" aria-hidden="true" />
            </div>
            <DialogTitle className="mb-2 text-xl font-semibold text-foreground">
              Unable to Play Trailer
            </DialogTitle>
            <DialogDescription className="mb-6 text-muted-foreground">
              {error}
            </DialogDescription>
            <div className="flex flex-wrap items-center justify-center gap-3">
              <button
                type="button"
                ref={closeButtonRef}
                onClick={retry}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex items-center rounded-full bg-primary px-6 py-3 font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
                )}
              >
                <RotateCcw className="mr-2 size-4" aria-hidden="true" />
                Try Again
              </button>
              <button
                type="button"
                onClick={handleClose}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex items-center rounded-full border border-border px-6 py-3 font-semibold text-foreground hover:bg-muted focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
                )}
              >
                <ArrowLeft className="mr-2 size-4" aria-hidden="true" />
                Go Back
              </button>
            </div>
          </div>
        </DialogFullscreenContent>
      </Dialog>
    );
  }

  if (!trailerKey && shouldFetchMovie && moviePending) {
    return (
      <Dialog open onOpenChange={handleDialogOpenChange}>
        <DialogFullscreenContent
          ref={containerRef}
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "flex items-center justify-center bg-linear-to-b from-card via-background to-card",
          )}
          onOpenAutoFocus={handleDialogOpenAutoFocus}
          onEscapeKeyDown={handleDialogEscapeKeyDown}
        >
          <DialogTitle className="sr-only">Loading trailer</DialogTitle>
          <DialogDescription className="sr-only">
            Please wait while the trailer loads.
          </DialogDescription>

          <div className="text-center">
            <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-primary/10">
              <Spinner className="size-10 text-primary" />
            </div>
            <p className="text-lg font-medium text-foreground">Loading trailer...</p>
            <p className="mt-2 text-sm text-muted-foreground">Please wait</p>
          </div>
        </DialogFullscreenContent>
      </Dialog>
    );
  }

  if (!trailerKey && (movieIsError || data?.error)) {
    return (
      <Dialog open onOpenChange={handleDialogOpenChange}>
        <DialogFullscreenContent
          ref={containerRef}
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "flex items-center justify-center bg-linear-to-b from-card via-background to-card",
          )}
          onOpenAutoFocus={handleDialogOpenAutoFocus}
          onEscapeKeyDown={handleDialogEscapeKeyDown}
        >
          <div className="max-w-md px-4 text-center">
            <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-destructive/10">
              <AlertCircle className="size-10 text-destructive" aria-hidden="true" />
            </div>
            <DialogTitle className="mb-2 text-xl font-semibold text-foreground">
              Unable to Load Trailer
            </DialogTitle>
            <DialogDescription className="mb-6 text-muted-foreground">
              {data?.message || "Something went wrong while loading the trailer."}
            </DialogDescription>
            <div className="flex flex-wrap items-center justify-center gap-3">
              <button
                type="button"
                ref={closeButtonRef}
                onClick={() => void refetchMovie()}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex items-center rounded-full bg-primary px-6 py-3 font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
                )}
              >
                <RotateCcw className="mr-2 size-4" aria-hidden="true" />
                Try Again
              </button>
              <button
                type="button"
                onClick={handleClose}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "inline-flex items-center rounded-full border border-border px-6 py-3 font-semibold text-foreground hover:bg-muted focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
                )}
              >
                <ArrowLeft className="mr-2 size-4" aria-hidden="true" />
                Go Back
              </button>
            </div>
          </div>
        </DialogFullscreenContent>
      </Dialog>
    );
  }

  if (!trailerKey) {
    return (
      <Dialog open onOpenChange={handleDialogOpenChange}>
        <DialogFullscreenContent
          ref={containerRef}
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "flex items-center justify-center bg-linear-to-b from-card via-background to-card",
          )}
          onOpenAutoFocus={handleDialogOpenAutoFocus}
          onEscapeKeyDown={handleDialogEscapeKeyDown}
        >
          <div className="max-w-md px-4 text-center">
            <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-muted">
              <Film className="size-10 text-muted-foreground" aria-hidden="true" />
            </div>
            <DialogTitle className="mb-2 text-xl font-semibold text-foreground">
              No Trailer Available
            </DialogTitle>
            <DialogDescription className="mb-6 text-muted-foreground">
              This movie doesn't have a trailer yet.
            </DialogDescription>
            <button
              type="button"
              ref={closeButtonRef}
              onClick={handleClose}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "rounded-full bg-primary px-6 py-3 font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
              )}
            >
              <ArrowLeft className="mr-2 size-4" aria-hidden="true" />
              Go Back
            </button>
          </div>
        </DialogFullscreenContent>
      </Dialog>
    );
  }

  const isLoading = trailerKey && !isReady;
  const isEffectivelyMuted = isMuted || volume === 0;

  return (
    <Dialog open onOpenChange={handleDialogOpenChange}>
      <DialogFullscreenContent
        ref={containerRef}
        className={cn(
          MOTION_MEDIA_OVERLAY_ENTER_CLASS,
          "flex flex-col bg-linear-to-b from-card via-background to-card",
        )}
        onOpenAutoFocus={handleDialogOpenAutoFocus}
        onEscapeKeyDown={handleDialogEscapeKeyDown}
      >
      <div className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </div>

      <DialogDescription className="sr-only">
        Keyboard shortcuts: Space or K to play/pause, J or left arrow to rewind
        {MOVIE_SEEK_STEP_SEC} seconds, L or right arrow to forward{" "}
        {MOVIE_SEEK_STEP_SEC} seconds, up/down arrows for volume, M to mute, F
        for fullscreen, Escape to exit fullscreen or close.
      </DialogDescription>

      <header
        className={cn(
          MOTION_PLAYER_CHROME_PANEL_CLASS,
          "flex items-center justify-between border-b border-border/50 bg-card/95 px-4 py-3 backdrop-blur-lg",
        )}
      >
        <div className="flex items-center gap-3">
          <Film className="size-5 text-primary" aria-hidden="true" />
          <div>
            <DialogTitle className="truncate text-base font-semibold text-foreground">
              {title}
            </DialogTitle>
            <p className="text-xs text-muted-foreground">Now Playing</p>
          </div>
        </div>
        <button
          type="button"
          ref={closeButtonRef}
          onClick={handleClose}
          className={cn(
            MOTION_PLAYER_CHROME_BUTTON_CLASS,
            "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
          )}
          aria-label="Close trailer (Escape)"
        >
          <X className="size-5" aria-hidden="true" />
        </button>
      </header>

      <div className="relative flex flex-1 items-center justify-center p-4">
        <div className="aspect-video w-full max-w-6xl">
          <div ref={playerContainerRef} className="size-full" />
        </div>

        {isLoading && (
          <div
            className={cn(
              MOTION_MEDIA_OVERLAY_ENTER_CLASS,
              "absolute inset-0 flex items-center justify-center bg-background/90 backdrop-blur-sm",
            )}
          >
            <div className="text-center">
              <div className="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-primary/10">
                <Spinner className="size-8 text-primary" />
              </div>
              <p className="font-medium text-foreground">Loading trailer...</p>
            </div>
          </div>
        )}
      </div>

      <footer
        className={cn(
          MOTION_PLAYER_CHROME_PANEL_CLASS,
          "border-t border-border/50 bg-card/95 p-4 backdrop-blur-lg",
        )}
      >
        <div className="mx-auto max-w-4xl">
          <ProgressBar
            variant="trailer"
            currentTime={currentTime}
            duration={duration}
            onSeek={seekTo}
            ariaLabel="Seek through trailer"
          />

          <div className="flex items-center justify-between">
            <div className="flex min-w-[100px] items-center gap-2">
              <span className="text-sm text-muted-foreground tabular-nums">
                {formatTimecode(currentTime, {
                  forceHours: duration >= 3600,
                })}
              </span>
              <span className="text-muted-foreground">/</span>
              <span className="text-sm text-muted-foreground tabular-nums">
                {formatTimecode(duration)}
              </span>
            </div>

            <div
              className="flex items-center gap-2"
              role="group"
              aria-label="Playback controls"
            >
              <button
                type="button"
                onClick={() => seekBackward(MOVIE_SEEK_STEP_SEC)}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
                )}
                aria-label={`Rewind ${MOVIE_SEEK_STEP_SEC} seconds (J or Left Arrow)`}
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>

              <button
                type="button"
                onClick={togglePlay}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
                )}
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

              <button
                type="button"
                onClick={() => seekForward(MOVIE_SEEK_STEP_SEC)}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
                )}
                aria-label={`Forward ${MOVIE_SEEK_STEP_SEC} seconds (L or Right Arrow)`}
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            <div className="flex min-w-[100px] items-center justify-end gap-2">
              <button
                type="button"
                onClick={toggleMute}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
                )}
                aria-label={isEffectivelyMuted ? "Unmute (M)" : "Mute (M)"}
                aria-pressed={isEffectivelyMuted}
              >
                {isEffectivelyMuted ? (
                  <VolumeX className="size-5" aria-hidden="true" />
                ) : volume < 50 ? (
                  <Volume1 className="size-5" aria-hidden="true" />
                ) : (
                  <Volume2 className="size-5" aria-hidden="true" />
                )}
              </button>

              <button
                type="button"
                onClick={toggleFullscreen}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
                )}
                aria-label={
                  isBrowserFullscreen ? "Exit fullscreen (F)" : "Fullscreen (F)"
                }
                aria-pressed={isBrowserFullscreen}
              >
                {isBrowserFullscreen ? (
                  <Minimize className="size-5" aria-hidden="true" />
                ) : (
                  <Maximize className="size-5" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>
        </div>
      </footer>
      </DialogFullscreenContent>
    </Dialog>
  );
}
