import type { LibraryMovieCrewType } from "@/types/movies";

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

const usdCurrencyFormatter = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  maximumFractionDigits: 0,
});

const DATE_ONLY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

// takes in a date string and returns a Date in the viewer's local time
//
// Catalog dates (movie release, album release, show air dates) are stored
// date-only. `new Date("2024-03-01")` parses those as UTC midnight but reads
// back in local time, so every such date rendered a day early west of UTC —
// and a January date reported the previous year. A date-only string is
// therefore split and built as a local date; anything carrying a time or zone
// is left to the normal parser. Use this anywhere a stored date is rendered or
// a calendar field is read off it, never a bare `new Date(stored)`.
export function parseCatalogDate(date: string) {
  const dateOnly = DATE_ONLY_PATTERN.exec(date);

  return dateOnly
    ? new Date(
        Number(dateOnly[1]),
        Number(dateOnly[2]) - 1,
        Number(dateOnly[3]),
      )
    : new Date(date);
}

// takes in a date string and returns a formatted date string
// format is month day, year
export function formatDate(date: string) {
  const d = parseCatalogDate(date);

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
// ("m:ss"); returns an empty string for missing or invalid durations so
// callers can conditionally render it
export function formatTrackDuration(ms: number) {
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }

  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

// Formats bit rate data obtained from ffprobe scan
export function formatBitRate(bitRate: number) {
  return `${Math.round(bitRate / 1000)} kbps`;
}

// Format seconds into a clock timecode for playback positions, including the
// hours field only when needed: "1:23" under an hour, "1:05:23" once it
// crosses an hour. Use this wherever mm:ss would overflow into confusing
// values like "65:23". When rendering a current-time/duration pair, pass
// `forceHours` on the current-time side (keyed on the duration) so the readout
// keeps the duration's shape ("0:05:00 / 2:05:00") instead of changing width
// when playback crosses the hour mark.
export function formatTimecode(
  seconds: number,
  options?: { forceHours?: boolean },
) {
  if (!isFinite(seconds) || isNaN(seconds) || seconds < 0) return "0:00";

  const total = Math.floor(seconds);
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;

  if (hours > 0 || options?.forceHours) {
    return `${hours}:${mins.toString().padStart(2, "0")}:${secs
      .toString()
      .padStart(2, "0")}`;
  }

  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

// Format seconds into words a screen reader can read naturally, e.g.
// "1 hour 5 minutes 23 seconds". Zero-valued fields are dropped, and a value of
// zero reads as "0 seconds" rather than an empty string.
export function formatSpokenTime(seconds: number) {
  if (!isFinite(seconds) || isNaN(seconds) || seconds < 0) return "0 seconds";

  const total = Math.floor(seconds);
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;

  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} ${hours === 1 ? "hour" : "hours"}`);
  if (mins > 0) parts.push(`${mins} ${mins === 1 ? "minute" : "minutes"}`);
  if (secs > 0 || parts.length === 0) {
    parts.push(`${secs} ${secs === 1 ? "second" : "seconds"}`);
  }

  return parts.join(" ");
}

/**
 * "1 hr 35 min" / "45 min" — the abbreviated shape shared by the runtime chip
 * and the resume note. Zero-valued fields are dropped; callers guarantee at
 * least one whole minute.
 */
function hourMinuteText(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} hr`);
  if (minutes > 0) parts.push(`${minutes} min`);

  return parts.join(" ");
}

