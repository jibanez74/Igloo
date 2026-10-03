import { Maximize, Minimize, Pause, Play } from "lucide-react";
import { useEffect, useEffectEvent, useRef, type RefObject } from "react";

import ProgressBar from "@/components/playback/ProgressBar";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { usePrerollQueue } from "@/hooks/usePrerollQueue";
import { useShortcutHints } from "@/hooks/useShortcutHints";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";
import {
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
  PREROLL_MOVIE_WARMUP_SEC,
} from "@/lib/constants";
import { formatTimecode } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PrerollTrailer } from "@/types";

type PrerollPlayerProps = {
  /** The queue; undefined while it is still being fetched. */
  trailers: PrerollTrailer[] | undefined;
  /** The queue request failed or timed out: the movie starts at once. */
  loadFailed: boolean;
  chromeFullscreenMode: boolean;
  controlsVisible: boolean;
  isFullscreen: boolean;
  isImmersiveViewport: boolean;
  /**
   * The "Skip trailer" button, the pre-roll's primary control. The page
   * also hands it to the resume dialog as its focus-restore target, since
   * the dialog closes (and restores focus) after the pre-roll has appeared.
   */
  skipButtonRef: RefObject<HTMLButtonElement | null>;
  /** The pre-roll is over, by any route; the movie starts. */
  onFinish: () => void;
  /**
   * The last trailer is in its final seconds: prepare the movie stream now
   * so it starts promptly, without having held a transcode slot all along.
   */
  onWarmup: () => void;
  onShowControls: () => void;
  onToggleFullscreen: () => void;
  onExitFullscreen: () => void;
};

/**
 * The trailer pre-roll: a phase inside the movie player that plays the
 * queued YouTube trailers in turn, with its own chrome in place of the
 * movie's. It stands over the (not yet, or just barely, mounted) movie
 * surface, so Back still leaves the whole play session in one step.
 */
