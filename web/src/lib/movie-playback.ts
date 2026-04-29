import { updateMovieWatchProgress } from "@/lib/api";
import { HLS_RESUME_REWIND_BUFFER_SEC } from "@/lib/constants";
import {
  STREAM_MODES,
  formatSubtitleLabel,
  type StreamModeId,
} from "@/lib/playback";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import type { SubtitleType } from "@/types/movies";

export type MoviePlaybackStatus =
  | { kind: "ready" }
  | { kind: "notFound" }
  | { kind: "loading"; message: string }
  | { kind: "modeUnavailable"; modeLabel: string }
  | { kind: "error"; message: string };

export function deriveMoviePlaybackStatus(args: {
  movieNotFound: boolean;
  movieIsPending: boolean;
  hasMovie: boolean;
  requestedMode: StreamModeId;
  techPending: boolean;
  modeUnavailable: boolean;
  playbackError: string | null;
}): MoviePlaybackStatus {
  if (args.movieNotFound) return { kind: "notFound" };
  if (args.movieIsPending || !args.hasMovie) {
    return { kind: "loading", message: "Loading movie..." };
  }
  if (args.requestedMode !== "direct" && args.techPending) {
    return { kind: "loading", message: "Preparing playback..." };
  }
  if (args.modeUnavailable) {
    const modeLabel =
      STREAM_MODES.find((m) => m.id === args.requestedMode)?.label ??
      args.requestedMode;
    return { kind: "modeUnavailable", modeLabel };
  }
  if (args.playbackError) return { kind: "error", message: args.playbackError };
  return { kind: "ready" };
}

export const MOVIE_SEEK_STEP_SEC = 10;
export const MOVIE_VOLUME_STEP = 0.1;
export const MOVIE_CONTROLS_IDLE_MS = 3000;
export const MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS = 15_000;
export const MOVIE_WATCH_PROGRESS_MIN_SECONDS = 180;
export const MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD = 0.98;
export const MOVIE_HLS_FORWARD_REBASE_THRESHOLD_SEC = 120;

const MEDIA_ERR_NETWORK = 2;
const MEDIA_ERR_DECODE = 3;
const MEDIA_ERR_SRC_NOT_SUPPORTED = 4;

type PlaybackTimingOptions = {
  isHlsPlayback: boolean;
  hlsStartSec: number;
  movieDurationSec?: number;
};

type SubtitleTrackInfoOptions = {
  movieId: number;
  resolvedSubtitleTrack: number | null;
  techLoaded: boolean;
  subtitleStreams: SubtitleType[];
};

type RebaseOptions = {
  isHlsPlayback: boolean;
  targetTimeSec: number;
  hlsStartSec: number;
  currentVideoTimeSec: number;
};

export function buildMovieStreamUrl(
  movieId: number,
  mode: StreamModeId,
  audioTrack: number,
  hlsStartSec: number,
  reloadKey: number,
): string {
  if (mode === "direct") return `/api/movies/${movieId}/stream`;

  const params = new URLSearchParams({
    audio_track: String(audioTrack),
  });
  if (hlsStartSec > 0) {
    params.set("start", String(Math.floor(hlsStartSec)));
  }
  if (reloadKey > 0) {
    params.set("reload", String(reloadKey));
  }

  return `/api/movies/${movieId}/hls/${mode}/playlist.m3u8?${params}`;
}

export function hlsStartTimeSec(isHlsPlayback: boolean, startSec: number) {
  return isHlsPlayback
    ? Math.max(0, startSec - HLS_RESUME_REWIND_BUFFER_SEC)
    : 0;
}

export function hlsPlaybackOffsetSec(
  isHlsPlayback: boolean,
  startSec: number,
  hlsStartSec: number,
) {
  return isHlsPlayback ? Math.max(0, startSec - hlsStartSec) : 0;
}

export function toAbsolutePlaybackTime(
  timeSec: number,
  { isHlsPlayback, hlsStartSec }: PlaybackTimingOptions,
) {
  return isHlsPlayback ? hlsStartSec + timeSec : timeSec;
}

export function toMediaPlaybackTime(
  timeSec: number,
  { isHlsPlayback, hlsStartSec }: PlaybackTimingOptions,
) {
  return isHlsPlayback ? Math.max(0, timeSec - hlsStartSec) : timeSec;
}

