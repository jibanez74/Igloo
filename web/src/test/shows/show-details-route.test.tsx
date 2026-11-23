import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import { showDetailsQueryOpts } from "@/lib/query-opts";
import {
  countFetchRequests,
  deferredResponse,
  jsonResponse,
  requestURL,
  stubApi,
} from "../helpers/api";
import { createTestQueryClient } from "../helpers/render";
import { readDocumentHead, renderRoute } from "../helpers/render-route";
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
  // Only this show's seasons are served, so an episode URL built against the
  // wrong show fails the test.
  const seasons = Object.fromEntries(
    showDetails().seasons.map(({ season_number }) => [
      `/api/shows/${SHOW_ID}/seasons/${season_number}/episodes`,
      seasonEpisodes(season_number),
    ]),
  );

  return stubApi(
    {
      [`/api/shows/details/${SHOW_ID}`]: () =>
        detailsBody === undefined
          ? showDetails()
          : jsonResponse(detailsBody, detailsStatus ?? 200),
      ...seasons,
      // A season the show lacks: the loader asks for the URL's season beside
      // the details, before it can know the show has none.
      [`/api/shows/${SHOW_ID}/seasons/47/episodes`]: seasonEpisodes(47),
    },
    { user: { name: "Show User", email: "shows@example.com" } },
  );
}

describe("show details route", () => {
  it("renders fresh prefetched details without requesting them again", async () => {
    const fetchMock = mockShowDetailsFetch();
    const queryClient = createTestQueryClient();
    const detailsUrl = `/api/shows/details/${SHOW_ID}`;
    const episodesUrl = `/api/shows/${SHOW_ID}/seasons/1/episodes`;

    await queryClient.prefetchQuery(showDetailsQueryOpts(SHOW_ID));
    expect(countFetchRequests(fetchMock, detailsUrl)).toBe(1);
    expect(countFetchRequests(fetchMock, episodesUrl)).toBe(0);

    await renderRoute(`/tv-shows/${SHOW_ID}`, { queryClient });

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("list", {
        name: /Season 1 episodes, 2 in this library/,
      }),
    ).toBeInTheDocument();
    expect(countFetchRequests(fetchMock, detailsUrl)).toBe(1);
    expect(countFetchRequests(fetchMock, episodesUrl)).toBe(1);
  });

  it("titles the page with the show's name, premiere year and overview", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    await waitFor(() => {
      expect(readDocumentHead()).toMatchObject({
        title: "Frost Harbor (2024) - Igloo",
        description: "A harbor freezes and a town changes with it.",
        descriptionCount: 1,
      });
    });
  });

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
    expect(
      await screen.findByRole("list", {
        name: /Specials episodes, 1 in this library/,
      }),
    ).toBeInTheDocument();
  });

  it("asks for the season the URL names beside the details, not after them", async () => {
    const fetchMock = mockShowDetailsFetch();
    const details = deferredResponse();
    const serve = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation(input =>
      requestURL(input) === `/api/shows/details/${SHOW_ID}`
        ? details.promise
        : serve(input),
    );
    const episodesUrl = `/api/shows/${SHOW_ID}/seasons/2/episodes`;

    const view = renderRoute(`/tv-shows/${SHOW_ID}?season=2`);
    await waitFor(() => {
      expect(countFetchRequests(fetchMock, episodesUrl)).toBe(1);
    });
    details.resolve(jsonResponse({ error: false, data: showDetails() }));
    await view;

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();
    expect(countFetchRequests(fetchMock, episodesUrl)).toBe(1);
  });

  it("falls back to the default season when the URL names one the show lacks, and rewrites the URL", async () => {
    mockShowDetailsFetch();

    const { router } = await renderRoute(`/tv-shows/${SHOW_ID}?season=47`);

    expect(
      await screen.findByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Season 1" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // The address bar follows the tab that is actually selected, and the
    // rewrite replaces the bad entry so Back does not return to it.
    expect(router.state.location.search).toEqual({ season: 1 });
    expect(router.history.length).toBe(1);
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

  // Specials are season zero, so a falsy check would drop them from the URL.
  it("writes the specials season to the URL as zero", async () => {
    const user = userEvent.setup();
    mockShowDetailsFetch();

    const { router } = await renderRoute(`/tv-shows/${SHOW_ID}`);

    await user.click(await screen.findByRole("tab", { name: "Specials" }));

    expect(
      await screen.findByRole("list", {
        name: /Specials episodes, 1 in this library/,
      }),
    ).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ season: 0 });
  });

  it("names the show's networks in its About section", async () => {
    mockShowDetailsFetch();

    await renderRoute(`/tv-shows/${SHOW_ID}`);

    const aboutHeading = await screen.findByRole("heading", {
      name: "About Frost Harbor",
    });
    const about = aboutHeading.closest("section");
    expect(about).not.toBeNull();
    expect(within(about as HTMLElement).getByText("Contract Network")).toBeInTheDocument();
    // The mark sits on a light plate in the dark theme (design-system §3.3).
    const logo = (about as HTMLElement).querySelector('img[src$="/network.png"]');
    expect(logo).not.toBeNull();
    expect(logo?.parentElement).toHaveClass("dark:bg-white/90", "rounded-sm");
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

    // apiRequest turns any 404 into its canned not-found envelope; the guard
    // words it for the reader instead of showing that string.
    expect(
      await screen.findByText("We couldn't find that show."),
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
      screen.getByRole("link", { name: /Ada Contract as Captain, 10 episodes/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
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

describe("show person links", () => {
  it("links cast, creators and expanded crew with season return state", async () => {
    mockShowDetailsFetch();
    await renderRoute(`/tv-shows/${SHOW_ID}?season=2`);
    const actor = await screen.findByRole("link", { name: /Ada Contract as Captain, 10 episodes/ });
    const expected = `/people/9001?returnTo=${encodeURIComponent(`/tv-shows/${SHOW_ID}?season=2`)}`;
    expect(actor).toHaveAttribute("href", expected);
    expect(screen.getByRole("link", { name: "Ada Contract" })).toHaveAttribute("href", expected);
    await userEvent.click(screen.getByRole("button", { name: "Show all crew" }));
    expect(screen.getByRole("link", { name: "Bo Contract" })).toHaveAttribute("href", expected);
  });
});
