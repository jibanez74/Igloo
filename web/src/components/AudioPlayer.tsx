import { useRef, useState, useEffect } from "react";
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

  // Handle play/pause toggle
  const handleTogglePlay = () => {
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

  // Handle seeking to a specific time (used by ProgressBar component)
  const handleSeek = (newTime: number) => {
    if (!audioRef.current) return;
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
          className='fixed inset-0 z-50 flex animate-in flex-col bg-linear-to-b from-slate-900 via-slate-800
            to-slate-900 duration-300 zoom-in-95 fade-in slide-in-from-bottom-2'
        >
          {/* Screen reader live region for playback status - only announces on meaningful changes */}
          <div className='sr-only' aria-live='polite' aria-atomic='true'>
            {announcement}
          </div>

          {/* Header with minimize button */}
          <header className='flex items-center justify-between px-6 py-4'>
            <button
              onClick={onMinimize}
              className='flex size-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
              aria-label='Minimize player (Escape)'
            >
              <i
                className='fa-solid fa-chevron-down text-lg'
                aria-hidden='true'
              />
            </button>
            <div className='text-center' id='player-header'>
              <p className='text-xs tracking-widest text-slate-500 uppercase'>
                Now Playing
              </p>
              <p className='mt-0.5 text-sm text-slate-400'>{albumTitle}</p>
            </div>
            {onClose ? (
              <button
                onClick={onClose}
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
                aria-label='Stop playback and close player'
              >
                <i className='fa-solid fa-xmark text-lg' aria-hidden='true' />
              </button>
            ) : (
              <div className='h-10 w-10' aria-hidden='true' />
            )}
          </header>

          {/* Main content */}
          <main className='flex flex-1 flex-col items-center justify-center px-8 pb-8'>
            {/* Album cover */}
            <figure className='mb-8 size-72 overflow-hidden rounded-2xl shadow-2xl shadow-black/50 sm:size-80 md:size-96'>
              {coverUrl ? (
                <img
                  src={coverUrl}
                  alt={`Album cover for ${albumTitle}`}
                  className='size-full object-cover'
                />
              ) : (
                <div
                  className='flex h-full w-full items-center justify-center bg-slate-800'
                  role='img'
                  aria-label='No album cover available'
                >
                  <i
                    className='fa-solid fa-compact-disc text-8xl text-slate-600'
                    aria-hidden='true'
                  />
                </div>
              )}
            </figure>

            {/* Track info */}
            <div className='mb-8 max-w-md text-center'>
              <h1
                id='track-title'
                className='truncate text-2xl font-bold text-white sm:text-3xl'
              >
                {track.title}
              </h1>
              <p className='mt-1 truncate text-lg text-amber-400'>{artist}</p>
            </div>

            {/* Progress bar */}
            <ProgressBar
              currentTime={currentTime}
              duration={duration}
              onSeek={handleSeek}
              variant="expanded"
            />

            {/* Playback controls */}
            <div
              className='flex items-center gap-6'
              role='group'
              aria-label='Playback controls'
            >
              <button
                onClick={playPrevious}
                disabled={!hasPrevious}
                className='flex h-14 w-14 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30'
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
                onClick={handleTogglePlay}
                disabled={isLoading}
                className='flex h-20 w-20 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-xl shadow-amber-500/30 transition-colors hover:bg-amber-400 focus:ring-4 focus:ring-amber-400/50 focus:outline-none disabled:opacity-50'
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
                    className='fa-solid fa-play ml-1 text-3xl'
                    aria-hidden='true'
                  />
                )}
              </button>

              <button
                onClick={playNext}
                disabled={!hasNext}
                className='flex h-14 w-14 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800/50 hover:text-white focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30'
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

            {/* Volume control */}
            <div className='mt-6'>
              <VolumeControl audioRef={audioRef} variant='expanded' />
            </div>

            {/* Track number indicator */}
            <p className='mt-4 text-sm text-slate-500'>
              Track {currentIndex + 1} of {tracks.length}
            </p>
          </main>
        </div>
      )}

      {/* Minimized (bottom bar) mode - always rendered when track exists */}
      <div
        role='region'
        aria-label='Audio player'
        className='fixed right-0 bottom-0 left-0 z-40 animate-in border-t border-slate-700/50 bg-slate-900/95 shadow-2xl shadow-black/50
          backdrop-blur-lg duration-300 fade-in slide-in-from-bottom'
      >
        <div className='mx-auto max-w-7xl px-4 py-3'>
          <div className='flex items-center gap-4'>
            {/* Album cover and track info - clickable to expand */}
            <button
              onClick={onExpand}
              className='flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left transition-opacity hover:opacity-80 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
              aria-label={`Expand player. Now playing: ${track.title} by ${artist}`}
            >
              {/* Album cover */}
              <div className='h-12 w-12 shrink-0 overflow-hidden rounded-lg bg-slate-800 shadow-lg'>
                {coverUrl ? (
                  <img
                    src={coverUrl}
                    alt={albumTitle}
                    className='h-full w-full object-cover'
                  />
                ) : (
                  <div className='flex h-full w-full items-center justify-center'>
                    <i
                      className='fa-solid fa-compact-disc text-lg text-slate-600'
                      aria-hidden='true'
                    />
                  </div>
                )}
              </div>

              {/* Track info */}
              <div className='min-w-0 flex-1'>
                <p className='truncate text-sm font-medium text-white'>
                  {track.title}
                </p>
                <p className='truncate text-xs text-slate-400'>{artist}</p>
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
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30'
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
                onClick={handleTogglePlay}
                disabled={isLoading}
                className='flex h-12 w-12 items-center justify-center rounded-full bg-amber-500 text-slate-900 shadow-lg shadow-amber-500/20 transition-colors hover:bg-amber-400 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50'
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
                className='flex h-10 w-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none disabled:cursor-not-allowed disabled:opacity-30'
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
            <ProgressBar
              currentTime={currentTime}
              duration={duration}
              onSeek={handleSeek}
              variant="minimized"
            />

            {/* Volume control */}
            <div className='hidden sm:block'>
              <VolumeControl audioRef={audioRef} variant='minimized' />
            </div>

            {/* Expand button */}
            <button
              onClick={onExpand}
              className='hidden h-8 w-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none sm:flex'
              aria-label='Expand to fullscreen player'
            >
              <i className='fa-solid fa-chevron-up' aria-hidden='true' />
            </button>

            {/* Close button */}
            {onClose && (
              <button
                onClick={onClose}
                className='flex h-8 w-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                aria-label='Stop playback and close player'
              >
                <i className='fa-solid fa-xmark' aria-hidden='true' />
              </button>
            )}
          </div>

          {/* Mobile progress bar */}
          <ProgressBar
            currentTime={currentTime}
            duration={duration}
            onSeek={handleSeek}
            variant="mobile"
          />
        </div>
      </div>
    </>
  );
}