export function toAbsoluteDuration(
  durationSec: number,
  { isHlsPlayback, hlsStartSec, movieDurationSec }: PlaybackTimingOptions,
) {
  if (!isHlsPlayback) return durationSec;
  if (movieDurationSec && movieDurationSec > 0) {
    return movieDurationSec;
  }
  return hlsStartSec + durationSec;
}

export function displayedMovieDuration(
  durationSec: number,
  { isHlsPlayback, movieDurationSec }: PlaybackTimingOptions,
) {
  if (isHlsPlayback && movieDurationSec && movieDurationSec > 0) {
    return movieDurationSec;
  }
  return durationSec;
}

export function buildMovieSubtitleTrackInfo({
  movieId,
  resolvedSubtitleTrack,
  techLoaded,
  subtitleStreams,
}: SubtitleTrackInfoOptions) {
  if (resolvedSubtitleTrack === null || !techLoaded) return null;
  if (
    resolvedSubtitleTrack < 0 ||
    resolvedSubtitleTrack >= subtitleStreams.length
  ) {
    return null;
  }

  const sub = subtitleStreams[resolvedSubtitleTrack];
  return {
    url: `/api/movies/${movieId}/subtitles/${resolvedSubtitleTrack}/web.vtt`,
    label: formatSubtitleLabel(sub, resolvedSubtitleTrack),
    srclang: unwrapStringOrUndefined(sub.language) ?? "",
  };
}

export function hasEligibleMovieResumeProgress(
  progressSec: number | null,
  durationSec: number | null,
) {
  return (
    progressSec !== null &&
    durationSec !== null &&
    durationSec > 0 &&
    progressSec >= MOVIE_WATCH_PROGRESS_MIN_SECONDS &&
    progressSec / durationSec < MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD
  );
}

export function clampMoviePlaybackTime(
  value: number,
  currentDuration: number,
  fallbackDuration: number,
) {
  const knownDuration =
    currentDuration > 0
      ? currentDuration
      : fallbackDuration > 0
        ? fallbackDuration
        : value;
  return Math.max(0, Math.min(value, knownDuration));
}

export function shouldRebaseHlsMovieSession({
  isHlsPlayback,
  targetTimeSec,
  hlsStartSec,
  currentVideoTimeSec,
}: RebaseOptions) {
  return (
    isHlsPlayback &&
    (targetTimeSec < hlsStartSec ||
      targetTimeSec >
        currentVideoTimeSec + MOVIE_HLS_FORWARD_REBASE_THRESHOLD_SEC)
  );
}

export function currentPlaybackTimestampMs() {
  return Date.now();
}

export function nativeMoviePlaybackErrorMessage(
  code: number | null | undefined,
) {
  if (code === MEDIA_ERR_SRC_NOT_SUPPORTED) {
    return "This media format is not supported by the browser.";
  }
  if (code === MEDIA_ERR_DECODE) {
    return "The stream could not be decoded.";
  }
  if (code === MEDIA_ERR_NETWORK) {
    return "A network error interrupted playback.";
  }
  return "Playback failed.";
}

export function shouldPersistMovieWatchProgress(
  progressSec: number,
  durationSec: number,
) {
  if (!(durationSec > 0)) return false;
  const clampedProgress = Math.max(0, Math.min(progressSec, durationSec));
  const completionRatio = clampedProgress / durationSec;

  return (
    completionRatio >= MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD ||
    clampedProgress >= MOVIE_WATCH_PROGRESS_MIN_SECONDS
  );
}

export async function persistMovieWatchProgress(
  movieId: number,
  progressSec: number,
  durationSec: number,
  options?: { keepalive?: boolean },
) {
  if (!(durationSec > 0)) return;

  const clampedProgress = Math.max(0, Math.min(progressSec, durationSec));
  if (!shouldPersistMovieWatchProgress(clampedProgress, durationSec)) return;

  if (options?.keepalive) {
    try {
      await fetch(`/api/movies/${movieId}/watch-progress`, {
        method: "PUT",
        credentials: "include",
        keepalive: true,
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          progress_sec: clampedProgress,
          duration_sec: durationSec,
        }),
      });
    } catch {
      // Best-effort pagehide save; ignore network failures.
    }
    return;
  }

  const res = await updateMovieWatchProgress(
    movieId,
    clampedProgress,
    durationSec,
  );
  if (res.error) {
    throw new Error(res.message);
  }
}
