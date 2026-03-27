import { TMDB_IMAGE_BASE } from "./constants";

/**
 * Builds a full TMDB image URL: `{TMDB_IMAGE_BASE}/{size}{path}`.
 * Normalizes `path` so a leading slash is present (TMDB returns paths like `/abc.jpg` or `abc.jpg`).
 * Returns an empty string when `path` is null, undefined, or empty.
 */
export function buildTmdbImageUrl(
  path: string | null | undefined,
  size: string,
): string {
  if (path == null || path === "") return "";
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return `${TMDB_IMAGE_BASE}/${size}${normalized}`;
}
