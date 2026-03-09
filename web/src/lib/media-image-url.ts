/**
 * Returns the display URL for a media image (album cover, musician thumb, etc.).
 * Handles both local paths (starting with /api) and remote URLs (starting with http).
 * Use this for img src when displaying backend-provided image paths.
 * Returns null for empty or invalid values.
 */
export function getMediaImageUrl(
  value: string | null | undefined
): string | null {
  if (value == null || typeof value !== "string") return null;
  const s = value.trim();
  if (!s) return null;
  // Local same-origin path (e.g. /api/static/albums/1.jpg)
  if (s.startsWith("/api")) return s;
  // Remote URL
  if (s.startsWith("http://") || s.startsWith("https://")) return s;
  return null;
}
