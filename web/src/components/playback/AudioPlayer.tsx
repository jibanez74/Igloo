import { useRef, useState, useEffect, useEffectEvent } from "react";
import type { TrackType } from "@/types";
import MiniPlayerBar from "@/components/playback/MiniPlayerBar";
import NowPlayingDialog from "@/components/playback/NowPlayingDialog";
import {
  syncMediaSessionPosition,
  useAudioMediaSession,
} from "@/hooks/useAudioMediaSession";
import { useAudioPlaybackKeyboard } from "@/hooks/useAudioPlaybackKeyboard";
import { playMediaElement, toggleMediaPlayback } from "@/lib/audio-utils";

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
  // Driven by the <audio> element's loadstart/canplay/error events, not by a
  // state transition, so useTransition does not apply.
  // react-doctor-disable-next-line react-doctor/rendering-usetransition-loading
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

  // Effects below key on currentTrackId (and other primitives) rather than the
  // track object: queue rebuilds allocate fresh track objects for the same
  // track, and object-identity deps would tear down and re-register handlers
  // for no behavioral reason.
  const trackTitle = track?.title ?? null;

  useAudioMediaSession({
    audioRef,
    currentTrackId,
    trackTitle,
    albumTitle,
    albumCover,
    musicianName,
    isPlaying,
    onStop: () => onClose?.(),
    onPrevious: () => playPrevious(),
    onNext: () => playNext(),
    onSeekBy: seekBy,
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
    if (currentTrackId === null || !audio) return;

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
  }, [audioRef, currentTrackId]);

  const handleTogglePlay = () => {
    // While loading the button is aria-disabled (never `disabled`, which
    // would drop it from the VoiceOver focus order) — guard here instead.
    if (isLoading) return;
    toggleMediaPlayback(audioRef.current);
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

  useAudioPlaybackKeyboard({
    audioRef,
    enabled: currentTrackId !== null && !isKeyboardSuspended,
    onTogglePlay: handleTogglePlay,
    onSeekBy: seekBy,
    onNextTrack: () => playNext(),
    onPreviousTrack: () => playPrevious(),
    onRestart: () => handleSeek(0),
  });

  useEffect(() => {
    if (!audioRef.current || !streamUrl) return;

    const audio = audioRef.current;
    audio.load();
    void playMediaElement(audio);
  }, [audioRef, streamUrl]);

  const transport = {
    isPlaying,
    isLoading,
    hasNext,
    previousAriaLabel,
    nextAriaLabel,
    playPauseAriaLabel,
    onPrevious: playPrevious,
    onTogglePlay: handleTogglePlay,
    onNext: playNext,
  };

  if (!track) return null;

  return (
    <>
      {/* Audio-only music playback; captions do not apply. */}
      {/* react-doctor-disable-next-line react-doctor/media-has-caption */}
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

      <NowPlayingDialog
        track={track}
        artist={artist}
        albumTitle={albumTitle}
        albumCover={albumCover}
        isExpanded={isExpanded}
        onMinimize={handleMinimize}
        onClose={onClose}
        currentTime={currentTime}
        duration={duration}
        onSeek={handleSeek}
        audioRef={audioRef}
        trackPosition={trimmedCount + currentIndex + 1}
        trackTotal={trimmedCount + tracks.length}
        transport={{ ...transport, playPauseButtonRef }}
      />

      {!isExpanded && (
        <MiniPlayerBar
          track={track}
          artist={artist}
          albumCover={albumCover}
          onExpand={onExpand}
          onClose={onClose}
          expandButtonRef={expandButtonRef}
          currentTime={currentTime}
          duration={duration}
          onSeek={handleSeek}
          audioRef={audioRef}
          transport={transport}
        />
      )}
    </>
  );
}
