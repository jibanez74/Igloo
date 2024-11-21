// Blurhash placeholders optimized for TMDB images
export const imagePlaceholders = {
  poster: "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
  backdrop: "L6PZfSi_.AyE_3t7t7R**0o#DgR4",
} as const;

// TMDB image base URL
const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/";

// Fixed sizes for TV platform
const POSTER_SIZE = "w500";
const BACKDROP_SIZE = "w1280";

export function getTMDBImageURL(
  path: string | null,
  type: "poster" | "backdrop"
): string {
  if (!path) return "";

  const size = type === "poster" ? POSTER_SIZE : BACKDROP_SIZE;
  return `${TMDB_IMAGE_BASE}${size}${path}`;
}

export const getPlaceholder = (
  type: keyof typeof imagePlaceholders
): string => {
  return imagePlaceholders[type];
};
