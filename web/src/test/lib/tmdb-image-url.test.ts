import { describe, expect, it } from "vitest";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";

describe("buildTmdbImageUrl", () => {
  it("proxies a TMDB path at the requested size", () => {
    expect(buildTmdbImageUrl("/abc.jpg", TMDB_POSTER_SIZE)).toBe(
      "/api/tmdb/images/w500/abc.jpg",
    );
    expect(buildTmdbImageUrl("abc.jpg", TMDB_POSTER_SIZE)).toBe(
      "/api/tmdb/images/w500/abc.jpg",
    );
  });

  it("returns artwork the server downloaded as it is", () => {
    const copy = "/api/static/movies/603/the_matrix_poster.jpg?v=1a2b3c4d";
    expect(buildTmdbImageUrl(copy, TMDB_POSTER_SIZE)).toBe(copy);
  });

  it("is empty without a path", () => {
    expect(buildTmdbImageUrl(null, TMDB_POSTER_SIZE)).toBe("");
    expect(buildTmdbImageUrl(undefined, TMDB_POSTER_SIZE)).toBe("");
    expect(buildTmdbImageUrl("", TMDB_POSTER_SIZE)).toBe("");
  });
});