/** "1 hour 35 minutes" / "45 minutes" — the spoken twin of `hourMinuteText`. */
function hourMinuteSpoken(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} ${hours === 1 ? "hour" : "hours"}`);
  if (minutes > 0) {
    parts.push(`${minutes} ${minutes === 1 ? "minute" : "minutes"}`);
  }

  return parts.join(" ");
}

export function formatRuntimeMinutes(
  minutes: number | null | undefined,
): string | null {
  if (minutes == null || !Number.isFinite(minutes) || minutes <= 0) return null;
  const totalMinutes = Math.floor(minutes);
  if (totalMinutes <= 0) return null;

  return hourMinuteText(totalMinutes);
}

export function formatSpokenRuntimeMinutes(
  minutes: number | null | undefined,
): string | null {
  if (minutes == null || !Number.isFinite(minutes) || minutes <= 0) return null;
  const totalMinutes = Math.floor(minutes);
  if (totalMinutes <= 0) return null;

  return hourMinuteSpoken(totalMinutes);
}

/** A remaining-time label: the compact text shown, plus the words spoken. */
export type TimeLeftLabel = { text: string; spoken: string };

/**
 * Remaining watch time for the movie hero's resume strip and the episode rows,
 * e.g. `{ text: "1 hr 35 min left", spoken: "1 hour 35 minutes left" }`. The
 * abbreviated text is rendered `aria-hidden` next to an `sr-only` span holding
 * the spoken form, since screen readers read "hr"/"min" inconsistently.
 *
 * Rounds up and drops zero-valued fields. Seconds appear only inside the last
 * minute: both surfaces read the position once when they mount, so a seconds
 * field above that would only ever be a stale number.
 */
export function formatTimeLeft(
  progressSec: number,
  durationSec: number,
): TimeLeftLabel {
  const remaining = durationSec - progressSec;
  const totalSeconds = Number.isFinite(remaining)
    ? Math.max(0, Math.ceil(remaining))
    : 0;

  // Rounding up before the branch keeps 59.5s out of the seconds field: it
  // reads "1 min left" rather than "60 sec left".
  if (totalSeconds < 60) {
    const seconds = Math.max(1, totalSeconds);

    return {
      text: `${seconds} sec left`,
      spoken: `${seconds} ${seconds === 1 ? "second" : "seconds"} left`,
    };
  }

  const totalMinutes = Math.ceil(totalSeconds / 60);

  return {
    text: `${hourMinuteText(totalMinutes)} left`,
    spoken: `${hourMinuteSpoken(totalMinutes)} left`,
  };
}

// Format currency for budget/revenue (movie details)
export function formatCurrency(amount: number): string {
  if (!amount) return "-";
  return usdCurrencyFormatter.format(amount);
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
function isYouTubeExtraVideoSite(site: string): boolean {
  return site.trim().toLowerCase() === "youtube";
}

/**
 * Sort order for library extra video `type` (see `mapTmdbVideoType` on the server):
 * trailers first, then special features, then other/unknown.
 */
function extraVideoTypeSortRank(type: string): number {
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

/**
 * YouTube-only extras, sorted: trailers → special features → others, then
 * title. Structural so movie, in-theaters, and show extras all pass through
 * and keep their own element type.
 */
export function prepareYouTubeExtrasForDisplay<
  T extends { title: string; type: string; site: string },
>(videos: T[]): T[] {
  return videos.filter(v => isYouTubeExtraVideoSite(v.site)).sort(
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

/**
 * Season tab and heading label. Season zero is TMDB's specials bucket, which
 * reads as "Specials" rather than "Season 0".
 */
export function seasonLabel(seasonNumber: number): string {
  return seasonNumber === 0 ? "Specials" : `Season ${seasonNumber}`;
}

/** "S1 E3": the compact episode code used in play labels and player titles. */
export function episodeCode(seasonNumber: number, episodeNumber: number): string {
  return `S${seasonNumber} E${episodeNumber}`;
}

/** "Frost Harbor · S1 E3 · Pilot": the player header for an episode. */
export function episodeTitle(
  showName: string,
  seasonNumber: number,
  episodeNumber: number,
  episodeName: string,
): string {
  return `${showName} · ${episodeCode(seasonNumber, episodeNumber)} · ${episodeName}`;
}

/** "1 episode", "3 seasons": count plus the noun, pluralized with an "s". */
export function pluralize(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

/**
 * The form of an explicit singular/plural pair that matches a count - for
 * nouns that carry both forms (`LibraryNoun`) and for counts the caller
 * formats itself. Use `pluralize` when the noun takes a plain "s" and the
 * count needs no formatting.
 */
export function nounForCount(
  count: number,
  noun: { singular: string; plural: string },
): string {
  return count === 1 ? noun.singular : noun.plural;
}

/**
 * How far into a title the viewer is, as a whole percent clamped to 0-100. One
 * definition for every surface that shows it: the card progress bars, the bars
 * on the movie hero and the episode rows, and the "N% watched" announced in a
 * card's link label. An unknown duration reads as 0 rather than NaN.
 */
export function watchProgressPercent(
  progressSec: number,
  durationSec: number,
): number {
  if (!(durationSec > 0)) return 0;

  return Math.min(100, Math.max(0, Math.round((progressSec / durationSec) * 100)));
}
