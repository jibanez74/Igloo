import { describe, expect, it } from "vitest";
import { listenHead, movieHead, routeHead, showHead } from "@/lib/route-head";

describe("routeHead", () => {
  it("suffixes the title with the app name", () => {
    expect(routeHead("Movies")).toEqual({
      meta: [{ title: "Movies - Igloo" }],
    });
  });

  it("adds the description only when there is one", () => {
    expect(routeHead("Movies", "Your movies.").meta).toEqual([
      { title: "Movies - Igloo" },
      { name: "description", content: "Your movies." },
    ]);
    expect(routeHead("Movies", "").meta).toHaveLength(1);
    expect(routeHead("Movies", null).meta).toHaveLength(1);
  });

  it("caps the description at 160 characters", () => {
    const [, description] = routeHead("Movies", "x".repeat(200)).meta;

    expect(description).toEqual({ name: "description", content: "x".repeat(160) });
  });
});

describe("movieHead", () => {
  it("titles a movie with its year and describes it with its overview", () => {
    expect(
      movieHead({ title: "Heat", year: 1995, overview: "A heist in Los Angeles." })
        .meta,
    ).toEqual([
      { title: "Heat (1995) - Igloo" },
      { name: "description", content: "A heist in Los Angeles." },
    ]);
  });

  it("leaves the year out when it is unknown and falls back to a stock description", () => {
    expect(movieHead({ title: "Heat", year: null, overview: null }).meta).toEqual([
      { title: "Heat - Igloo" },
      { name: "description", content: "Watch Heat in your Igloo media library." },
    ]);
  });

  it("titles a missing movie generically", () => {
    expect(movieHead(null).meta).toEqual([{ title: "Movie - Igloo" }]);
    expect(movieHead(undefined).meta).toEqual([{ title: "Movie - Igloo" }]);
  });
});

describe("showHead", () => {
  it("titles a show with its premiere year and describes it with its overview", () => {
    expect(
      showHead({ title: "Frost Harbor", year: 2024, overview: "A harbor freezes." })
        .meta,
    ).toEqual([
      { title: "Frost Harbor (2024) - Igloo" },
      { name: "description", content: "A harbor freezes." },
    ]);
  });

  it("falls back to a browse description and a generic title", () => {
    expect(
      showHead({ title: "Frost Harbor", year: null, overview: null }).meta,
    ).toEqual([
      { title: "Frost Harbor - Igloo" },
      {
        name: "description",
        content: "Browse Frost Harbor in your Igloo media library.",
      },
    ]);
    expect(showHead(null).meta).toEqual([{ title: "TV Show - Igloo" }]);
  });
});

describe("listenHead", () => {
  it("describes a music page with its summary, in the music library by default", () => {
    expect(listenHead("Blue Record", "1 track")).toEqual({
      meta: [
        { title: "Blue Record - Igloo" },
        {
          name: "description",
          content: "Listen to Blue Record - 1 track in your Igloo music library.",
        },
      ],
    });
  });

  it("names another place when given one", () => {
    expect(listenHead("Road Trip", "12 tracks", "playlist").meta[1]).toEqual({
      name: "description",
      content: "Listen to Road Trip - 12 tracks in your Igloo playlist.",
    });
  });
});
