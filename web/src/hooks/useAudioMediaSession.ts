import { useEffect, useEffectEvent } from "react";
import type { RefObject } from "react";
import { AUDIO_SEEK_STEP_SECONDS } from "@/lib/constants";
import { playMediaElement } from "@/lib/audio-utils";

const MEDIA_SESSION_ACTIONS = [
  "play",
  "pause",
  "stop",
  "previoustrack",
  "nexttrack",
  "seekbackward",
  "seekforward",
  "seekto",
] as const;

function positionStateSupported() {
  return (
    typeof navigator !== "undefined" &&
    "mediaSession" in navigator &&
    typeof navigator.mediaSession?.setPositionState === "function"
  );
}

type AudioMediaSessionOptions = {
  audioRef: RefObject<HTMLAudioElement | null>;

  // Everything below is a primitive: queue rebuilds allocate fresh track
  // objects for the same track, and object-identity deps would tear down and
  // re-register the OS handlers for no behavioral reason.
  currentTrackId: number | null;
  trackTitle: string | null;
  albumTitle: string;
  albumCover: string | null;
  musicianName: string | null;
  isPlaying: boolean;

  onStop: () => void;
  onPrevious: () => void;
  onNext: () => void;
  onSeekBy: (offsetSeconds: number) => void;
};

// Publishes the current track to the OS/lock-screen media controls and wires
// their buttons back into the player. Returns a position-sync callback the
// player calls from its timeupdate/durationchange handlers.
export function useAudioMediaSession({
  audioRef,
  currentTrackId,
  trackTitle,
  albumTitle,
  albumCover,
  musicianName,
  isPlaying,
  onStop,
  onPrevious,
  onNext,
  onSeekBy,
}: AudioMediaSessionOptions) {
  useEffect(() => {
    if (!("mediaSession" in navigator)) {
      return;
    }

    // Clear stale lock-screen/OS media info once playback stops; otherwise
    // the last track keeps showing after the player is closed.
    if (
      currentTrackId === null ||
      trackTitle === null ||
      typeof MediaMetadata === "undefined"
    ) {
      navigator.mediaSession.metadata = null;
      return;
    }

    // MediaMetadata artwork wants absolute URLs; new URL() leaves already
    // absolute covers untouched and resolves /api paths against the origin.
    const artworkUrl = albumCover
      ? new URL(albumCover, window.location.origin).toString()
      : null;

    navigator.mediaSession.metadata = new MediaMetadata({
      title: trackTitle,
      artist: musicianName ?? albumTitle,
      album: albumTitle,
      artwork: artworkUrl ? [{ src: artworkUrl }] : [],
    });
  }, [currentTrackId, trackTitle, albumCover, albumTitle, musicianName]);

  useEffect(() => {
    if (!("mediaSession" in navigator)) return;

    navigator.mediaSession.playbackState =
      currentTrackId === null ? "none" : isPlaying ? "playing" : "paused";
  }, [isPlaying, currentTrackId]);

  const handlePlay = useEffectEvent(() => {
    void playMediaElement(audioRef.current);
  });

  const handlePause = useEffectEvent(() => {
    audioRef.current?.pause();
  });

  const handleStop = useEffectEvent(() => {
    onStop();
  });

  const handlePrevious = useEffectEvent(() => {
    onPrevious();
  });

  const handleNext = useEffectEvent(() => {
    onNext();
  });

  const handleSeekBackward = useEffectEvent(
    ({ seekOffset }: { seekOffset?: number }) => {
      onSeekBy(-(seekOffset ?? AUDIO_SEEK_STEP_SECONDS));
    },
  );

  const handleSeekForward = useEffectEvent(
    ({ seekOffset }: { seekOffset?: number }) => {
      onSeekBy(seekOffset ?? AUDIO_SEEK_STEP_SECONDS);
    },
  );

  const handleSeekTo = useEffectEvent(({ seekTime }: { seekTime?: number }) => {
    const audio = audioRef.current;
    if (!audio || seekTime == null) return;

    audio.currentTime = seekTime;
  });

  useEffect(() => {
    if (currentTrackId === null || !("mediaSession" in navigator)) return;

    navigator.mediaSession.setActionHandler("play", handlePlay);
    navigator.mediaSession.setActionHandler("pause", handlePause);
    navigator.mediaSession.setActionHandler("stop", handleStop);
    navigator.mediaSession.setActionHandler("previoustrack", handlePrevious);
    navigator.mediaSession.setActionHandler("nexttrack", handleNext);
    navigator.mediaSession.setActionHandler("seekbackward", handleSeekBackward);
    navigator.mediaSession.setActionHandler("seekforward", handleSeekForward);
    navigator.mediaSession.setActionHandler("seekto", handleSeekTo);

    return () => {
      for (const action of MEDIA_SESSION_ACTIONS) {
        navigator.mediaSession.setActionHandler(action, null);
      }
    };
  }, [audioRef, currentTrackId]);
}

// Report playback position to the OS media controls. Pure DOM work with no
// reactive dependencies, so it lives outside the hook — the player calls it
// from its timeupdate/durationchange handlers.
export function syncMediaSessionPosition(audio: HTMLAudioElement) {
  if (!positionStateSupported()) {
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

  // Optional chaining inside a try block prevents the React Compiler from
  // compiling this function, so check for the method up front.
  if (typeof navigator.mediaSession.setPositionState !== "function") {
    return;
  }
  const position = Math.max(0, Math.min(currentPosition, audioDuration));

  try {
    navigator.mediaSession.setPositionState({
      duration: audioDuration,
      playbackRate,
      position,
    });
  } catch {
    // Some browsers expose Media Session but reject position updates.
  }
}

