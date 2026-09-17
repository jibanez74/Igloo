import { YOUTUBE_THUMBNAIL_BASE } from "./constants";

/**
 * Builds a same-origin proxied YouTube thumbnail URL:
 * `{YOUTUBE_THUMBNAIL_BASE}/{key}` (the server fetches hqdefault.jpg).
 * Returns an empty string when `key` is null, undefined, or empty.
 */
export function buildYouTubeThumbnailUrl(
  key: string | null | undefined,
): string {
  if (key == null || key === "") return "";
  return `${YOUTUBE_THUMBNAIL_BASE}/${encodeURIComponent(key)}`;
}
