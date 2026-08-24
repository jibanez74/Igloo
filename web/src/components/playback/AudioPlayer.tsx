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
import PlayerLikeButton from "@/components/playback/PlayerLikeButton";
import ProgressBar from "@/components/playback/ProgressBar";
import VolumeControl from "@/components/playback/VolumeControl";
import {
  AUDIO_SEEK_STEP_SECONDS,
  AUDIO_VOLUME_STEP,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
  PLAYER_TRANSPORT_INERT_CLASS,
} from "@/lib/constants";
import { playMediaElement, toggleMediaPlayback } from "@/lib/audio-utils";
import { cn } from "@/lib/utils";

type AudioPlayerProps = {
  track: TrackType | null;
  tracks: TrackType[];
  // Played tracks trimmed from the front of an endless queue; added back into
  // the "Track N of M" counter so it never jumps backwards. Finite queues
  // never trim, so the default keeps the counter untouched.
  trimmedCount?: number;
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
const PREVIOUS_TRACK_ARIA_LABEL = "Previous track";

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
  trimmedCount = 0,
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
    if (currentTrackId !== lastState.trackId) {
      // The <audio> element persists across track changes, so the old track's
      // position/duration would otherwise show until the new track's
      // timeupdate/durationchange events fire.
      setCurrentTime(0);
      setDuration(0);
    }
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
  const nextAriaLabel = hasNext ? "Next track" : "No next track";
  // Mirrors playPrevious(): past the restart threshold (or with no previous
  // track) the button restarts the current track instead of navigating.
  const previousAriaLabel =
    hasPrevious && currentTime <= RESTART_THRESHOLD_SECONDS
      ? PREVIOUS_TRACK_ARIA_LABEL
      : "Restart track";
  const playPauseAriaLabel = isPlaying ? "Pause" : "Play";
  const streamUrl = track ? `/api/music/tracks/${track.id}/stream` : null;

  // Seek relative to the current position, clamped to the track bounds.
  const seekBy = (offsetSeconds: number) => {
    const audio = audioRef.current;
    if (!audio) return;

    const totalDuration = audio.duration || duration;
    audio.currentTime = Math.min(
      totalDuration,
      Math.max(0, audio.currentTime + offsetSeconds),
    );
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
    if (!("mediaSession" in navigator)) {
      return;
    }

    // Clear stale lock-screen/OS media info once playback stops; otherwise
    // the last track keeps showing after the player is closed.
    if (!track || typeof MediaMetadata === "undefined") {
      navigator.mediaSession.metadata = null;
      return;
    }

    // MediaMetadata artwork wants absolute URLs; new URL() leaves already
    // absolute covers untouched and resolves /api paths against the origin.
    const artworkUrl = albumCover
      ? new URL(albumCover, window.location.origin).toString()
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
    void playMediaElement(audioRef.current);
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
      seekBy(-(seekOffset ?? AUDIO_SEEK_STEP_SECONDS));
    },
  );

  const handleMediaSessionSeekForward = useEffectEvent(
    ({ seekOffset }: { seekOffset?: number }) => {
      seekBy(seekOffset ?? AUDIO_SEEK_STEP_SECONDS);
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

  const handleTogglePlay = () => {
    // While loading the button is aria-disabled (never `disabled`, which
    // would drop it from the VoiceOver focus order) — guard here instead.
    if (isLoading) return;
    toggleMediaPlayback(audioRef.current);
  };

  const handleGlobalKeyDown = useEffectEvent((event: KeyboardEvent) => {
    const audio = audioRef.current;

    if (!track || !audio || isKeyboardSuspended) {
      return;
    }

    const target = event.target instanceof HTMLElement ? event.target : null;

    // The player's own sliders (seek, volume) stay eligible for the global
    // shortcuts — they never take text input — but their range navigation keys
    // belong to the native slider. Anything else that takes typing suppresses
    // shortcuts.
    const isPlayerRangeInput =
      target instanceof HTMLInputElement &&
      target.type === "range" &&
      target.closest("[data-audio-player]") !== null;

    if (
      target &&
      !isPlayerRangeInput &&
      (target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable)
    ) {
      return;
    }

    if (
      isPlayerRangeInput &&
      (ARROW_KEYS.has(event.key) || event.key === "Home")
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
        // Space must activate the focused control everywhere, and navigation
        // keys belong to widgets like tabs and radios — except inside the
        // player's own chrome, where they keep controlling playback.
        if (event.key === " ") {
          return;
        }

        if (
          (ARROW_KEYS.has(event.key) || event.key === "Home") &&
          !interactive.closest("[data-audio-player]")
        ) {
          return;
        }
      }
    }

    switch (event.key) {
      case " ":
        event.preventDefault();
        handleTogglePlay();
        break;
      case "ArrowLeft":
        event.preventDefault();
        seekBy(-AUDIO_SEEK_STEP_SECONDS);
        break;
      case "ArrowRight":
        event.preventDefault();
        seekBy(AUDIO_SEEK_STEP_SECONDS);
        break;
      case "ArrowUp":
        event.preventDefault();
        audio.volume = Math.min(1, audio.volume + AUDIO_VOLUME_STEP);
        break;
      case "ArrowDown":
        event.preventDefault();
        audio.volume = Math.max(0, audio.volume - AUDIO_VOLUME_STEP);
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
    audio.load();
    void playMediaElement(audio);
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
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-accent/50",
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
                    PLAYER_ICON_BUTTON_CLASS,
                    "size-10 hover:bg-accent/50",
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

              <div className="mb-8 flex max-w-md items-center gap-3">
                <div className="size-10 shrink-0" aria-hidden="true" />
                <div className="min-w-0 text-center">
                  <h1
                    id="track-title"
                    className="truncate text-2xl font-bold text-foreground sm:text-3xl"
                  >
                    {track.title}
                  </h1>
                  <p className="mt-1 truncate text-lg text-primary">{artist}</p>
                </div>
                <PlayerLikeButton
                  trackId={track.id}
                  trackTitle={track.title}
                  variant="expanded"
                />
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
                    PLAYER_ICON_BUTTON_CLASS,
                    "size-14 hover:bg-accent/50",
                  )}
                  aria-label={previousAriaLabel}
                >
                  <SkipBack className="size-6" aria-hidden="true" />
                </button>

                <button
                  type="button"
                  ref={playPauseButtonRef}
                  onClick={handleTogglePlay}
                  aria-disabled={isLoading || undefined}
                  aria-busy={isLoading || undefined}
                  className={cn(
                    PLAYER_PRIMARY_BUTTON_CLASS,
                    "size-20 shadow-xl shadow-primary/30",
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
                    PLAYER_ICON_BUTTON_CLASS,
                    PLAYER_TRANSPORT_INERT_CLASS,
                    "size-14 hover:bg-accent/50",
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
                Track {trimmedCount + currentIndex + 1} of{" "}
                {trimmedCount + tracks.length}
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
                  FOCUS_VISIBLE_RING_CLASS,
                  "flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left hover:opacity-80",
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

              <PlayerLikeButton
                trackId={track.id}
                trackTitle={track.title}
                variant="minimized"
              />

              <div
                className="flex items-center gap-2"
                role="group"
                aria-label="Playback controls"
              >
                <button
                  type="button"
                  onClick={playPrevious}
                  className={cn(PLAYER_ICON_BUTTON_CLASS, "size-10 hover:bg-accent")}
                  aria-label={previousAriaLabel}
                >
                  <SkipBack className="size-4" aria-hidden="true" />
                </button>

                <button
                  type="button"
                  onClick={handleTogglePlay}
                  aria-disabled={isLoading || undefined}
                  aria-busy={isLoading || undefined}
                  className={cn(
                    PLAYER_PRIMARY_BUTTON_CLASS,
                    "size-12 shadow-lg shadow-primary/20",
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
                    PLAYER_ICON_BUTTON_CLASS,
                    PLAYER_TRANSPORT_INERT_CLASS,
                    "size-10 hover:bg-accent",
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
                  PLAYER_ICON_BUTTON_CLASS,
                  "hidden size-8 hover:bg-accent sm:flex",
                )}
                aria-label="Expand to fullscreen player"
              >
                <ChevronUp className="size-4" aria-hidden="true" />
              </button>

              {onClose && (
                <button
                  type="button"
                  onClick={onClose}
                  className={cn(PLAYER_ICON_BUTTON_CLASS, "size-8 hover:bg-accent")}
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
