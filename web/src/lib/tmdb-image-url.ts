import { TMDB_IMAGE_BASE } from "./constants";

/**
 * Builds a proxied TMDB image URL: `{TMDB_IMAGE_BASE}/{size}{path}`.
 * Normalizes `path` so a leading slash is present (TMDB returns paths like `/abc.jpg` or `abc.jpg`).
 * A path that is already a server URL, such as `/api/static/...` for artwork the
 * server downloaded, is returned as it is: the server stored it at one size.
 * Returns an empty string when `path` is null, undefined, or empty.
 */
export function buildTmdbImageUrl(
  path: string | null | undefined,
  size: string,
): string {
  if (path == null || path === "") return "";
  if (path.startsWith("/api/")) return path;
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return `${TMDB_IMAGE_BASE}/${size}${normalized}`;
}
