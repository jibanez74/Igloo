import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ContinueWatchingEpisodeCard from "@/components/shows/ContinueWatchingEpisodeCard";
import type { ContinueWatchingEpisodeItemType } from "@/types";
import { nullableInt64, nullableString } from "../helpers/fixtures";
import { renderWithQueryClient } from "../helpers/render";
import { linkSearch } from "../helpers/router-link-mock";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

function episodeItem(
  overrides: Partial<ContinueWatchingEpisodeItemType> = {},
): ContinueWatchingEpisodeItemType {
  return {
    kind: "episode",
    id: 70103,
    title: "Frost Harbor",
    poster_path: nullableString("/frost-harbor.jpg"),
    year: nullableInt64(2025),
    progress_sec: 600,
    duration_sec: 2400,
    show_id: 301,
    season_number: 1,
    episode_number: 4,
    episode_name: "Thin Ice",
    ...overrides,
  };
}

describe("ContinueWatchingEpisodeCard", () => {
  it("names the show and the episode and announces the progress", () => {
    renderWithQueryClient(<ContinueWatchingEpisodeCard episode={episodeItem()} />);

    // 600 / 2400 is 25%.
    const details = screen.getByRole("link", {
      name: "Frost Harbor, S1 E4 · Thin Ice, 25% watched",
    });
    expect(details).toHaveAttribute("href", "/tv-shows/301");
    expect(screen.getByRole("heading", { name: "Frost Harbor" })).toBeInTheDocument();
    expect(screen.getByText("S1 E4 · Thin Ice")).toBeInTheDocument();
  });

  it("plays the episode and lets the play route resolve where it starts", () => {
    renderWithQueryClient(<ContinueWatchingEpisodeCard episode={episodeItem()} />);

    const play = screen.getByRole("link", {
      name: "Resume Frost Harbor S1 E4 · Thin Ice",
    });
    expect(play).toHaveAttribute("href", "/tv-shows/301/episodes/70103/play");
    // No search params: playSearchSchema defaults start and audio_track to 0,
    // and the movie card's play link is bare for the same reason.
    expect(linkSearch(play)).toBeNull();
  });

  it("falls back to the show icon when the show has no poster", () => {
    renderWithQueryClient(
      <ContinueWatchingEpisodeCard
        episode={episodeItem({ poster_path: nullableString() })}
      />,
    );

    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
