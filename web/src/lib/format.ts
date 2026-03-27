import type {
  LibraryMovieCrewType,
  LibraryMovieExtraVideoType,
} from "@/types/movies";

const months = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

// takes in a date string and returns a formatted date string
// format is month day, year
export function formatDate(date: string) {
  const d = new Date(date);

  return `${months[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

// takes in a duration in milliseconds and returns a formatted duration string
// the format is hours:minutes:seconds
export function formatDuration(ms: number) {
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }

  return `${minutes}m ${seconds}s`;
}

// takes in a duration in milliseconds and returns a formatted duration string
export function formatTrackDuration(ms: number) {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

// Formats bit rate data obtained from ffprobe scan
export function formatBitRate(bitRate: number) {
  return `${Math.round(bitRate / 1000)} kbps`;
}

// Format seconds into mm:ss format (for audio player progress)
// Handles edge cases like NaN and Infinity
export function formatTimeSeconds(seconds: number) {
  if (!isFinite(seconds) || isNaN(seconds)) return "0:00";

  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);

  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

// Format currency for budget/revenue (movie details)
export function formatCurrency(amount: number): string {
  if (!amount) return "-";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(amount);
}

/** TMDB extra video `type` values → user-facing labels */
const EXTRA_VIDEO_TYPE_LABELS: Record<string, string> = {
  trailer: "Trailer",
  teaser: "Teaser",
  clip: "Clip",
  featurette: "Featurette",
  behind_the_scenes: "Behind the scenes",
  special_feature: "Special feature",
  opening_credits: "Opening credits",
  bloopers: "Bloopers",
  documentary: "Documentary",
};

/** Maps TMDB extra video type strings to readable labels; falls back to title-cased text */
export function formatExtraVideoType(type: string): string {
  const key = type.trim().toLowerCase().replace(/-/g, "_");
  if (EXTRA_VIDEO_TYPE_LABELS[key]) return EXTRA_VIDEO_TYPE_LABELS[key];
  return key
    .split("_")
    .filter(Boolean)
    .map(w => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

/** API stores `site` as lowercase (`youtube`, `vimeo`, `other`). */
export function isYouTubeExtraVideoSite(site: string): boolean {
  return site.trim().toLowerCase() === "youtube";
}

/**
 * Sort order for library extra video `type` (see `mapTmdbVideoType` on the server):
 * trailers first, then special features, then other/unknown.
 */
export function extraVideoTypeSortRank(type: string): number {
  const key = type.trim().toLowerCase().replace(/-/g, "_");
  switch (key) {
    case "trailer":
      return 0;
    case "special_feature":
      return 1;
    case "other":
      return 2;
    default:
      return 3;
  }
}

/** YouTube-only extras, sorted: trailers → special features → others, then title. */
export function prepareYouTubeExtrasForDisplay(
  videos: LibraryMovieExtraVideoType[],
): LibraryMovieExtraVideoType[] {
  return [...videos.filter(v => isYouTubeExtraVideoSite(v.site))].sort(
    (a, b) => {
      const byType =
        extraVideoTypeSortRank(a.type) - extraVideoTypeSortRank(b.type);
      if (byType !== 0) return byType;
      return a.title.localeCompare(b.title, undefined, { sensitivity: "base" });
    },
  );
}

export function sortLibraryCrewForDisplay(
  a: LibraryMovieCrewType,
  b: LibraryMovieCrewType,
): number {
  const byDept = a.department.localeCompare(b.department, undefined, {
    sensitivity: "base",
  });
  if (byDept !== 0) return byDept;
  const byJob = a.job.localeCompare(b.job, undefined, { sensitivity: "base" });
  if (byJob !== 0) return byJob;
  return a.artist_name.localeCompare(b.artist_name, undefined, {
    sensitivity: "base",
  });
}
