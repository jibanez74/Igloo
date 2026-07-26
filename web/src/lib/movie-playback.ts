import { updateMovieWatchProgress } from "@/lib/api";
import {
  HLS_PLAYBACK_SESSION_QUERY_PARAM,
  HLS_RESUME_REWIND_BUFFER_SEC,
  MEDIA_ERR_DECODE,
  MEDIA_ERR_NETWORK,
  MEDIA_ERR_SRC_NOT_SUPPORTED,
  MOVIE_HLS_FORWARD_REBASE_THRESHOLD_SEC,
  MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD,
  MOVIE_WATCH_PROGRESS_MIN_SECONDS,
  STREAM_MODES,
} from "@/lib/constants";
import { formatSubtitleLabel, normalizeLang } from "@/lib/playback";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import type {
  MoviePlaybackStatus,
  StreamModeId,
} from "@/types/playback";
import type { SubtitleType } from "@/types/movies";

type MoviePlaybackStatusArgs = {
  movieNotFound: boolean;
  movieIsPending: boolean;
  hasMovie: boolean;
  requestedMode: StreamModeId;
  techPending: boolean;
  playbackPreferencesReady: boolean;
  modeUnavailable: boolean;
  playbackError: string | null;
};

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

const MOVIE_HLS_PLAYBACK_SESSION_STORAGE_PREFIX = "igloo:movie-hls-playback-session:";
const HLS_PLAYBACK_SESSION_ID_PATTERN = new RegExp(
  "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$",
);

type HlsPlaybackSessionStorage = Pick<Storage, "getItem" | "setItem">;

function movieHlsPlaybackSessionStorageKey(movieId: number): string {
  return MOVIE_HLS_PLAYBACK_SESSION_STORAGE_PREFIX + String(movieId);
}

function browserSessionStorage(): HlsPlaybackSessionStorage | null {
  if (typeof window === "undefined") return null;

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function getOrCreateMovieHlsPlaybackSessionId(
  movieId: number,
  storage: HlsPlaybackSessionStorage | null = browserSessionStorage(),
): string {
  const create = () => createPlaybackSessionId();
  if (!Number.isFinite(movieId) || movieId <= 0 || !storage) return create();

  const key = movieHlsPlaybackSessionStorageKey(movieId);
  try {
    const existing = storage.getItem(key);
    if (existing && HLS_PLAYBACK_SESSION_ID_PATTERN.test(existing)) {
      return existing;
    }

    const next = create();
    storage.setItem(key, next);
    return next;
  } catch {
    return create();
  }
}

export async function stopMovieHlsPlaybackSession(
  movieId: number,
  playbackSessionId: string,
  options?: { keepalive?: boolean },
): Promise<void> {
  if (
    !Number.isFinite(movieId) ||
    movieId <= 0 ||
    !HLS_PLAYBACK_SESSION_ID_PATTERN.test(playbackSessionId)
  ) {
    return;
  }

  const params = new URLSearchParams({
    [HLS_PLAYBACK_SESSION_QUERY_PARAM]: playbackSessionId,
  });

  try {
    await fetch(`/api/movies/${movieId}/hls/session/stop?${params.toString()}`, {
      method: "POST",
      credentials: "include",
      keepalive: options?.keepalive === true,
    });
  } catch {
    // Best-effort HLS cleanup; server TTL remains the fallback.
  }
}

export function deriveMoviePlaybackStatus(
  args: MoviePlaybackStatusArgs,
): MoviePlaybackStatus {
  if (args.movieNotFound) return { kind: "notFound" };
  if (args.movieIsPending || !args.hasMovie) {
    return { kind: "loading", message: "Loading movie..." };
  }
  if (!args.playbackPreferencesReady) {
    return { kind: "loading", message: "Preparing playback..." };
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

export function createPlaybackSessionId(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }

  const bytes = new Uint8Array(16);
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i += 1) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, "0"));
  return `${hex.slice(0, 4).join("")}-${hex.slice(4, 6).join("")}-${hex.slice(6, 8).join("")}-${hex.slice(8, 10).join("")}-${hex.slice(10, 16).join("")}`;
}

export function buildMovieStreamUrl(
  movieId: number,
  mode: StreamModeId,
  audioTrack: number | null,
  hlsStartSec: number,
  reloadKey: number,
  playbackSessionId: string,
): string {
  // Direct play serves the raw container, so there is no track to select;
  // resolvePlaybackSettings guarantees audioTrack is the first stream here.
  if (mode === "direct") return `/api/movies/${movieId}/stream`;

  const params = new URLSearchParams({
    [HLS_PLAYBACK_SESSION_QUERY_PARAM]: playbackSessionId,
    start: String(Math.floor(hlsStartSec)),
  });
  if (audioTrack !== null) {
    params.set("audio_track", String(audioTrack));
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
    srclang: normalizeLang(unwrapStringOrUndefined(sub.language)) ?? "",
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

// shouldPersistMovieWatchProgress intentionally uses OR so near-complete short videos are saved when completion >= MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD or clampedProgress >= MOVIE_WATCH_PROGRESS_MIN_SECONDS; hasEligibleMovieResumeProgress uses AND to only surface resume for unfinished, sufficiently-long content.
function shouldPersistMovieWatchProgress(
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
  saveSessionId: string,
  saveSequence: number,
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
          save_session_id: saveSessionId,
          save_sequence: saveSequence,
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
    saveSessionId,
    saveSequence,
  );
  if (res.error) {
    throw new Error(res.message);
  }
}
