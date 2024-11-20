// Blurhash placeholders optimized for TMDB images
export const imagePlaceholders = {
  // Optimized for movie posters (2:3 aspect ratio)
  poster: "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
  
  // Optimized for backdrops (16:9 aspect ratio)
  backdrop: "L6PZfSi_.AyE_3t7t7R**0o#DgR4"
} as const;

// TMDB image base URLs
const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/";

export const imageSize = {
  poster: {
    original: "original",
    w500: "w500",
    w342: "w342",
    w185: "w185"
  },
  backdrop: {
    original: "original",
    w1280: "w1280",
    w780: "w780"
  }
} as const;

type ImageType = keyof typeof imagePlaceholders;
type PosterSize = keyof typeof imageSize.poster;
type BackdropSize = keyof typeof imageSize.backdrop;

export const getTMDBImageURL = (
  path: string | null,
  type: "poster",
  size: PosterSize
): string;
export const getTMDBImageURL = (
  path: string | null,
  type: "backdrop",
  size: BackdropSize
): string;
export const getTMDBImageURL = (
  path: string | null,
  type: ImageType,
  size: PosterSize | BackdropSize
): string {
  if (!path) return "";
  return `${TMDB_IMAGE_BASE}${imageSize[type][size]}${path}`;
}

export const getPlaceholder = (type: ImageType) => {
  return imagePlaceholders[type];
}; 