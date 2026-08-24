import { useEffect, useEffectEvent, useRef } from "react";
import type { RefObject } from "react";

type VideoMediaSessionOptions = {
  videoRef: RefObject<HTMLVideoElement | null>;
  title: string;
  artist?: string;
  artworkUrl?: string | null;
  currentTime: number;
  duration: number;
  playing: boolean;
  enabled?: boolean;
  seekStepSec?: number;
  onPlay: () => void | Promise<void>;
  onPause: () => void;
  onSeek: (time: number) => void;
};

const DEFAULT_SEEK_STEP_SEC = 10;

/**
 * timeupdate fires ~4x/sec; the OS extrapolates position from the last report,
 * so re-reporting is only needed when the position drifts beyond what a seek
 * would explain or when duration/rate/play-state change.
 */
const MEDIA_SESSION_POSITION_MIN_DELTA_SEC = 5;

function mediaSessionSupported() {
  return (
    typeof navigator !== "undefined" &&
    "mediaSession" in navigator &&
    typeof window !== "undefined"
  );
}

function mediaMetadataSupported() {
  return typeof MediaMetadata !== "undefined";
}

function toAbsoluteArtworkUrl(url: string | null | undefined) {
  if (!url) return null;
  if (url.startsWith("http://") || url.startsWith("https://")) return url;
  return `${window.location.origin}${url}`;
}

function safePositionState(
  duration: number,
  position: number,
  playbackRate: number,
) {
  if (!(duration > 0) || !Number.isFinite(duration)) return null;

  return {
    duration,
    playbackRate: playbackRate > 0 ? playbackRate : 1,
    position: Math.max(0, Math.min(position, duration)),
  };
}

export function useVideoMediaSession({
  videoRef,
  title,
  artist,
  artworkUrl,
  currentTime,
  duration,
  playing,
  enabled = true,
  seekStepSec = DEFAULT_SEEK_STEP_SEC,
  onPlay,
  onPause,
  onSeek,
}: VideoMediaSessionOptions) {
  const handlePlay = useEffectEvent(() => {
    void onPlay();
  });

  const handlePause = useEffectEvent(() => {
    onPause();
  });

  const handleSeekBackward = useEffectEvent(
    ({ seekOffset }: MediaSessionActionDetails) => {
      onSeek(Math.max(0, currentTime - (seekOffset ?? seekStepSec)));
    },
  );

  const handleSeekForward = useEffectEvent(
    ({ seekOffset }: MediaSessionActionDetails) => {
      const targetTime = currentTime + (seekOffset ?? seekStepSec);
      onSeek(duration > 0 ? Math.min(targetTime, duration) : targetTime);
    },
  );

  const handleSeekTo = useEffectEvent(
    ({ seekTime }: MediaSessionActionDetails) => {
      if (seekTime == null) return;
      onSeek(duration > 0 ? Math.min(Math.max(0, seekTime), duration) : seekTime);
    },
  );

  useEffect(() => {
    if (!enabled || !mediaSessionSupported() || !mediaMetadataSupported()) return;

    const artwork = toAbsoluteArtworkUrl(artworkUrl);
    navigator.mediaSession.metadata = new MediaMetadata({
      title,
      artist,
      artwork: artwork ? [{ src: artwork }] : [],
    });

    return () => {
      navigator.mediaSession.metadata = null;
    };
  }, [artist, artworkUrl, enabled, title]);

  useEffect(() => {
    if (!enabled || !mediaSessionSupported()) return;

    navigator.mediaSession.playbackState = playing ? "playing" : "paused";
  }, [enabled, playing]);

  useEffect(() => {
    if (!enabled || !mediaSessionSupported()) return;

    navigator.mediaSession.setActionHandler("play", handlePlay);
    navigator.mediaSession.setActionHandler("pause", handlePause);
    navigator.mediaSession.setActionHandler("seekbackward", handleSeekBackward);
    navigator.mediaSession.setActionHandler("seekforward", handleSeekForward);
    navigator.mediaSession.setActionHandler("seekto", handleSeekTo);

    return () => {
      for (const action of [
        "play",
        "pause",
        "seekbackward",
        "seekforward",
        "seekto",
      ] as const) {
        navigator.mediaSession.setActionHandler(action, null);
      }
    };
  }, [enabled]);

  const lastReportedPositionRef = useRef<{
    position: number;
    duration: number;
    playbackRate: number;
    playing: boolean;
  } | null>(null);

  useEffect(() => {
    if (!enabled || !mediaSessionSupported()) {
      lastReportedPositionRef.current = null;
      return;
    }

    const video = videoRef.current;
    const state = safePositionState(
      duration,
      currentTime,
      video?.playbackRate ?? 1,
    );
    if (!state) return;

    const last = lastReportedPositionRef.current;
    const isRedundantUpdate =
      last !== null &&
      last.duration === state.duration &&
      last.playbackRate === state.playbackRate &&
      last.playing === playing &&
      Math.abs(state.position - last.position) <
        MEDIA_SESSION_POSITION_MIN_DELTA_SEC;
    if (isRedundantUpdate) return;

    try {
      navigator.mediaSession.setPositionState?.(state);
      lastReportedPositionRef.current = { ...state, playing };
    } catch {
      // Some mobile browsers expose Media Session but reject position updates.
    }
  }, [currentTime, duration, enabled, playing, videoRef]);
}