export default function PrerollPlayer({
  trailers,
  loadFailed,
  chromeFullscreenMode,
  controlsVisible,
  isFullscreen,
  isImmersiveViewport,
  skipButtonRef,
  onFinish,
  onWarmup,
  onShowControls,
  onToggleFullscreen,
  onExitFullscreen,
}: PrerollPlayerProps) {
  const queue = usePrerollQueue({ trailers, loadFailed });
  const { loading, done, current, position, total, isLast } = queue;
  const warmupFiredRef = useRef(false);
  // An embed error arrives twice: through onError, which skips the trailer,
  // and as the hook's error state. An error state on a trailer that reported
  // no code means the player itself is unavailable (the API never loaded, or
  // the ready watchdog fired), which ends the pre-roll instead of costing a
  // watchdog timeout per trailer.
  const codeErrorKeyRef = useRef<string | null>(null);
  const { showShortcutHints, withShortcut } = useShortcutHints();

  const {
    containerRef,
    isReady,
    isPlaying,
    currentTime,
    duration,
    error,
    togglePlay,
    seekTo,
  } = useYouTubePlayer({
    videoId: current?.youtube_key ?? null,
    autoplay: true,
    controls: false,
    onEnd: queue.handleTrailerEnded,
    onError: () => {
      codeErrorKeyRef.current = current?.youtube_key ?? null;
      queue.handleTrailerError();
    },
  });

  const finish = useEffectEvent(onFinish);
  useEffect(() => {
    if (done) finish();
  }, [done]);

  const skipAll = queue.skipAll;
  const currentKey = current?.youtube_key ?? null;
  useEffect(() => {
    if (!error || codeErrorKeyRef.current === currentKey) return;
    skipAll();
  }, [error, currentKey, skipAll]);

  const warmup = useEffectEvent(onWarmup);
  useEffect(() => {
    if (!isLast || !isReady || warmupFiredRef.current) return;
    const nearEnd =
      duration <= 0 || duration - currentTime <= PREROLL_MOVIE_WARMUP_SEC;
    if (!nearEnd) return;
    warmupFiredRef.current = true;
    warmup();
  }, [isLast, isReady, duration, currentTime]);

  useEffect(() => {
    if (loading || done) return;
    skipButtonRef.current?.focus({ preventScroll: true });
  }, [loading, done, skipButtonRef]);

  const handleKeyboardShortcut = useEffectEvent((event: KeyboardEvent) => {
    if (
      event.defaultPrevented ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey
    ) {
      return;
    }
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    if (
      target.isContentEditable ||
      target.closest('textarea, select, input:not([type="range"])')
    ) {
      return;
    }
    const sliderKeys = [
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "ArrowDown",
      "Home",
      "End",
    ];
    if (
      target.closest('input[type="range"]') &&
      sliderKeys.includes(event.key)
    ) {
      return;
    }
    if (
      target.closest("button, a[href]") &&
      (event.key === " " || event.key === "Enter")
    ) {
      return;
    }

    switch (event.key) {
      case " ":
      case "k":
      case "K":
        event.preventDefault();
        togglePlay();
        break;
      case "n":
      case "N":
        event.preventDefault();
        queue.skip();
        break;
      case "s":
      case "S":
        event.preventDefault();
        queue.skipAll();
        break;
      case "f":
      case "F":
        event.preventDefault();
        onToggleFullscreen();
        break;
      case "Escape":
        onExitFullscreen();
        break;
      default:
        return;
    }
    onShowControls();
  });

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) =>
      handleKeyboardShortcut(event);
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const fullscreenLabel = chromeFullscreenMode
    ? isImmersiveViewport && !isFullscreen
      ? "Exit expanded view"
      : "Exit fullscreen"
    : "Fullscreen";

  return (
    // The layer sits on the player's fullscreen click-to-toggle surface; a
    // click on the trailer must not reach the movie warming up beneath it.
    // react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions
    <section
      aria-label="Trailers before the movie"
      onClick={(event) => event.stopPropagation()}
      className="absolute inset-0 z-20 flex flex-col bg-black"
    >
      <LiveAnnouncer
        message={
          current ? `Trailer ${position} of ${total}: ${current.title}` : ""
        }
        announcementKey={position}
        politeness="polite"
      />
      {showShortcutHints && (
        <p className="sr-only">
          Trailer keyboard shortcuts: Space or K to play or pause the trailer, N
          to skip the trailer, S to start the movie, F for fullscreen, Escape to
          exit fullscreen.
        </p>
      )}

      <div className="relative flex min-h-0 flex-1 items-center justify-center">
        {loading ? (
          <div
            role="status"
            className="flex items-center gap-3 rounded-full bg-background/80 px-5 py-3 backdrop-blur-sm"
          >
            <Spinner className="size-5 text-primary" aria-hidden="true" />
            <p className="text-sm font-medium text-foreground">
              Loading trailers…
            </p>
          </div>
        ) : null}
        {current ? (
          <div className="relative aspect-video max-h-full w-full max-w-6xl bg-black">
            <div ref={containerRef} className="size-full" />
            {!isReady ? (
              <div
                role="status"
                className="absolute inset-0 flex items-center justify-center bg-black"
              >
                <Spinner className="size-8 text-primary" aria-hidden="true" />
                <span className="sr-only">Loading trailer</span>
              </div>
            ) : null}
            {/*
              A pointer convenience over the embedded player: toggling here
              keeps focus and the keyboard in this document instead of the
              YouTube iframe. The footer play button is the keyboard route.
            */}
            {/* react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions */}
            <div
              aria-hidden="true"
              className="absolute inset-0"
              onClick={togglePlay}
            />
          </div>
        ) : null}
      </div>

      {current ? (
        <footer
          className={
            chromeFullscreenMode
              ? cn(
                  MOTION_PLAYER_CHROME_PANEL_CLASS,
                  "absolute inset-x-0 bottom-0 z-10 border-t border-border bg-background/95 p-4 backdrop-blur-lg",
                  controlsVisible
                    ? "translate-y-0 opacity-100"
                    : "pointer-events-none translate-y-full opacity-0",
                )
              : "shrink-0 border-t border-border bg-background/95 p-4 backdrop-blur-lg"
          }
        >
          <div className="mx-auto max-w-4xl">
            <div className="mb-4" role="group" aria-label="Trailer progress">
              <ProgressBar
                variant="trailer"
                currentTime={currentTime}
                duration={duration}
                onSeek={seekTo}
                ariaLabel="Seek through trailer"
                resetKey={current.youtube_key}
              />
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2 text-sm">
                <span className="shrink-0 text-muted-foreground tabular-nums">
                  Trailer {position} of {total}
                </span>
                <span className="text-muted-foreground" aria-hidden="true">
                  ·
                </span>
                <span className="truncate font-medium text-foreground">
                  {current.title}
                </span>
                <span className="shrink-0 text-muted-foreground tabular-nums">
                  {formatTimecode(currentTime)} / {formatTimecode(duration)}
                </span>
              </div>

              <div
                className="flex items-center gap-2"
                role="group"
                aria-label="Trailer controls"
              >
                <button
                  type="button"
                  onClick={togglePlay}
                  className={cn(
                    PLAYER_PRIMARY_BUTTON_CLASS,
                    "size-12 shadow-lg shadow-primary/20",
                  )}
                  aria-label={withShortcut(
                    isPlaying ? "Pause trailer" : "Play trailer",
                    "Space or K",
                  )}
                >
                  {isPlaying ? (
                    <Pause className="size-5 fill-current" aria-hidden="true" />
                  ) : (
                    <Play
                      className="ml-0.5 size-5 fill-current"
                      aria-hidden="true"
                    />
                  )}
                </button>
                <Button
                  type="button"
                  ref={skipButtonRef}
                  variant="outline"
                  size="sm"
                  onClick={queue.skip}
                  aria-label={withShortcut("Skip trailer", "N")}
                  className="border-border bg-muted text-foreground hover:bg-accent"
                >
                  Skip trailer
                </Button>
                <Button
                  type="button"
                  variant="accent"
                  size="sm"
                  onClick={queue.skipAll}
                  aria-label={withShortcut("Start movie", "S")}
                >
                  Start movie
                </Button>
                <button
                  type="button"
                  onClick={onToggleFullscreen}
                  className={cn(
                    PLAYER_ICON_BUTTON_CLASS,
                    "size-10 hover:bg-accent",
                  )}
                  aria-label={withShortcut(fullscreenLabel, "F")}
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
      ) : null}
    </section>
  );
}
