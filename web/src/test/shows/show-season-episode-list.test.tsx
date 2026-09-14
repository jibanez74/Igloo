import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ShowSeasonEpisodeList from "@/components/shows/ShowSeasonEpisodeList";
import {
  countFetchRequests,
  deferredResponse,
  jsonResponse,
  requestURL,
} from "../helpers/api";
import { nullableFloat64 } from "../helpers/fixtures";
import { renderWithQueryClient } from "../helpers/render";
import { linkSearch } from "../helpers/router-link-mock";
import { SHOW_ID, episode, seasonEpisodes } from "../helpers/show-details";

const showActionFailedMock = vi.fn();

// The rows link into the episode player; the component is rendered without a
// router here, so Link becomes a plain anchor that keeps the resolved href.
vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  };
});

function seasonWithProgress() {
  const season = seasonEpisodes(1);
  return {
    ...season,
    episodes: [
      episode(1, 1, {
        progress_sec: nullableFloat64(1200),
        duration_sec: nullableFloat64(2700),
      }),
      episode(1, 2, { watched: true }),
    ],
  };
}

describe("ShowSeasonEpisodeList", () => {
  it("links every row to its episode player", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => jsonResponse({ error: false, data: seasonEpisodes(1) })),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const play = await screen.findByRole("link", {
      name: "Play S1 E2 S1 Episode 2",
    });
    expect(play).toHaveAttribute(
      "href",
      `/tv-shows/${SHOW_ID}/episodes/70102/play`,
    );
    expect(linkSearch(play)).toEqual({ start: 0, audio_track: 0 });
  });

  it("shows resume progress and watched state from the season payload", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => jsonResponse({ error: false, data: seasonWithProgress() })),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const inProgress = await screen.findByRole("article", {
      name: /1\.\s*S1 Episode 1/,
    });
    expect(within(inProgress).getByText("25 min left")).toBeInTheDocument();
    // The abbreviated note is aria-hidden; the sr-only twin speaks it in full.
    expect(
      within(inProgress).getByText("25 minutes left"),
    ).toHaveClass("sr-only");
    expect(
      within(inProgress).getByRole("link", { name: /^Resume S1 E1/ }),
    ).toBeInTheDocument();
    expect(
      within(inProgress).getByRole("button", { name: "Mark S1 E1 as watched" }),
    ).toHaveAttribute("aria-pressed", "false");

    const watched = screen.getByRole("article", { name: /2\.\s*S1 Episode 2/ });
    expect(within(watched).getByText("Watched")).toBeInTheDocument();
    expect(within(watched).queryByText(/min left/)).not.toBeInTheDocument();
    expect(
      within(watched).getByRole("button", { name: "Mark S1 E2 as unwatched" }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("marks an episode watched optimistically and keeps the server's answer", async () => {
    const user = userEvent.setup();
    const watchedRequest = deferredResponse();
    // The server owns the state: once marked, the season refetch that the
    // toggle triggers must return the watched row too.
    let markedWatched = false;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestURL(input);
      if (url === "/api/shows/episodes/70101/watch-progress/watched") {
        expect(init?.method).toBe("PUT");
        expect(JSON.parse(String(init?.body))).toEqual({ watched: true });
        return watchedRequest.promise;
      }
      const season = seasonWithProgress();
      return jsonResponse({
        error: false,
        data: markedWatched
          ? {
              ...season,
              episodes: [
                episode(1, 1, { watched: true }),
                episode(1, 2, { watched: true }),
              ],
            }
          : season,
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const row = await screen.findByRole("article", { name: /1\.\s*S1 Episode 1/ });
    expect(within(row).getByText("25 min left")).toBeInTheDocument();
    await user.click(
      within(row).getByRole("button", { name: "Mark S1 E1 as watched" }),
    );

    // The row flips before the server has answered: the request is still
    // pending, and the saved position goes with it, as the server would.
    await waitFor(() => {
      expect(
        within(row).getByRole("button", { name: "Mark S1 E1 as unwatched" }),
      ).toHaveAttribute("aria-pressed", "true");
    });
    expect(within(row).getByText("Watched")).toBeInTheDocument();
    expect(within(row).queryByText("25 min left")).not.toBeInTheDocument();
    expect(
      countFetchRequests(
        fetchMock,
        "/api/shows/episodes/70101/watch-progress/watched",
      ),
    ).toBe(1);

    markedWatched = true;
    watchedRequest.resolve(
      jsonResponse({ error: false, data: { episode_id: 70101, watched: true } }),
    );
    await waitFor(() =>
      expect(
        countFetchRequests(fetchMock, `/api/shows/${SHOW_ID}/seasons/1/episodes`),
      ).toBeGreaterThan(1),
    );
    expect(
      within(row).getByRole("button", { name: "Mark S1 E1 as unwatched" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  it("rolls the row back and reports the failure when marking watched fails", async () => {
    const user = userEvent.setup();
    const watchedRequest = deferredResponse();
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = requestURL(input);
      if (url === "/api/shows/episodes/70101/watch-progress/watched") {
        return watchedRequest.promise;
      }
      return jsonResponse({ error: false, data: seasonWithProgress() });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const row = await screen.findByRole("article", { name: /1\.\s*S1 Episode 1/ });
    await user.click(
      within(row).getByRole("button", { name: "Mark S1 E1 as watched" }),
    );
    await waitFor(() => {
      expect(
        within(row).getByRole("button", { name: "Mark S1 E1 as unwatched" }),
      ).toHaveAttribute("aria-pressed", "true");
    });
    expect(within(row).queryByText("25 min left")).not.toBeInTheDocument();

    watchedRequest.resolve(jsonResponse({ error: true, message: "nope" }, 500));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update watched status",
        "nope",
      );
    });
    expect(
      within(row).getByRole("button", { name: "Mark S1 E1 as watched" }),
    ).toHaveAttribute("aria-pressed", "false");
    expect(within(row).getByText("25 min left")).toBeInTheDocument();
    expect(within(row).queryByText("Watched")).not.toBeInTheDocument();
  });

  it("renders episode rows with number, runtime, air date, and overview", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => jsonResponse({ error: false, data: seasonEpisodes(1) })),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const list = await screen.findByRole("list", {
      name: /Season 1 episodes, 2 in this library/,
    });
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(2);

    expect(
      within(rows[0]).getByRole("heading", { name: /1\.\s*S1 Episode 1/ }),
    ).toBeInTheDocument();
    expect(within(rows[0]).getByText("47 min")).toBeInTheDocument();
    expect(within(rows[0]).getByText("An episode happens.")).toBeInTheDocument();
    // formatDate builds a date-only value as a local date, so the visible day
    // is stable whatever timezone the runner sits in.
    expect(
      within(rows[0]).getByText("March 8, 2024", { selector: "time" }),
    ).toHaveAttribute("datetime", "2024-03-08");
  });

  it("names the season in its empty state", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        jsonResponse({
          error: false,
          data: { season: seasonEpisodes(1).season, episodes: [] },
        }),
      ),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    expect(await screen.findByText("No episodes yet")).toBeInTheDocument();
    expect(
      screen.getByText(/No episodes of Season 1 are in this library/),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("Season 1: no episodes in this library", {
        selector: "[role='status']",
      }),
    ).toBeInTheDocument();
  });

  it("names each episode row after its heading", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => jsonResponse({ error: false, data: seasonEpisodes(1) })),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    const row = await screen.findByRole("article", {
      name: /1\.\s*S1 Episode 1/,
    });
    // Episode titles sit one level under the season heading (h3).
    expect(
      within(row).getByRole("heading", { level: 4 }),
    ).toHaveTextContent(/S1 Episode 1/);
  });

  it("calls specials by name rather than by number", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        jsonResponse({
          error: false,
          data: { season: seasonEpisodes(0).season, episodes: [] },
        }),
      ),
    );

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={0} />,
    );

    expect(
      await screen.findByText(/No episodes of Specials are in this library/),
    ).toBeInTheDocument();
  });

  it("offers a retry that refetches after a load failure", async () => {
    const user = userEvent.setup();
    let attempt = 0;
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      expect(requestURL(input)).toBe(
        `/api/shows/${SHOW_ID}/seasons/1/episodes`,
      );
      attempt += 1;
      if (attempt === 1) {
        return jsonResponse({ error: true, message: "server exploded" }, 500);
      }
      return jsonResponse({ error: false, data: seasonEpisodes(1) });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderWithQueryClient(
      <ShowSeasonEpisodeList showId={SHOW_ID} seasonNumber={1} />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "server exploded",
    );

    await user.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() => {
      expect(
        screen.getByRole("list", { name: /Season 1 episodes/ }),
      ).toBeInTheDocument();
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
