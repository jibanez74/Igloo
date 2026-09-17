import { screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ShowDetailsHeroActions from "@/components/shows/ShowDetailsHeroActions";
import { jsonResponse } from "../helpers/api";
import { nullableFloat64 } from "../helpers/fixtures";
import { renderWithQueryClient } from "../helpers/render";
import { linkSearch } from "../helpers/router-link-mock";
import { SHOW_ID, episode, seasonEpisodes } from "../helpers/show-details";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

function stubSeason(episodes: ReturnType<typeof episode>[]) {
  const fetchMock = vi.fn(() =>
    jsonResponse({
      error: false,
      data: { season: seasonEpisodes(1).season, episodes },
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const playPath = (episodeId: number) =>
  `/tv-shows/${SHOW_ID}/episodes/${episodeId}/play`;

describe("ShowDetailsHeroActions", () => {
  it("plays the first episode of a fresh season", async () => {
    stubSeason([episode(1, 1), episode(1, 2)]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    const link = await screen.findByRole("link", {
      name: "Play S1 E1 S1 Episode 1",
    });
    expect(link).toHaveAttribute("href", playPath(70101));
    expect(linkSearch(link)).toEqual({ start: 0, audio_track: 0 });
  });

  it("resumes the first partly watched episode ahead of unwatched ones", async () => {
    stubSeason([
      episode(1, 1, { watched: true }),
      episode(1, 2),
      episode(1, 3, {
        progress_sec: nullableFloat64(600),
        duration_sec: nullableFloat64(2700),
      }),
    ]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    const link = await screen.findByRole("link", {
      name: "Resume S1 E3 S1 Episode 3",
    });
    expect(link).toHaveAttribute("href", playPath(70103));
  });

  it("skips watched episodes when nothing is in progress", async () => {
    stubSeason([episode(1, 1, { watched: true }), episode(1, 2)]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    expect(
      await screen.findByRole("link", { name: "Play S1 E2 S1 Episode 2" }),
    ).toHaveAttribute("href", playPath(70102));
  });

  it("falls back to the first episode of a fully watched season", async () => {
    stubSeason([episode(1, 1, { watched: true }), episode(1, 2, { watched: true })]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    expect(
      await screen.findByRole("link", { name: "Play S1 E1 S1 Episode 1" }),
    ).toHaveAttribute("href", playPath(70101));
  });

  it("renders nothing for a season with no episodes", async () => {
    const fetchMock = stubSeason([]);

    const { container, queryClient } = renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    // Nothing renders while the season is unknown either, so wait for the
    // query to settle before reading the empty state as the answer.
    await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
    await waitFor(() =>
      expect(queryClient.isFetching()).toBe(0),
    );
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
