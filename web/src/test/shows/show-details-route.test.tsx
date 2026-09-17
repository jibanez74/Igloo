import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";
import {
  getDetailMotionWrappers,
  getHeroMotionWrapper,
  getLowerMotionWrapper,
} from "../helpers/motion";
import { SHOW_ID, seasonEpisodes, showDetails } from "../helpers/show-details";

type MockOptions = {
  detailsStatus?: number;
  detailsBody?: unknown;
};

function mockShowDetailsFetch({ detailsStatus, detailsBody }: MockOptions = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Show User",
            email: "shows@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url === `/api/shows/details/${SHOW_ID}`) {
      if (detailsBody !== undefined) {
        return jsonResponse(detailsBody, detailsStatus ?? 200);
      }
      return jsonResponse({ error: false, data: showDetails() });
    }

    const episodesMatch = url.match(
      /^\/api\/shows\/(\d+)\/seasons\/(\d+)\/episodes$/,
    );
    if (episodesMatch) {
      // Validate the show id too: a season fixture served for any id would
      // hide an episode URL built against the wrong show.
      if (Number.parseInt(episodesMatch[1], 10) !== SHOW_ID) {
        return jsonResponse({ error: true, message: "show not found" }, 404);
      }

      const seasonNumber = Number.parseInt(episodesMatch[2], 10);
      return jsonResponse({
        error: false,
        data: seasonEpisodes(seasonNumber),
      });
    }

    // Anything the page asks for that is not stubbed fails loudly rather than
    // hanging the test.
    return jsonResponse(
      { error: true, message: `Unexpected request: ${url}` },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("show details route", () => {
  it("defaults to the first season in the returned order, not to specials", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    // Specials are season zero but sort last, so Season 1 is the default.
    const season1Tab = screen.getByRole("tab", { name: "Season 1" });
    expect(season1Tab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Specials" })).toHaveAttribute(
      "aria-selected",
      "false",
    );

    const episodes = screen.getByRole("list", {
      name: /Season 1 episodes, 2 in this library/,
    });
    expect(within(episodes).getAllByRole("listitem")).toHaveLength(2);
  });

  it("honors the season search parameter", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}?season=0`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Specials" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to the default season when the URL names one the show lacks", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}?season=47`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Season 1" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("switches seasons and writes the choice to the URL without stacking history", async () => {
    const user = userEvent.setup();
    mockShowDetailsFetch();

    const { router } = await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    // renderRoute builds the router on createMemoryHistory, so window.history
    // never moves; the router's own history is what records push vs replace.
    const historyDepthBefore = router.history.length;

    await user.click(screen.getByRole("tab", { name: "Season 2" }));

    expect(
      await screen.findByRole("list", {
        name: /Season 2 episodes, 1 in this library/,
      }),
    ).toBeInTheDocument();

    expect(router.state.location.search).toEqual({ season: 2 });
    expect(router.history.length).toBe(historyDepthBefore);
  });

  it("carries the selected season into the trailer return link", async () => {
    const user = userEvent.setup();
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Season 2" }));

    expect(
      await screen.findByRole("list", {
        name: /Season 2 episodes, 1 in this library/,
      }),
    ).toBeInTheDocument();

    // Without the season the trailer would return the viewer to season one.
    expect(
      screen.getByRole("link", { name: /Frost Harbor Trailer/ }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining(
        `returnTo=${encodeURIComponent(`/tv-shows/${SHOW_ID}?season=2`)}`,
      ),
    );
  });

  it("renders the three-stage entrance stagger", async () => {
    mockShowDetailsFetch();

    const { container } = await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    const wrappers = getDetailMotionWrappers(container);
    const heroWrapper = getHeroMotionWrapper(container);
    const lowerWrapper = getLowerMotionWrapper(container);
    const backdropWrapper = wrappers.find(
      element =>
        element !== heroWrapper &&
        element !== lowerWrapper &&
        element.className.startsWith(DETAIL_PAGE_CONTENT_ENTER_CLASS) &&
        !element.className.includes("delay-"),
    );

    expect(wrappers).toHaveLength(3);
    expect(backdropWrapper).toBeDefined();
    expect(heroWrapper?.className).toContain("delay-75 motion-reduce:delay-0");
    expect(lowerWrapper?.className).toContain("delay-150 motion-reduce:delay-0");
  });

  it("excludes specials from the season and episode tallies", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    // The fixture has three local seasons (1, 2, specials) and 4 local
    // episodes, against a TMDB count of 2 seasons and 10 episodes. TMDB counts
    // the numbered run only, so the page must too, or it would read "3 of 2".
    expect(screen.getByText("2 seasons in this library")).toBeInTheDocument();
    expect(
      screen.getByText("3 of 10 episodes available in this library"),
    ).toBeInTheDocument();
  });

  it("renders the not-found state with a link back to the TV library", async () => {
    mockShowDetailsFetch({
      detailsBody: { error: true, message: "show not found" },
      detailsStatus: 404,
    });

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    // apiRequest turns any 404 into its canned not-found envelope, so the page
    // shows that message rather than the server's.
    expect(
      await screen.findByText(/The resource you requested was not found/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Back to TV Shows/i }),
    ).toHaveAttribute("href", "/tv-shows");
  });

  it("wires each season tab to the episode list as its tabpanel", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    const season1Tab = await screen.findByRole("tab", { name: "Season 1" });
    const panel = screen.getByRole("tabpanel");

    // A tab must control a panel that exists, or screen readers announce a
    // tab with nowhere to go.
    expect(season1Tab).toHaveAttribute("aria-controls", panel.id);
    expect(
      within(panel).getByRole("list", { name: /Season 1 episodes/ }),
    ).toBeInTheDocument();
  });

  it("announces the episodes of the season that was just selected", async () => {
    const user = userEvent.setup();
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Season 2" }));

    // The tab change itself says nothing; the list announces its resolution.
    expect(
      await screen.findByText("Season 2: 1 episode in this library", {
        selector: "[role='status']",
      }),
    ).toBeInTheDocument();
  });

  it("keys duplicate roles for one artist without a React key collision", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();

    // Both roles of the same artist render, and React logs no duplicate-key
    // warning, because the rows are keyed on credit_id.
    expect(
      screen.getByRole("article", { name: /Ada Contract as Captain, 10 episodes/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("article", {
        name: /Ada Contract as Captain's Double, 2 episodes/,
      }),
    ).toBeInTheDocument();

    const duplicateKeyWarnings = errorSpy.mock.calls.filter(call =>
      String(call[0]).includes("same key"),
    );
    expect(duplicateKeyWarnings).toEqual([]);

    errorSpy.mockRestore();
  });
});
