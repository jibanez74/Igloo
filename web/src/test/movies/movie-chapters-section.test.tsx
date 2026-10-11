import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MovieChaptersSection from "@/components/movies/MovieChaptersSection";
import type { ChapterType } from "@/types/movies";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

function chapter(id: number, title: string, startTime: number): ChapterType {
  return {
    id,
    title,
    start_time: startTime,
    movie_id: 17,
  };
}

describe("MovieChaptersSection", () => {
  it("names an untitled chapter by its number, as the player's menu does", () => {
    render(
      <MovieChaptersSection
        movieId={17}
        playbackSettings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        chapters={[
          chapter(1, "Opening", 0),
          chapter(2, "   ", 300),
          chapter(3, "", 600),
        ]}
      />,
    );

    const links = screen.getAllByRole("link");
    expect(links.map(link => link.textContent)).toEqual([
      "Opening0:00",
      "Chapter 25:00",
      "Chapter 310:00",
    ]);
  });
});
