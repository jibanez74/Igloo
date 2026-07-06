import { useRef, useState, useEffect, useEffectEvent } from "react";
import {
  ChevronDown,
  ChevronUp,
  X,
  Disc3,
  SkipBack,
  SkipForward,
  Pause,
  Play,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogDescription,
  DialogFullscreenContent,
  DialogTitle,
} from "@/components/ui/dialog";
import type { TrackType } from "@/types";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";
import {
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type AudioPlayerProps = {
  track: TrackType | null;
  tracks: TrackType[];
  albumCover: string | null;
  albumTitle: string;
  musicianName: string | null;
  onTrackChange: (track: TrackType) => void;
  onClose?: () => void;
  audioRef: React.RefObject<HTMLAudioElement | null>;
  isPlaying: boolean;
  onPlayStateChange: (playing: boolean) => void;
  isExpanded: boolean;
  onMinimize: () => void;
  onExpand: () => void;
  isKeyboardSuspended?: boolean;
};

// "Previous" restarts the current track instead of navigating once playback
// has passed this many seconds.
const RESTART_THRESHOLD_SECONDS = 3;

// Controls whose native keyboard interaction must win over the global
// playback shortcuts.
const INTERACTIVE_SELECTOR =
  'button, a[href], select, summary, [role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"], [role="option"], [role="tab"], [role="radio"], [role="checkbox"], [role="switch"], [role="slider"], [role="spinbutton"]';

// Overlays that own their keyboard interaction entirely. The player's own
// fullscreen dialog is exempted via the data-audio-player marker.
const FOREIGN_OVERLAY_SELECTOR =
  '[role="dialog"]:not([data-audio-player]), [role="alertdialog"], [role="menu"], [role="listbox"]';

const ARROW_KEYS = new Set([
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "ArrowDown",
]);

function mediaSessionSupported() {
  return (
    typeof navigator !== "undefined" &&
    "mediaSession" in navigator &&
    typeof navigator.mediaSession?.setPositionState === "function"
  );
}

export default function AudioPlayer({
  track,
  tracks,
  albumCover,
  albumTitle,
  musicianName,
  onTrackChange,
  onClose,
  audioRef,
  isPlaying,
  onPlayStateChange,
  isExpanded,
  onMinimize,
  onExpand,
  isKeyboardSuspended = false,
}: AudioPlayerProps) {
  const playPauseButtonRef = useRef<HTMLButtonElement>(null);
  const expandButtonRef = useRef<HTMLButtonElement>(null);
  const shouldRestoreExpandFocusRef = useRef(false);

  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isLoading, setIsLoading] = useState(false);

  const artist = musicianName || albumTitle;

  // The dialog's accessible name already announces the current track on open, so
  // the live region stays silent on first render and only speaks on subsequent
  // play/pause toggles and track changes. We compare against the previous render
  // (adjusting state during render, per the React "you might not need an effect"
  // guidance) rather than seeding the live region with content on mount.
  const [announcement, setAnnouncement] = useState("");
  const [lastState, setLastState] = useState<{
    trackId: TrackType["id"] | null;
    isPlaying: boolean;
  }>({ trackId: track?.id ?? null, isPlaying });

  const currentTrackId = track?.id ?? null;
  if (
    currentTrackId !== lastState.trackId ||
    isPlaying !== lastState.isPlaying
  ) {
    if (!track) {
      setAnnouncement("");
    } else if (lastState.trackId !== null) {
      setAnnouncement(
        track.id !== lastState.trackId
          ? `Now playing: ${track.title} by ${artist}`
          : isPlaying
            ? "Playing"
            : "Paused",
      );
    }
    setLastState({ trackId: currentTrackId, isPlaying });
  }

  const currentIndex = track ? tracks.findIndex(t => t.id === track.id) : -1;
  const hasPrevious = currentIndex > 0;
  const hasNext = currentIndex < tracks.length - 1 && currentIndex !== -1;
  const prevAriaLabel = "Previous track";
  const nextAriaLabel = hasNext ? "Next track" : "No next track";
  const playPauseAriaLabel = isPlaying ? "Pause" : "Play";
  const streamUrl = track ? `/api/music/tracks/${track.id}/stream` : null;

  const playAudio = async () => {
    const audio = audioRef.current;
    if (!audio) return;

    try {
      await audio.play();
    } catch {
      // Autoplay can still be blocked by the browser in some cases.
    }
  };

  // Spotify-style previous: within the first few seconds go to the previous
  // track; otherwise (or when there is no previous track) restart the current
  // one. The button therefore never needs to be disabled.
  const playPrevious = () => {
    const audio = audioRef.current;

    if (hasPrevious && (audio?.currentTime ?? 0) <= RESTART_THRESHOLD_SECONDS) {
      onTrackChange(tracks[currentIndex - 1]);
      return;
    }

    if (audio) {
      audio.currentTime = 0;
      setCurrentTime(0);
    }
  };

  const handleMinimize = () => {
    shouldRestoreExpandFocusRef.current = true;
    onMinimize();
  };

  useEffect(() => {
    if (isExpanded || !shouldRestoreExpandFocusRef.current) {
      return;
    }

    shouldRestoreExpandFocusRef.current = false;

    const focusExpandButton = () => {
      expandButtonRef.current?.focus({ preventScroll: true });
    };

    if (typeof window.requestAnimationFrame === "function") {
      const frame = window.requestAnimationFrame(focusExpandButton);
      return () => window.cancelAnimationFrame(frame);
    }

    const timer = window.setTimeout(focusExpandButton, 0);
    return () => window.clearTimeout(timer);
  }, [isExpanded]);

  useEffect(() => {
    if (
      !track ||
      !("mediaSession" in navigator) ||
      typeof MediaMetadata === "undefined"
    ) {
      return;
    }

    const artworkUrl = albumCover
      ? albumCover.startsWith("http")
        ? albumCover
        : `${window.location.origin}${albumCover}`
      : null;

    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title,
      artist: musicianName ?? albumTitle,
      album: albumTitle,
      artwork: artworkUrl ? [{ src: artworkUrl }] : [],
    });
  }, [track, albumCover, albumTitle, musicianName]);

  useEffect(() => {
    if (!("mediaSession" in navigator)) return;

    navigator.mediaSession.playbackState = !track
      ? "none"
      : isPlaying
        ? "playing"
        : "paused";
  }, [isPlaying, track]);

  const handleMediaSessionPlay = useEffectEvent(() => {
    void playAudio();
  });

  const handleMediaSessionPause = useEffectEvent(() => {
    audioRef.current?.pause();
  });

  const handleMediaSessionStop = useEffectEvent(() => {
    onClose?.();
  });

  const handleMediaSessionPrevious = useEffectEvent(() => {
    playPrevious();
  });

  const handleMediaSessionNext = useEffectEvent(() => {
    if (hasNext) {
      onTrackChange(tracks[currentIndex + 1]);
    }
  });

  const handleMediaSessionSeekBackward = useEffectEvent(
    ({ seekOffset }: { seekOffset?: number }) => {
      const audio = audioRef.current;
      if (!audio) return;

      audio.currentTime = Math.max(
        0,
        audio.currentTime - (seekOffset ?? 10),
      );
    },
  );

  const handleMediaSessionSeekForward = useEffectEvent(
    ({ seekOffset }: { seekOffset?: number }) => {
      const audio = audioRef.current;
      if (!audio) return;

      const totalDuration = audio.duration || duration;
      audio.currentTime = Math.min(
        totalDuration,
        audio.currentTime + (seekOffset ?? 10),
      );
    },
  );

  const handleMediaSessionSeekTo = useEffectEvent(
    ({ seekTime }: { seekTime?: number }) => {
      const audio = audioRef.current;
      if (!audio || seekTime == null) return;

      audio.currentTime = seekTime;
    },
  );

  useEffect(() => {
    if (!track || !("mediaSession" in navigator)) return;

    navigator.mediaSession.setActionHandler("play", handleMediaSessionPlay);
    navigator.mediaSession.setActionHandler("pause", handleMediaSessionPause);
    navigator.mediaSession.setActionHandler("stop", handleMediaSessionStop);
    navigator.mediaSession.setActionHandler(
      "previoustrack",
      handleMediaSessionPrevious,
    );
    navigator.mediaSession.setActionHandler("nexttrack", handleMediaSessionNext);
    navigator.mediaSession.setActionHandler(
      "seekbackward",
      handleMediaSessionSeekBackward,
    );
    navigator.mediaSession.setActionHandler(
      "seekforward",
      handleMediaSessionSeekForward,
    );
    navigator.mediaSession.setActionHandler("seekto", handleMediaSessionSeekTo);

    return () => {
      for (const action of [
        "play",
        "pause",
        "stop",
        "previoustrack",
        "nexttrack",
        "seekbackward",
        "seekforward",
        "seekto",
      ] as const) {
        navigator.mediaSession.setActionHandler(action, null);
      }
    };
  }, [audioRef, track]);

  const syncMediaSessionPosition = useEffectEvent((audio: HTMLAudioElement) => {
    if (!mediaSessionSupported()) {
      return;
    }

    const audioDuration = audio.duration;
    if (!Number.isFinite(audioDuration) || audioDuration <= 0) {
      return;
    }

    const currentPosition = Number.isFinite(audio.currentTime)
      ? audio.currentTime
      : 0;
    const playbackRate =
      Number.isFinite(audio.playbackRate) && audio.playbackRate > 0
        ? audio.playbackRate
        : 1;

    try {
      navigator.mediaSession.setPositionState?.({
        duration: audioDuration,
        playbackRate,
        position: Math.max(0, Math.min(currentPosition, audioDuration)),
      });
    } catch {
      // Some browsers expose Media Session but reject position updates.
    }
  });

  const handleAudioPlay = useEffectEvent(() => {
    onPlayStateChange(true);
  });

  const handleAudioPause = useEffectEvent(() => {
    onPlayStateChange(false);
  });

  const handleAudioTimeUpdate = useEffectEvent((audio: HTMLAudioElement) => {
    setCurrentTime(audio.currentTime);
    syncMediaSessionPosition(audio);
  });

  const handleAudioDurationChange = useEffectEvent((audio: HTMLAudioElement) => {
    setDuration(audio.duration || 0);
    syncMediaSessionPosition(audio);
  });

  const handleAudioLoadStart = useEffectEvent(() => {
    setIsLoading(true);
  });

  const handleAudioCanPlay = useEffectEvent(() => {
    setIsLoading(false);
  });

  const handleAudioError = useEffectEvent(() => {
    setIsLoading(false);
  });

  const handleAudioEnded = useEffectEvent(() => {
    if (hasNext) {
      onTrackChange(tracks[currentIndex + 1]);
    } else {
      onPlayStateChange(false);
      setCurrentTime(0);
    }
  });

  useEffect(() => {
    const audio = audioRef.current;
    if (!track || !audio) return;

    const onPlay = () => handleAudioPlay();
    const onPause = () => handleAudioPause();
    const onTimeUpdate = () => handleAudioTimeUpdate(audio);
    const onDurationChange = () => handleAudioDurationChange(audio);
    const onLoadStart = () => handleAudioLoadStart();
    const onCanPlay = () => handleAudioCanPlay();
    const onError = () => handleAudioError();
    const onEnded = () => handleAudioEnded();

    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    audio.addEventListener("timeupdate", onTimeUpdate);
    audio.addEventListener("durationchange", onDurationChange);
    audio.addEventListener("loadstart", onLoadStart);
    audio.addEventListener("canplay", onCanPlay);
    audio.addEventListener("error", onError);
    audio.addEventListener("ended", onEnded);

    return () => {
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
      audio.removeEventListener("timeupdate", onTimeUpdate);
      audio.removeEventListener("durationchange", onDurationChange);
      audio.removeEventListener("loadstart", onLoadStart);
      audio.removeEventListener("canplay", onCanPlay);
      audio.removeEventListener("error", onError);
      audio.removeEventListener("ended", onEnded);
    };
  }, [audioRef, track]);

  const handleGlobalKeyDown = useEffectEvent((event: KeyboardEvent) => {
    const audio = audioRef.current;

    if (!track || !audio || isKeyboardSuspended) {
      return;
    }

    const target = event.target instanceof HTMLElement ? event.target : null;

    if (
      target &&
      (target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable)
    ) {
      return;
    }

    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }

    if (target) {
      if (target.closest(FOREIGN_OVERLAY_SELECTOR)) {
        return;
      }

      const interactive = target.closest(INTERACTIVE_SELECTOR);
      if (interactive) {
        // Space must activate the focused control everywhere, and arrow keys
        // belong to widgets like tabs and radios — except inside the player's
        // own chrome, where arrows keep seeking and adjusting volume.
        if (event.key === " ") {
          return;
        }

        if (
          ARROW_KEYS.has(event.key) &&
          !interactive.closest("[data-audio-player]")
        ) {
          return;
        }
      }
    }

    switch (event.key) {
      case " ":
        event.preventDefault();
        if (audio.paused) {
          void playAudio();
        } else {
          audio.pause();
        }
        break;
      case "ArrowLeft":
        event.preventDefault();
        audio.currentTime = Math.max(0, audio.currentTime - 10);
        break;
      case "ArrowRight": {
        event.preventDefault();
        const totalDuration = audio.duration || duration;
        audio.currentTime = Math.min(totalDuration, audio.currentTime + 10);
        break;
      }
      case "ArrowUp":
        event.preventDefault();
        audio.volume = Math.min(1, audio.volume + 0.1);
        break;
      case "ArrowDown":
        event.preventDefault();
        audio.volume = Math.max(0, audio.volume - 0.1);
        break;
      case "n":
      case "N":
      case "MediaTrackNext":
        event.preventDefault();
        if (hasNext) {
          onTrackChange(tracks[currentIndex + 1]);
        }
        break;
      case "p":
      case "P":
      case "MediaTrackPrevious":
        event.preventDefault();
        playPrevious();
        break;
      case "r":
      case "R":
      case "Home":
        event.preventDefault();
        audio.currentTime = 0;
        break;
      case "m":
      case "M":
        event.preventDefault();
        audio.muted = !audio.muted;
        break;
    }
  });

  useEffect(() => {
    window.addEventListener("keydown", handleGlobalKeyDown);

    return () => window.removeEventListener("keydown", handleGlobalKeyDown);
  }, []);

  const handleTogglePlay = () => {
    const audio = audioRef.current;
    if (!audio) return;

    if (audio.paused) {
      void playAudio();
    } else {
      audio.pause();
    }
  };

  const playNext = () => {
    if (hasNext) {
      onTrackChange(tracks[currentIndex + 1]);
    }
  };

  const handleSeek = (newTime: number) => {
    if (!audioRef.current) return;

    audioRef.current.currentTime = newTime;
    setCurrentTime(newTime);
  };

  useEffect(() => {
    if (!audioRef.current || !streamUrl) return;

    const audio = audioRef.current;

    const loadTrack = async () => {
      audio.load();
      try {
        await audio.play();
      } catch {
        // Autoplay can still be blocked by the browser in some cases.
      }
    };

    void loadTrack();
  }, [audioRef, streamUrl]);

  if (!track) return null;

  return (
    <>
      <audio ref={audioRef} preload="metadata" className="hidden">
        {streamUrl && (
          <source src={streamUrl} type={track.mime_type || undefined} />
        )}
      </audio>

      {/* Rendered outside the dialog so track changes and play/pause are
          still announced while the player is minimized. */}
      <div className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </div>

      <Dialog
        open={isExpanded}
        onOpenChange={open => {
          if (!open) {
            handleMinimize();
          }
        }}
      >
        {isExpanded && (
          <DialogFullscreenContent
            data-audio-player=""
            className={cn(
              MOTION_MEDIA_OVERLAY_ENTER_CLASS,
              "flex flex-col bg-linear-to-b from-background via-muted to-background",
            )}
            onOpenAutoFocus={event => {
              event.preventDefault();
              playPauseButtonRef.current?.focus({ preventScroll: true });
            }}
            onCloseAutoFocus={event => {
              event.preventDefault();
            }}
          >
            <DialogTitle className="sr-only">
              Now playing: {track.title} by {artist}
            </DialogTitle>
            <DialogDescription className="sr-only">
              Press Escape to minimize.
            </DialogDescription>

            <header className="flex items-center justify-between px-6 py-4">
              <button
                type="button"
                onClick={handleMinimize}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent/50 hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
                aria-label="Minimize player (Escape)"
              >
                <ChevronDown className="size-5" aria-hidden="true" />
              </button>
              <div className="text-center" id="player-header">
                <p className="text-xs tracking-widest text-muted-foreground uppercase">
                  Now Playing
                </p>
                <p className="mt-0.5 text-sm text-muted-foreground">{albumTitle}</p>
              </div>
              {onClose ? (
                <button
                  type="button"
                  onClick={onClose}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent/50 hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                  )}
                  aria-label="Stop playback and close player"
                >
                  <X className="size-5" aria-hidden="true" />
                </button>
              ) : (
                <div className="size-10" aria-hidden="true" />
              )}
            </header>

            <main className="flex flex-1 flex-col items-center justify-center px-8 pb-8">
              <div className="mb-8 size-72 overflow-hidden rounded-2xl shadow-2xl shadow-black/50 sm:size-80 md:size-96">
                {albumCover ? (
                  <img
                    src={albumCover}
                    alt={`Album cover for ${albumTitle}`}
                    className="size-full object-cover"
                  />
                ) : (
                  <div
                    className="flex size-full items-center justify-center bg-muted"
                    role="img"
                    aria-label="No album cover available"
                  >
                    <Disc3 className="size-24 text-muted-foreground" aria-hidden="true" />
                  </div>
                )}
              </div>

              <div className="mb-8 max-w-md text-center">
                <h1
                  id="track-title"
                  className="truncate text-2xl font-bold text-foreground sm:text-3xl"
                >
                  {track.title}
                </h1>
                <p className="mt-1 truncate text-lg text-primary">{artist}</p>
              </div>

              <ProgressBar
                currentTime={currentTime}
                duration={duration}
                onSeek={handleSeek}
                variant="expanded"
                resetKey={track.id}
              />

              <div
                className="flex items-center gap-6"
                role="group"
                aria-label="Playback controls"
              >
                <button
                  type="button"
                  onClick={playPrevious}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-14 items-center justify-center rounded-full text-muted-foreground hover:bg-accent/50 hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                  )}
                  aria-label={prevAriaLabel}
                >
                  <SkipBack className="size-6" aria-hidden="true" />
                </button>

                <button
                  type="button"
                  ref={playPauseButtonRef}
                  onClick={handleTogglePlay}
                  disabled={isLoading}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-20 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-xl shadow-primary/30 hover:bg-primary/90 focus:ring-4 focus:ring-ring/50 focus:outline-none disabled:opacity-50",
                  )}
                  aria-label={isLoading ? "Loading" : playPauseAriaLabel}
                >
                  {isLoading ? (
                    <Spinner className="size-8" />
                  ) : isPlaying ? (
                    <Pause className="size-8 fill-current" aria-hidden="true" />
                  ) : (
                    <Play className="size-8 fill-current" aria-hidden="true" />
                  )}
                </button>

                <button
                  type="button"
                  onClick={playNext}
                  aria-disabled={!hasNext}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-14 items-center justify-center rounded-full text-muted-foreground hover:bg-accent/50 hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none aria-disabled:cursor-not-allowed aria-disabled:opacity-30",
                  )}
                  aria-label={nextAriaLabel}
                >
                  <SkipForward className="size-6" aria-hidden="true" />
                </button>
              </div>

              <div className="mt-6">
                <VolumeControl
                  mediaRef={audioRef}
                  variant="expanded"
                />
              </div>

              <p className="mt-4 text-sm text-muted-foreground">
                Track {currentIndex + 1} of {tracks.length}
              </p>
            </main>
          </DialogFullscreenContent>
        )}
      </Dialog>

      {!isExpanded && (
        <div
          role="region"
          aria-label="Audio player"
          data-audio-player=""
          className={cn(
            MOTION_PLAYER_CHROME_ENTER_CLASS,
            "fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/95 shadow-2xl shadow-black/50 backdrop-blur-lg",
          )}
        >
          <div className="mx-auto max-w-7xl px-4 py-3">
            <div className="flex items-center gap-4">
              <button
                ref={expandButtonRef}
                type="button"
                onClick={onExpand}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left hover:opacity-80 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none",
                )}
                aria-label={`Expand player. Now playing: ${track.title} by ${artist}`}
              >
                <div
                  className="size-12 shrink-0 overflow-hidden rounded-lg bg-muted shadow-lg"
                  aria-hidden="true"
                >
                  {albumCover ? (
                    <img
                      src={albumCover}
                      alt=""
                      className="size-full object-cover"
                    />
                  ) : (
                    <div className="flex size-full items-center justify-center">
                      <Disc3 className="size-5 text-muted-foreground" aria-hidden="true" />
                    </div>
                  )}
                </div>

                <div className="min-w-0 flex-1" aria-hidden="true">
                  <p className="truncate text-sm font-medium text-foreground">
                    {track.title}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">{artist}</p>
                </div>
              </button>

              <div
                className="flex items-center gap-2"
                role="group"
                aria-label="Playback controls"
              >
                <button
                  type="button"
                  onClick={playPrevious}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-none",
                  )}
                  aria-label={prevAriaLabel}
                >
                  <SkipBack className="size-4" aria-hidden="true" />
                </button>

                <button
                  type="button"
                  onClick={handleTogglePlay}
                  disabled={isLoading}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-12 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none disabled:opacity-50",
                  )}
                  aria-label={isLoading ? "Loading" : playPauseAriaLabel}
                >
                  {isLoading ? (
                    <Spinner className="size-5" />
                  ) : isPlaying ? (
                    <Pause className="size-5 fill-current" aria-hidden="true" />
                  ) : (
                    <Play className="size-5 fill-current" aria-hidden="true" />
                  )}
                </button>

                <button
                  type="button"
                  onClick={playNext}
                  aria-disabled={!hasNext}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-none aria-disabled:cursor-not-allowed aria-disabled:opacity-30",
                  )}
                  aria-label={nextAriaLabel}
                >
                  <SkipForward className="size-4" aria-hidden="true" />
                </button>
              </div>

              <ProgressBar
                currentTime={currentTime}
                duration={duration}
                onSeek={handleSeek}
                variant="minimized"
                resetKey={track.id}
              />

              <div className="hidden sm:block">
                <VolumeControl
                  mediaRef={audioRef}
                  variant="minimized"
                />
              </div>

              <button
                type="button"
                onClick={onExpand}
                className={cn(
                  MOTION_PLAYER_CHROME_BUTTON_CLASS,
                  "hidden size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-none sm:flex",
                )}
                aria-label="Expand to fullscreen player"
              >
                <ChevronUp className="size-4" aria-hidden="true" />
              </button>

              {onClose && (
                <button
                  type="button"
                  onClick={onClose}
                  className={cn(
                    MOTION_PLAYER_CHROME_BUTTON_CLASS,
                    "flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-none",
                  )}
                  aria-label="Stop playback and close player"
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              )}
            </div>

            <ProgressBar
              currentTime={currentTime}
              duration={duration}
              onSeek={handleSeek}
              variant="mobile"
              resetKey={track.id}
            />
          </div>
        </div>
      )}
    </>
  );
}
