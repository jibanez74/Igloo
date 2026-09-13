import type { PlaybackMediaRef } from "@/types/playback";

export function movieMediaRef(id: number): PlaybackMediaRef {
  return { kind: "movie", id };
}

export function episodeMediaRef(id: number): PlaybackMediaRef {
  return { kind: "episode", id };
}

/**
 * The route prefix every playback endpoint of a media item hangs off:
 * /stream, /hls/…, /subtitles/…, /technical-details, /watch-progress. Movies
 * and episodes expose the same suffixes, so this is the only place the
 * client knows which family a ref belongs to.
 */
export function mediaApiBasePath(ref: PlaybackMediaRef): string {
  return ref.kind === "movie"
    ? `/api/movies/${ref.id}`
    : `/api/shows/episodes/${ref.id}`;
}

/** Stable string identity for storage and cache keys, e.g. "episode:12". */
export function mediaKey(ref: PlaybackMediaRef): string {
  return `${ref.kind}:${ref.id}`;
}

/** The noun user-facing copy uses for this media. */
export function mediaNoun(ref: PlaybackMediaRef): string {
  return ref.kind;
}
