import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ShowSeasonEpisodeList from "@/components/shows/ShowSeasonEpisodeList";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderWithQueryClient } from "../helpers/render";
import { SHOW_ID, seasonEpisodes } from "../helpers/show-details";

describe("ShowSeasonEpisodeList", () => {
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
