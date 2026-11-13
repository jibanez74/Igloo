import { useEffect, useEffectEvent, useSyncExternalStore } from "react";
import type { RefObject } from "react";
import { AUDIO_SEEK_STEP_SECONDS } from "@/lib/constants";
import { playMediaElement } from "@/lib/audio-utils";
import {
  claimMediaSession,
  getMediaSessionOwner,
  ownsMediaSession,
  releaseMediaSession,
  subscribeMediaSessionOwner,
} from "@/lib/media-session";

// There is one music player, so one owner token for it.
const MUSIC_SESSION = Symbol("music");

function trackMetadata(
  trackTitle: string | null,
  albumTitle: string,
  albumCover: string | null,
  musicianName: string | null,
) {
  if (trackTitle === null || typeof MediaMetadata === "undefined") {
    return null;
  }

  // MediaMetadata artwork wants absolute URLs; new URL() leaves already
  // absolute covers untouched and resolves /api paths against the origin.
  const artworkUrl = albumCover
    ? new URL(albumCover, window.location.origin).toString()
    : null;

  return new MediaMetadata({
    title: trackTitle,
    artist: musicianName ?? albumTitle,
    album: albumTitle,
    artwork: artworkUrl ? [{ src: artworkUrl }] : [],
  });
}

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

  // A video page pauses music and takes the session. Music leaves it alone
  // until the reader plays music again or the video lets the session go, then
  // claims it back with its own metadata and full action set: resuming the
  // same track changes nothing else this effect could react to.
  const sessionOwner = useSyncExternalStore(
    subscribeMediaSessionOwner,
    getMediaSessionOwner,
    getMediaSessionOwner,
  );
  const mayClaim =
    sessionOwner === null || sessionOwner === MUSIC_SESSION || isPlaying;

  useEffect(() => {
    if (!("mediaSession" in navigator)) return;

    // Clear stale lock-screen/OS media info once playback stops; otherwise
    // the last track keeps showing after the player is closed.
    if (currentTrackId === null) {
      releaseMediaSession(MUSIC_SESSION);
      return;
    }
    if (!mayClaim) return;

    claimMediaSession(
      MUSIC_SESSION,
      trackMetadata(trackTitle, albumTitle, albumCover, musicianName),
      {
        play: handlePlay,
        pause: handlePause,
        stop: handleStop,
        previoustrack: handlePrevious,
        nexttrack: handleNext,
        seekbackward: handleSeekBackward,
        seekforward: handleSeekForward,
        seekto: handleSeekTo,
      },
    );
  }, [
    audioRef,
    currentTrackId,
    trackTitle,
    albumCover,
    albumTitle,
    musicianName,
    mayClaim,
  ]);

  useEffect(() => {
    if (!ownsMediaSession(MUSIC_SESSION)) return;

    navigator.mediaSession.playbackState = isPlaying ? "playing" : "paused";
  }, [isPlaying, currentTrackId, sessionOwner]);

  useEffect(() => () => releaseMediaSession(MUSIC_SESSION), []);
}

// Report playback position to the OS media controls. Pure DOM work with no
// reactive dependencies, so it lives outside the hook — the player calls it
// from its timeupdate/durationchange handlers.
export function syncMediaSessionPosition(audio: HTMLAudioElement) {
  if (!positionStateSupported() || !ownsMediaSession(MUSIC_SESSION)) {
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

