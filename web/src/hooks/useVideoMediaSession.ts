import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import type { RefObject } from "react";
import {
  claimMediaSession,
  getMediaSessionOwner,
  releaseMediaSession,
  subscribeMediaSessionOwner,
  updateMediaSessionMetadata,
} from "@/lib/media-session";

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

function videoMetadata(
  title: string,
  artist: string | undefined,
  artworkUrl: string | null | undefined,
) {
  if (!mediaMetadataSupported()) return null;

  const artwork = toAbsoluteArtworkUrl(artworkUrl);
  return new MediaMetadata({
    title,
    artist,
    artwork: artwork ? [{ src: artwork }] : [],
  });
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

  // This surface's claim on the OS media controls (see lib/media-session):
  // the full action set replaces music's, and letting go clears the session
  // only while this surface still holds it, so music can take it back. Held
  // for as long as the surface is enabled: a title or artwork change only
  // replaces the metadata, so the play state and timeline stay published.
  const [sessionToken] = useState(() => Symbol("video"));
  const sessionOwner = useSyncExternalStore(
    subscribeMediaSessionOwner,
    getMediaSessionOwner,
    getMediaSessionOwner,
  );
  const ownsSession = sessionOwner === sessionToken;
  const sessionFree = sessionOwner === null;

  const claimSession = useEffectEvent(() => {
    claimMediaSession(
      sessionToken,
      videoMetadata(title, artist, artworkUrl),
      {
        play: handlePlay,
        pause: handlePause,
        seekbackward: handleSeekBackward,
        seekforward: handleSeekForward,
        seekto: handleSeekTo,
      },
    );
  });

  useEffect(() => {
    if (!enabled || !mediaSessionSupported()) return;

    claimSession();

    return () => releaseMediaSession(sessionToken);
  }, [enabled, sessionToken]);

  // Music played over this video and then closed: the controls are free
  // again, and the surface still on screen takes them back.
  useEffect(() => {
    if (enabled && sessionFree && mediaSessionSupported()) claimSession();
  }, [enabled, sessionFree]);

  useEffect(() => {
    if (!enabled) return;

    updateMediaSessionMetadata(
      sessionToken,
      videoMetadata(title, artist, artworkUrl),
    );
  }, [artist, artworkUrl, enabled, sessionToken, title]);

  useEffect(() => {
    if (!enabled || !ownsSession) return;

    navigator.mediaSession.playbackState = playing ? "playing" : "paused";
  }, [enabled, ownsSession, playing]);

  const lastReportedPositionRef = useRef<{
    position: number;
    duration: number;
    playbackRate: number;
    playing: boolean;
  } | null>(null);

  useEffect(() => {
    // Re-run on (re)gaining the session too: a claim cleared whatever timeline
    // the previous owner left, so the next report must not be skipped as
    // redundant.
    if (!enabled || !ownsSession) {
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
  }, [currentTime, duration, enabled, ownsSession, playing, videoRef]);
}
