import type { MovieDetailsType } from "@/types";

/**
 * Picks the parental certification (PG, PG-13, R, ...) out of a TMDB
 * `release_dates` payload. Prefers the US certification; otherwise falls back to
 * the first non-empty certification from any country. Returns null when none is
 * available.
 *
 * Mirrors the server's `TmdbMovie.Certification()` so in-theaters movies read the
 * same as library movies.
 */
export function pickTmdbCertification(
  releaseDates: MovieDetailsType["release_dates"] | undefined,
): string | null {
  let firstCert: string | null = null;

  for (const result of releaseDates?.results ?? []) {
    for (const entry of result.release_dates ?? []) {
      const cert = entry.certification.trim();
      if (cert === "") {
        continue;
      }
      if (result.iso_3166_1 === "US") {
        return cert;
      }
      if (firstCert === null) {
        firstCert = cert;
      }
    }
  }

  return firstCert;
}
