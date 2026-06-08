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
import type { TrackType } from "@/types";
import ProgressBar from "@/components/ProgressBar";
import VolumeControl from "@/components/VolumeControl";

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
  const expandedContainerRef = useRef<HTMLDivElement>(null);

  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isLoading, setIsLoading] = useState(false);

  const artist = musicianName || albumTitle;
  const announcement = track
    ? `${isPlaying ? "Now playing" : "Paused"}: ${track.title} by ${artist}`
    : "";

  const currentIndex = track ? tracks.findIndex(t => t.id === track.id) : -1;
  const hasPrevious = currentIndex > 0;
  const hasNext = currentIndex < tracks.length - 1 && currentIndex !== -1;
  const prevAriaLabel = hasPrevious ? "Previous track" : "No previous track";
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

  useEffect(() => {
    if (!isExpanded || !playPauseButtonRef.current) {
      return;
    }

    const timer = setTimeout(() => {
      playPauseButtonRef.current?.focus();
    }, 50);

    return () => clearTimeout(timer);
  }, [isExpanded]);

  useEffect(() => {
    if (!isExpanded) return;

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onMinimize();
      }
    };

    window.addEventListener("keydown", handleEscape);

    return () => window.removeEventListener("keydown", handleEscape);
  }, [isExpanded, onMinimize]);

  const handleExpandedKeyDown = (event: React.KeyboardEvent) => {
    if (!isExpanded || !expandedContainerRef.current) return;

    if (event.key !== "Tab") {
      return;
    }

    const focusableElements =
      expandedContainerRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );

    const firstElement = focusableElements[0];
    const lastElement = focusableElements[focusableElements.length - 1];

    if (event.shiftKey && document.activeElement === firstElement) {
      event.preventDefault();
      lastElement?.focus();
    } else if (!event.shiftKey && document.activeElement === lastElement) {
      event.preventDefault();
      firstElement?.focus();
    }
  };

  useEffect(() => {
    if (!track || !("mediaSession" in navigator)) return;

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
    if (hasPrevious) {
      onTrackChange(tracks[currentIndex - 1]);
    }
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
    if (!("mediaSession" in navigator) || audio.duration <= 0) {
      return;
    }

    navigator.mediaSession.setPositionState({
      duration: audio.duration,
      playbackRate: audio.playbackRate,
      position: audio.currentTime,
    });
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

    const target = event.target as HTMLElement;

    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.isContentEditable
    ) {
      return;
    }

    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
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
        if (hasPrevious) {
          onTrackChange(tracks[currentIndex - 1]);
        }
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

  const playPrevious = () => {
    if (hasPrevious) {
      onTrackChange(tracks[currentIndex - 1]);
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
        {streamUrl && <source src={streamUrl} type={track.mime_type} />}
      </audio>

      {isExpanded && (
        <div
          ref={expandedContainerRef}
          role="dialog"
          aria-modal="true"
          aria-label={`Now playing: ${track.title} by ${artist}`}
          onKeyDown={handleExpandedKeyDown}
          className="fixed inset-0 z-50 flex animate-in flex-col bg-linear-to-b from-slate-900 via-slate-800
            to-slate-900 duration-200 zoom-in-95 fade-in slide-in-from-bottom-2 motion-reduce:animate-none"
        >
          <div className="sr-only" aria-live="polite" aria-atomic="true">
            {announcement}
          </div>

          <header className="flex items-center justify-between px-6 py-4">
            <button
              onClick={onMinimize}
              className="flex size-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
              aria-label="Minimize player (Escape)"
            >
              <ChevronDown className="size-5" aria-hidden="true" />
            </button>
            <div className="text-center" id="player-header">
              <p className="text-xs tracking-widest text-slate-400 uppercase">
                Now Playing
              </p>
              <p className="mt-0.5 text-sm text-slate-400">{albumTitle}</p>
            </div>
            {onClose ? (
              <button
                onClick={onClose}
                className="flex size-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
                aria-label="Stop playback and close player"
              >
                <X className="size-5" aria-hidden="true" />
              </button>
            ) : (
              <div className="size-10" aria-hidden="true" />
            )}
          </header>

          <main className="flex flex-1 flex-col items-center justify-center px-8 pb-8">
            <figure className="mb-8 size-72 overflow-hidden rounded-2xl shadow-2xl shadow-black/50 sm:size-80 md:size-96">
              {albumCover ? (
                <img
                  src={albumCover}
                  alt={`Album cover for ${albumTitle}`}
                  className="size-full object-cover"
                />
              ) : (
                <div
                  className="flex size-full items-center justify-center bg-slate-800"
                  role="img"
                  aria-label="No album cover available"
                >
                  <Disc3 className="size-24 text-slate-600" aria-hidden="true" />
                </div>
              )}
            </figure>

            <div className="mb-8 max-w-md text-center">
              <h1
                id="track-title"
                className="truncate text-2xl font-bold text-white sm:text-3xl"
              >
                {track.title}
              </h1>
              <p className="mt-1 truncate text-lg text-amber-400">{artist}</p>
            </div>

            <ProgressBar
              currentTime={currentTime}
              duration={duration}
              onSeek={handleSeek}
              variant="expanded"
            />

            <div
              className="flex items-center gap-6"
              role="group"
              aria-label="Playback controls"
            >
              <button
                onClick={playPrevious}
                disabled={!hasPrevious}
                className="flex size-14 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30"
                aria-label={prevAriaLabel}
              >
                <SkipBack className="size-6" aria-hidden="true" />
              </button>

              <button
                ref={playPauseButtonRef}
                onClick={handleTogglePlay}
                disabled={isLoading}
                className="flex size-20 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-xl shadow-amber-500/30 transition-colors hover:bg-amber-400 focus:ring-4 focus:ring-amber-400/50 focus:outline-none disabled:opacity-50"
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
                onClick={playNext}
                disabled={!hasNext}
                className="flex size-14 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30"
                aria-label={nextAriaLabel}
              >
                <SkipForward className="size-6" aria-hidden="true" />
              </button>
            </div>

            <div className="mt-6">
              <VolumeControl
                mediaRef={audioRef}
                variant="expanded"
                accent="amber"
              />
            </div>

            <p className="mt-4 text-sm text-slate-400">
              Track {currentIndex + 1} of {tracks.length}
            </p>
          </main>
        </div>
      )}

      {!isExpanded && (
        <div
          role="region"
          aria-label="Audio player"
          className="fixed inset-x-0 bottom-0 z-40 animate-in border-t border-slate-700/50 bg-slate-900/95 shadow-2xl shadow-black/50
            backdrop-blur-lg duration-200 fade-in slide-in-from-bottom motion-reduce:animate-none"
        >
          <div className="mx-auto max-w-7xl px-4 py-3">
            <div className="flex items-center gap-4">
              <button
                onClick={onExpand}
                className="flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left transition-opacity duration-150 hover:opacity-80 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none motion-reduce:transition-none"
                aria-label={`Expand player. Now playing: ${track.title} by ${artist}`}
              >
                <div className="size-12 shrink-0 overflow-hidden rounded-lg bg-slate-800 shadow-lg">
                  {albumCover ? (
                    <img
                      src={albumCover}
                      alt={albumTitle}
                      className="size-full object-cover"
                    />
                  ) : (
                    <div className="flex size-full items-center justify-center">
                      <Disc3 className="size-5 text-slate-600" aria-hidden="true" />
                    </div>
                  )}
                </div>

                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-white">
                    {track.title}
                  </p>
                  <p className="truncate text-xs text-slate-400">{artist}</p>
                </div>
              </button>

              <div
                className="flex items-center gap-2"
                role="group"
                aria-label="Playback controls"
              >
                <button
                  onClick={playPrevious}
                  disabled={!hasPrevious}
                  className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30"
                  aria-label={prevAriaLabel}
                >
                  <SkipBack className="size-4" aria-hidden="true" />
                </button>

                <button
                  onClick={handleTogglePlay}
                  disabled={isLoading}
                  className="flex size-12 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50"
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
                  onClick={playNext}
                  disabled={!hasNext}
                  className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30"
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
              />

              <div className="hidden sm:block">
                <VolumeControl
                  mediaRef={audioRef}
                  variant="minimized"
                  accent="amber"
                />
              </div>

              <button
                onClick={onExpand}
                className="hidden size-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none sm:flex"
                aria-label="Expand to fullscreen player"
              >
                <ChevronUp className="size-4" aria-hidden="true" />
              </button>

              {onClose && (
                <button
                  onClick={onClose}
                  className="flex size-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none"
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
            />
          </div>
        </div>
      )}
    </>
  );
}
