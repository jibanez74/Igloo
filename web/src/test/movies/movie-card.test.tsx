import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MovieCard from "@/components/movies/MovieCard";
import type { LatestMovieType } from "@/types";
import { nullableInt64, nullableString } from "../helpers/fixtures";
import { renderWithQueryClient } from "../helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

const movie: LatestMovieType = {
  id: 104,
  title: "Ember Line",
  poster_path: nullableString("/ember-line.jpg"),
  year: nullableInt64(2026),
};

describe("MovieCard", () => {
  it("offers a plain play when nothing is in progress", () => {
    renderWithQueryClient(<MovieCard movie={movie} />);

    expect(
      screen.getByRole("link", { name: "Play Ember Line 2026" }),
    ).toHaveAttribute("href", "/movies/104/play");
    expect(
      screen.getByRole("link", { name: "Ember Line 2026" }),
    ).toHaveAttribute("href", "/movies/104");
  });

  it("resumes and announces the progress when a position is saved", () => {
    renderWithQueryClient(
      <MovieCard
        movie={movie}
        watchProgress={{ progressSec: 1830, durationSec: 5400 }}
      />,
    );

    // Same wording as ContinueWatchingEpisodeCard, its neighbour on home.
    expect(
      screen.getByRole("link", { name: "Resume Ember Line 2026" }),
    ).toHaveAttribute("href", "/movies/104/play");
    // 1830 / 5400 rounds to 34%.
    expect(
      screen.getByRole("link", { name: "Ember Line 2026, 34% watched" }),
    ).toHaveAttribute("href", "/movies/104");
    expect(screen.queryByRole("link", { name: /^Play / })).not.toBeInTheDocument();
  });

  it("keeps a plain play when the duration is unknown", () => {
    renderWithQueryClient(
      <MovieCard
        movie={movie}
        watchProgress={{ progressSec: 1830, durationSec: 0 }}
      />,
    );

    expect(
      screen.getByRole("link", { name: "Play Ember Line 2026" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Ember Line 2026" }),
    ).toBeInTheDocument();
  });
});
