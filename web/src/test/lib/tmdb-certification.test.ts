import { describe, expect, it } from "vitest";
import { pickTmdbCertification } from "@/lib/tmdb-certification";
import type { MovieDetailsType } from "@/types";

type ReleaseDates = MovieDetailsType["release_dates"];

function releaseDates(results: ReleaseDates["results"]): ReleaseDates {
  return { results };
}

describe("pickTmdbCertification", () => {
  it("prefers the US certification over earlier countries", () => {
    const value = pickTmdbCertification(
      releaseDates([
        { iso_3166_1: "DE", release_dates: [{ certification: "16" }] },
        { iso_3166_1: "US", release_dates: [{ certification: "PG-13" }] },
      ]),
    );

    expect(value).toBe("PG-13");
  });

  it("falls back to the first non-empty certification when there is no US entry", () => {
    const value = pickTmdbCertification(
      releaseDates([
        { iso_3166_1: "FR", release_dates: [{ certification: "" }] },
        { iso_3166_1: "GB", release_dates: [{ certification: "15" }] },
        { iso_3166_1: "DE", release_dates: [{ certification: "16" }] },
      ]),
    );

    expect(value).toBe("15");
  });

  it("skips empty and whitespace-only certifications, including for the US", () => {
    const value = pickTmdbCertification(
      releaseDates([
        { iso_3166_1: "US", release_dates: [{ certification: "   " }] },
        { iso_3166_1: "GB", release_dates: [{ certification: "  12A  " }] },
      ]),
    );

    expect(value).toBe("12A");
  });

  it("returns the trimmed US certification", () => {
    const value = pickTmdbCertification(
      releaseDates([
        { iso_3166_1: "US", release_dates: [{ certification: "  R  " }] },
      ]),
    );

    expect(value).toBe("R");
  });

  it("returns null when no certification is available", () => {
    expect(pickTmdbCertification(undefined)).toBeNull();
    expect(pickTmdbCertification(releaseDates(null))).toBeNull();
    expect(pickTmdbCertification(releaseDates([]))).toBeNull();
    expect(
      pickTmdbCertification(
        releaseDates([{ iso_3166_1: "US", release_dates: null }]),
      ),
    ).toBeNull();
    expect(
      pickTmdbCertification(
        releaseDates([{ iso_3166_1: "US", release_dates: [] }]),
      ),
    ).toBeNull();
  });
});
