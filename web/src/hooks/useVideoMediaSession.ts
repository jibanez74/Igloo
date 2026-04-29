import { useEffect, useEffectEvent } from "react";
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

  useEffect(() => {
    if (!enabled || !mediaSessionSupported()) return;

    const video = videoRef.current;
    const state = safePositionState(
      duration,
      currentTime,
      video?.playbackRate ?? 1,
    );
    if (!state) return;

    try {
      navigator.mediaSession.setPositionState?.(state);
    } catch {
      // Some mobile browsers expose Media Session but reject position updates.
    }
  }, [currentTime, duration, enabled, videoRef]);
}
