import { useRef, useState, useEffect } from "react";
import type { TrackType } from "@/types";

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
}: AudioPlayerProps) {
  const playPauseButtonRef = useRef<HTMLButtonElement>(null);
  const expandedContainerRef = useRef<HTMLDivElement>(null);

  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isLoading, setIsLoading] = useState(false);

  // Screen reader announcement - React Compiler will optimize this automatically
  const artist = musicianName || albumTitle;
  const announcement = track
    ? `${isPlaying ? "Now playing" : "Paused"}: ${track.title} by ${artist}`
    : "";

  // Focus the play/pause button when fullscreen opens
  useEffect(() => {
    if (isExpanded && playPauseButtonRef.current) {
      // Small delay to ensure the DOM is ready
      const timer = setTimeout(() => {
        playPauseButtonRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    }
  }, [isExpanded]);

  // Handle Escape key to minimize when expanded
  useEffect(() => {
    if (!isExpanded) return;

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onMinimize();
      }
    };

    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [isExpanded, onMinimize]);

  // Trap focus within the expanded player - React Compiler handles memoization
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isExpanded || !expandedContainerRef.current) return;

    if (e.key === "Tab") {
      const focusableElements =
        expandedContainerRef.current.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
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

  // Get current track index
  const currentIndex = track ? tracks.findIndex(t => t.id === track.id) : -1;
  const hasPrevious = currentIndex > 0;
  const hasNext = currentIndex < tracks.length - 1 && currentIndex !== -1;

  // Stream URL for current track
  const streamUrl = track ? `/api/music/tracks/${track.id}/stream` : null;

  // Handle play/pause
  const togglePlay = () => {
    if (!audioRef.current) return;

    if (isPlaying) {
      audioRef.current.pause();
    } else {
      audioRef.current.play();
    }
  };

  // Handle previous track
  const playPrevious = () => {
    if (hasPrevious) {
      onTrackChange(tracks[currentIndex - 1]);
    }
  };

  // Handle next track
  const playNext = () => {
    if (hasNext) {
      onTrackChange(tracks[currentIndex + 1]);
    }
  };

  // Handle seeking via progress bar click - uses the event target directly
  const handleProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!audioRef.current || !duration) return;

    const target = e.currentTarget;
    const rect = target.getBoundingClientRect();
    const clickX = e.clientX - rect.left;
    const percentage = clickX / rect.width;
    const newTime = percentage * duration;

    audioRef.current.currentTime = newTime;
    setCurrentTime(newTime);
  };

  // Update audio element when track changes
  useEffect(() => {
    if (audioRef.current && streamUrl) {
      audioRef.current.load();
      audioRef.current.play().catch(() => {
        // Autoplay might be blocked, that's okay
      });
    }
  }, [audioRef, streamUrl]);

  // Audio event handlers
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const handlePlay = () => onPlayStateChange(true);
    const handlePause = () => onPlayStateChange(false);
    const handleTimeUpdate = () => setCurrentTime(audio.currentTime);
    const handleDurationChange = () => setDuration(audio.duration || 0);
    const handleLoadStart = () => setIsLoading(true);
    const handleCanPlay = () => setIsLoading(false);
    const handleEnded = () => {
      // Play next track if available
      if (hasNext) {
        onTrackChange(tracks[currentIndex + 1]);
      } else {
        onPlayStateChange(false);
        setCurrentTime(0);
      }
    };

    audio.addEventListener("play", handlePlay);
    audio.addEventListener("pause", handlePause);
    audio.addEventListener("timeupdate", handleTimeUpdate);
    audio.addEventListener("durationchange", handleDurationChange);
    audio.addEventListener("loadstart", handleLoadStart);
    audio.addEventListener("canplay", handleCanPlay);
    audio.addEventListener("ended", handleEnded);

    return () => {
      audio.removeEventListener("play", handlePlay);
      audio.removeEventListener("pause", handlePause);
      audio.removeEventListener("timeupdate", handleTimeUpdate);
      audio.removeEventListener("durationchange", handleDurationChange);
      audio.removeEventListener("loadstart", handleLoadStart);
      audio.removeEventListener("canplay", handleCanPlay);
      audio.removeEventListener("ended", handleEnded);
    };
  }, [
    audioRef,
    hasNext,
    currentIndex,
    tracks,
    onTrackChange,
    onPlayStateChange,
  ]);

  // Keyboard controls
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!track || !audioRef.current) return;

      // Skip keyboard shortcuts when user is typing in an input field
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      // Skip shortcuts when modifier keys are pressed (allow browser shortcuts like Cmd+R)
      if (e.ctrlKey || e.metaKey || e.altKey) {
        return;
      }

      switch (e.key) {
        case " ":
          e.preventDefault();
          if (isPlaying) {
            audioRef.current.pause();
          } else {
            audioRef.current.play();
          }
          break;
        case "ArrowLeft":
          audioRef.current.currentTime = Math.max(0, currentTime - 10);
          break;
        case "ArrowRight":
          audioRef.current.currentTime = Math.min(duration, currentTime + 10);
          break;
        case "ArrowUp":
          e.preventDefault();
          audioRef.current.volume = Math.min(1, audioRef.current.volume + 0.1);
          break;
        case "ArrowDown":
          e.preventDefault();
          audioRef.current.volume = Math.max(0, audioRef.current.volume - 0.1);
          break;
        // Track navigation shortcuts for accessibility
        case "n":
        case "N":
        case "MediaTrackNext":
          e.preventDefault();
          if (hasNext) {
            onTrackChange(tracks[currentIndex + 1]);
          }
          break;
        case "p":
        case "P":
        case "MediaTrackPrevious":
          e.preventDefault();
          if (hasPrevious) {
            onTrackChange(tracks[currentIndex - 1]);
          }
          break;
        case "r":
        case "R":
        case "Home":
          e.preventDefault();
          audioRef.current.currentTime = 0;
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    audioRef,
    track,
    isPlaying,
    currentTime,
    duration,
    hasNext,
    hasPrevious,
    currentIndex,
    tracks,
    onTrackChange,
  ]);

  // Don't render if no track
  if (!track) return null;

  const coverUrl = albumCover;
  const progress = duration > 0 ? (currentTime / duration) * 100 : 0;

  return (
    <>
      {/* Single audio element - shared between expanded and minimized modes */}
      <audio ref={audioRef} preload='metadata' className='hidden'>
        {streamUrl && <source src={streamUrl} type={track.mime_type} />}
      </audio>

      {/* Expanded (fullscreen) mode */}
      {isExpanded && (
        <div
          ref={expandedContainerRef}
          role='dialog'
          aria-modal='true'
          aria-label={`Now playing: ${track.title} by ${artist}`}
          onKeyDown={handleKeyDown}
          className='fixed inset-0 z-50 bg-linear-to-b from-slate-900 via-slate-800 to-slate-900 flex flex-col
            animate-expand-in'
        >
          {/* Screen reader live region for playback status - only announces on meaningful changes */}
          <div className='sr-only' aria-live='polite' aria-atomic='true'>
            {announcement}
          </div>

          {/* Header with minimize button */}
          <header className='flex items-center justify-between px-6 py-4'>
            <button
              onClick={onMinimize}
              className='w-10 h-10 flex items-center justify-center rounded-full text-slate-400 hover:text-white hover:bg-slate-800/50 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
              aria-label='Minimize player (Escape)'
            >
              <i
                className='fa-solid fa-chevron-down text-lg'
                aria-hidden='true'
              />
            </button>
            <div className='text-center' id='player-header'>
              <p className='text-xs text-slate-500 uppercase tracking-widest'>
                Now Playing
              </p>
              <p className='text-sm text-slate-400 mt-0.5'>{albumTitle}</p>
            </div>
            {onClose ? (
              <button
                onClick={onClose}
                className='w-10 h-10 flex items-center justify-center rounded-full text-slate-400 hover:text-white hover:bg-slate-800/50 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
                aria-label='Stop playback and close player'
              >
                <i className='fa-solid fa-xmark text-lg' aria-hidden='true' />
              </button>
            ) : (
              <div className='w-10 h-10' aria-hidden='true' />
            )}
          </header>

          {/* Main content */}
          <main className='flex-1 flex flex-col items-center justify-center px-8 pb-8'>
            {/* Album cover */}
            <figure className='w-72 h-72 sm:w-80 sm:h-80 md:w-96 md:h-96 rounded-2xl overflow-hidden shadow-2xl shadow-black/50 mb-8'>
              {coverUrl ? (
                <img
                  src={coverUrl}
                  alt={`Album cover for ${albumTitle}`}
                  className='w-full h-full object-cover'
                />
              ) : (
                <div
                  className='w-full h-full bg-slate-800 flex items-center justify-center'
                  role='img'
                  aria-label='No album cover available'
                >
                  <i
                    className='fa-solid fa-compact-disc text-slate-600 text-8xl'
                    aria-hidden='true'
                  />
                </div>
              )}
            </figure>

            {/* Track info */}
            <div className='text-center max-w-md mb-8'>
              <h1
                id='track-title'
                className='text-2xl sm:text-3xl font-bold text-white truncate'
              >
                {track.title}
              </h1>
              <p className='text-lg text-amber-400 mt-1 truncate'>{artist}</p>
            </div>

            {/* Progress bar */}
            <div
              className='w-full max-w-md mb-6'
              role='group'
              aria-label='Playback progress'
            >
              <div
                onClick={handleProgressClick}
                onKeyDown={e => {
                  if (!audioRef.current || !duration) return;
                  if (e.key === "ArrowLeft") {
                    e.preventDefault();
                    audioRef.current.currentTime = Math.max(0, currentTime - 5);
                  } else if (e.key === "ArrowRight") {
                    e.preventDefault();
                    audioRef.current.currentTime = Math.min(
                      duration,
                      currentTime + 5
                    );
                  }
                }}
                tabIndex={0}
                className='h-2 bg-slate-700 rounded-full cursor-pointer group relative focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
                role='slider'
                aria-label='Seek through track'
                aria-valuenow={Math.round(currentTime)}
                aria-valuemin={0}
                aria-valuemax={Math.round(duration)}
                aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
              >
                <div
                  className='absolute inset-y-0 left-0 bg-amber-400 rounded-full transition-all'
                  style={{ width: `${progress}%` }}
                />
                <div
                  className='absolute top-1/2 -translate-y-1/2 w-4 h-4 bg-white rounded-full shadow-lg opacity-0 group-hover:opacity-100 group-focus:opacity-100 transition-opacity'
                  style={{ left: `calc(${progress}% - 8px)` }}
                />
              </div>
              <div className='flex justify-between mt-2'>
                <span
                  className='text-sm text-slate-500 tabular-nums'
                  aria-hidden='true'
                >
                  {formatTime(currentTime)}
                </span>
                <span
                  className='text-sm text-slate-500 tabular-nums'
                  aria-hidden='true'
                >
                  {formatTime(duration)}
                </span>
              </div>
            </div>

            {/* Playback controls */}
            <div
              className='flex items-center gap-6'
              role='group'
              aria-label='Playback controls'
            >
              <button
                onClick={playPrevious}
                disabled={!hasPrevious}
                className='w-14 h-14 flex items-center justify-center rounded-full text-slate-300 hover:text-white hover:bg-slate-800/50 disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
                aria-label={
                  hasPrevious
                    ? `Previous track: ${tracks[currentIndex - 1]?.title}`
                    : "No previous track"
                }
              >
                <i
                  className='fa-solid fa-backward-step text-2xl'
                  aria-hidden='true'
                />
              </button>

              <button
                ref={playPauseButtonRef}
                onClick={togglePlay}
                disabled={isLoading}
                className='w-20 h-20 flex items-center justify-center rounded-full bg-amber-500 text-slate-900 hover:bg-amber-400 disabled:opacity-50 transition-colors shadow-xl shadow-amber-500/30 focus:outline-none focus:ring-4 focus:ring-amber-400/50'
                aria-label={
                  isLoading
                    ? "Loading track"
                    : isPlaying
                      ? `Pause ${track.title}`
                      : `Play ${track.title}`
                }
              >
                {isLoading ? (
                  <i
                    className='fa-solid fa-spinner fa-spin text-3xl'
                    aria-hidden='true'
                  />
                ) : isPlaying ? (
                  <i
                    className='fa-solid fa-pause text-3xl'
                    aria-hidden='true'
                  />
                ) : (
                  <i
                    className='fa-solid fa-play text-3xl ml-1'
                    aria-hidden='true'
                  />
                )}
              </button>

              <button
                onClick={playNext}
                disabled={!hasNext}
                className='w-14 h-14 flex items-center justify-center rounded-full text-slate-300 hover:text-white hover:bg-slate-800/50 disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
                aria-label={
                  hasNext
                    ? `Next track: ${tracks[currentIndex + 1]?.title}`
                    : "No next track"
                }
              >
                <i
                  className='fa-solid fa-forward-step text-2xl'
                  aria-hidden='true'
                />
              </button>
            </div>

            {/* Track number indicator */}
            <p className='text-sm text-slate-500 mt-6'>
              Track {currentIndex + 1} of {tracks.length}
            </p>
          </main>
        </div>
      )}

      {/* Minimized (bottom bar) mode - always rendered when track exists */}
      <div
        role='region'
        aria-label='Audio player'
        className='fixed bottom-0 left-0 right-0 z-40 bg-slate-900/95 backdrop-blur-lg border-t border-slate-700/50 shadow-2xl shadow-black/50
          animate-slide-up'
      >
        <div className='max-w-7xl mx-auto px-4 py-3'>
          <div className='flex items-center gap-4'>
            {/* Album cover and track info - clickable to expand */}
            <button
              onClick={onExpand}
              className='flex items-center gap-3 min-w-0 flex-1 text-left hover:opacity-80 transition-opacity focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 rounded-lg'
              aria-label={`Expand player. Now playing: ${track.title} by ${artist}`}
            >
              {/* Album cover */}
              <div className='w-12 h-12 rounded-lg overflow-hidden bg-slate-800 shrink-0 shadow-lg'>
                {coverUrl ? (
                  <img
                    src={coverUrl}
                    alt={albumTitle}
                    className='w-full h-full object-cover'
                  />
                ) : (
                  <div className='w-full h-full flex items-center justify-center'>
                    <i
                      className='fa-solid fa-compact-disc text-slate-600 text-lg'
                      aria-hidden='true'
                    />
                  </div>
                )}
              </div>

              {/* Track info */}
              <div className='min-w-0 flex-1'>
                <p className='text-white font-medium truncate text-sm'>
                  {track.title}
                </p>
                <p className='text-slate-400 text-xs truncate'>{artist}</p>
              </div>
            </button>

            {/* Playback controls */}
            <div
              className='flex items-center gap-2'
              role='group'
              aria-label='Playback controls'
            >
              {/* Previous button */}
              <button
                onClick={playPrevious}
                disabled={!hasPrevious}
                className='w-10 h-10 flex items-center justify-center rounded-full text-slate-300 hover:text-white hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400'
                aria-label={
                  hasPrevious
                    ? `Previous track: ${tracks[currentIndex - 1]?.title}`
                    : "No previous track"
                }
              >
                <i className='fa-solid fa-backward-step' aria-hidden='true' />
              </button>

              {/* Play/Pause button */}
              <button
                onClick={togglePlay}
                disabled={isLoading}
                className='w-12 h-12 flex items-center justify-center rounded-full bg-amber-500 text-slate-900 hover:bg-amber-400 disabled:opacity-50 transition-colors shadow-lg shadow-amber-500/20 focus:outline-none focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900'
                aria-label={
                  isLoading
                    ? "Loading track"
                    : isPlaying
                      ? `Pause ${track.title}`
                      : `Play ${track.title}`
                }
              >
                {isLoading ? (
                  <i
                    className='fa-solid fa-spinner fa-spin'
                    aria-hidden='true'
                  />
                ) : isPlaying ? (
                  <i className='fa-solid fa-pause' aria-hidden='true' />
                ) : (
                  <i className='fa-solid fa-play ml-0.5' aria-hidden='true' />
                )}
              </button>

              {/* Next button */}
              <button
                onClick={playNext}
                disabled={!hasNext}
                className='w-10 h-10 flex items-center justify-center rounded-full text-slate-300 hover:text-white hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400'
                aria-label={
                  hasNext
                    ? `Next track: ${tracks[currentIndex + 1]?.title}`
                    : "No next track"
                }
              >
                <i className='fa-solid fa-forward-step' aria-hidden='true' />
              </button>
            </div>

            {/* Progress bar and time */}
            <div
              className='hidden sm:flex items-center gap-3 flex-1 max-w-md'
              role='group'
              aria-label='Playback progress'
            >
              {/* Current time */}
              <span
                className='text-xs text-slate-400 w-10 text-right tabular-nums'
                aria-hidden='true'
              >
                {formatTime(currentTime)}
              </span>

              {/* Progress bar */}
              <div
                onClick={handleProgressClick}
                onKeyDown={e => {
                  if (!audioRef.current || !duration) return;
                  if (e.key === "ArrowLeft") {
                    e.preventDefault();
                    audioRef.current.currentTime = Math.max(0, currentTime - 5);
                  } else if (e.key === "ArrowRight") {
                    e.preventDefault();
                    audioRef.current.currentTime = Math.min(
                      duration,
                      currentTime + 5
                    );
                  }
                }}
                tabIndex={0}
                className='flex-1 h-1.5 bg-slate-700 rounded-full cursor-pointer group relative focus:outline-none focus:ring-2 focus:ring-amber-400'
                role='slider'
                aria-label='Seek through track'
                aria-valuenow={Math.round(currentTime)}
                aria-valuemin={0}
                aria-valuemax={Math.round(duration)}
                aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
              >
                {/* Progress fill */}
                <div
                  className='absolute inset-y-0 left-0 bg-amber-400 rounded-full transition-all'
                  style={{ width: `${progress}%` }}
                />
                {/* Hover indicator */}
                <div
                  className='absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-white rounded-full shadow-md opacity-0 group-hover:opacity-100 group-focus:opacity-100 transition-opacity'
                  style={{ left: `calc(${progress}% - 6px)` }}
                />
              </div>

              {/* Duration */}
              <span
                className='text-xs text-slate-400 w-10 tabular-nums'
                aria-hidden='true'
              >
                {formatTime(duration)}
              </span>
            </div>

            {/* Expand button */}
            <button
              onClick={onExpand}
              className='hidden sm:flex w-8 h-8 items-center justify-center rounded-full text-slate-400 hover:text-white hover:bg-slate-800 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400'
              aria-label='Expand to fullscreen player'
            >
              <i className='fa-solid fa-chevron-up' aria-hidden='true' />
            </button>

            {/* Close button */}
            {onClose && (
              <button
                onClick={onClose}
                className='w-8 h-8 flex items-center justify-center rounded-full text-slate-400 hover:text-white hover:bg-slate-800 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-400'
                aria-label='Stop playback and close player'
              >
                <i className='fa-solid fa-xmark' aria-hidden='true' />
              </button>
            )}
          </div>

          {/* Mobile progress bar */}
          <div
            className='sm:hidden mt-2'
            role='group'
            aria-label='Playback progress'
          >
            <div
              onClick={handleProgressClick}
              onKeyDown={e => {
                if (!audioRef.current || !duration) return;
                if (e.key === "ArrowLeft") {
                  e.preventDefault();
                  audioRef.current.currentTime = Math.max(0, currentTime - 5);
                } else if (e.key === "ArrowRight") {
                  e.preventDefault();
                  audioRef.current.currentTime = Math.min(
                    duration,
                    currentTime + 5
                  );
                }
              }}
              tabIndex={0}
              className='h-1 bg-slate-700 rounded-full cursor-pointer focus:outline-none focus:ring-2 focus:ring-amber-400'
              role='slider'
              aria-label='Seek through track'
              aria-valuenow={Math.round(currentTime)}
              aria-valuemin={0}
              aria-valuemax={Math.round(duration)}
              aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
            >
              <div
                className='h-full bg-amber-400 rounded-full transition-all'
                style={{ width: `${progress}%` }}
              />
            </div>
            <div className='flex justify-between mt-1'>
              <span
                className='text-xs text-slate-500 tabular-nums'
                aria-hidden='true'
              >
                {formatTime(currentTime)}
              </span>
              <span
                className='text-xs text-slate-500 tabular-nums'
                aria-hidden='true'
              >
                {formatTime(duration)}
              </span>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

// Helper function to format time in mm:ss
function formatTime(seconds: number): string {
  if (!isFinite(seconds) || isNaN(seconds)) return "0:00";
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}
