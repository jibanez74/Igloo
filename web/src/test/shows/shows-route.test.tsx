import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTENT_FADE_TRANSITION_MS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  SHOW_DETAILS_KEY,
  SHOWS_PER_PAGE,
  SHOWS_STATS_KEY,
} from "@/lib/constants";
import { countFetchRequests, jsonResponse, requestURL } from "../helpers/api";
import { runContentFadeTransitionTimeout } from "../helpers/content-fade-transition";
import {
  restoreMatchMedia,
  setReducedMotionPreference,
} from "../helpers/dom";
import { authUser, nullableInt64, nullableString } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: toastMocks.showActionFailed,
    showSuccess: toastMocks.showSuccess,
  };
});

function show(id: number, name: string, year: number) {
  return {
    id,
    name,
    poster_path: nullableString(),
    premiere_year: nullableInt64(year),
    certification: nullableString("TV-14"),
  };
}

function mockShowsFetch(options?: { statsRefreshFailure?: boolean }) {
  const libraryShows = [
    show(401, "Frost Harbor", 2026),
    show(402, "Halcyon Drift", 2024),
  ];
  let statsRequestCount = 0;

  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(
        authUser({ name: "Show User", email: "shows@example.com" }),
      );
    }

    if (url === "/api/shows/stats") {
      statsRequestCount += 1;

      if (options?.statsRefreshFailure && statsRequestCount > 1) {
        return jsonResponse({
          error: true,
          message: "Show stats are unavailable.",
        });
      }

      return jsonResponse({
        error: false,
        data: {
          total_shows: 3,
        },
      });
    }

    if (url === `/api/shows/library?page=1&per_page=${SHOWS_PER_PAGE}&sort=asc`) {
      return jsonResponse({
        error: false,
        data: {
          shows: libraryShows,
          total: libraryShows.length,
          page: 1,
          per_page: SHOWS_PER_PAGE,
          total_pages: 1,
          sort: "asc",
        },
      });
    }

    if (url === "/api/shows/genres") {
      return jsonResponse({
        error: false,
        data: {
          genres: [
            {
              genre_id: 10,
              genre_tag: "Drama",
              show_count: 2,
            },
            {
              genre_id: 20,
              genre_tag: "Comedy",
              show_count: 1,
            },
          ],
        },
      });
    }

    if (url === `/api/shows/genres/10/shows?page=1&per_page=${SHOWS_PER_PAGE}&sort=asc`) {
      return jsonResponse({
        error: false,
        data: {
          shows: [show(403, "Northern Lights", 2019)],
          total: 1,
          page: 1,
          per_page: SHOWS_PER_PAGE,
          total_pages: 1,
          sort: "asc",
        },
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

async function renderShowsRoute(
  initialEntry: string,
  options?: { statsRefreshFailure?: boolean },
) {
  const fetchMock = mockShowsFetch(options);

  return {
    fetchMock,
    ...(await renderRoute(initialEntry)),
  };
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  restoreMatchMedia();
});

describe("tv shows route cards", () => {
  it("links each show to its details page without a play control", async () => {
    await renderShowsRoute("/tv-shows/");

    const frostHarbor = await screen.findByRole("link", {
      name: "Frost Harbor 2026",
    });

    expect(frostHarbor).toHaveAttribute("href", "/tv-shows/401");
    expect(
      screen.getByRole("link", { name: "Halcyon Drift 2024" }),
    ).toHaveAttribute("href", "/tv-shows/402");
    expect(screen.queryByRole("link", { name: /^Play / })).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }),
    ).toBeInTheDocument();
  });
});

describe("tv shows route tab transitions", () => {
  it("delays swapping from all shows to genres until the fade-out completes", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderShowsRoute("/tv-shows/");

    expect(screen.getByText("Frost Harbor")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Genres" }));

    expect(screen.getByText("Frost Harbor")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Drama/i }),
    ).not.toBeInTheDocument();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Drama/i })).toBeInTheDocument();
    });
  }, 10_000);

  it("switches tabs without waiting when reduced motion is enabled", async () => {
    setReducedMotionPreference(true);
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderShowsRoute("/tv-shows/");

    expect(screen.getByText("Frost Harbor")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Genres" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Drama/i })).toBeInTheDocument();
    });
    expect(
      setTimeoutSpy.mock.calls.some(([, delay]) => delay === CONTENT_FADE_TRANSITION_MS),
    ).toBe(false);
  });
});

describe("tv shows route section motion", () => {
  it("applies section entrance contracts without changing tab panel fade behavior", async () => {
    await renderShowsRoute("/tv-shows/");

    const heading = await screen.findByRole("heading", {
      name: "TV Show Library",
    });
    const stats = screen.getByRole("region", {
      name: "Library statistics: 3 shows",
    });
    const tabsRoot = screen.getByRole("tablist").closest('[data-slot="tabs"]');

    expect(heading.closest("header")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );
    expect(stats.parentElement?.className).toContain(
      MOTION_SECTION_ENTER_DELAYED_CLASS,
    );
    expect(tabsRoot?.className).toContain(MOTION_SECTION_ENTER_DELAYED_CLASS);
    expect(
      screen.getByRole("tabpanel", { name: "All Shows" }).firstElementChild
        ?.className,
    ).toContain(MOTION_SECTION_ENTER_CLASS);
  });
});

describe("tv shows route library refresh", () => {
  it("refreshes active show library queries without starting a scan", async () => {
    const user = userEvent.setup();

    const { fetchMock, queryClient } = await renderShowsRoute("/tv-shows/");

    const statsUrl = "/api/shows/stats";
    const libraryUrl = `/api/shows/library?page=1&per_page=${SHOWS_PER_PAGE}&sort=asc`;
    const scanUrl = "/api/settings/scan/shows";
    const inactiveDetailsKey = [SHOW_DETAILS_KEY, 999] as const;

    queryClient.setQueryData(inactiveDetailsKey, {
      error: false,
      data: { show: { id: 999, name: "Cached detail" } },
    });

    const initialStatsCalls = countFetchRequests(fetchMock, statsUrl);
    const initialLibraryCalls = countFetchRequests(fetchMock, libraryUrl);

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Refresh Library" }),
    );

    await waitFor(() => {
      expect(countFetchRequests(fetchMock, statsUrl)).toBeGreaterThan(
        initialStatsCalls,
      );
      expect(countFetchRequests(fetchMock, libraryUrl)).toBeGreaterThan(
        initialLibraryCalls,
      );
    });

    expect(queryClient.getQueryData(inactiveDetailsKey)).toBeUndefined();
    expect(countFetchRequests(fetchMock, scanUrl)).toBe(0);
    expect(toastMocks.showSuccess).toHaveBeenCalledWith(
      "Library refreshed",
      "Show library data is up to date.",
    );
  });

  it("reports refresh failure when an active show query returns an API error envelope", async () => {
    const user = userEvent.setup();

    const { queryClient } = await renderShowsRoute("/tv-shows/", {
      statsRefreshFailure: true,
    });

    await screen.findByRole("region", {
      name: "Library statistics: 3 shows",
    });

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Refresh Library" }),
    );

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "refresh library",
        "Unable to refresh the show library. Please try again.",
      );
    });

    expect(toastMocks.showSuccess).not.toHaveBeenCalled();
    expect(queryClient.getQueryData([SHOWS_STATS_KEY])).toEqual({
      error: true,
      message: "Show stats are unavailable.",
    });
  });
});

describe("tv shows route genres", () => {
  it("filters by the selected genre and restores focus to its button after clearing", async () => {
    const user = userEvent.setup();

    await renderShowsRoute("/tv-shows/?tab=genres&genreId=10");

    expect(
      await screen.findByRole("link", { name: "Northern Lights 2019" }),
    ).toHaveAttribute("href", "/tv-shows/403");
    expect(screen.getByRole("list", { name: "TV show genres" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Drama/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: /Comedy/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    await user.click(screen.getByRole("button", { name: "Clear genre filter" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Drama/ })).toHaveFocus();
    });
    expect(
      screen.queryByRole("link", { name: "Northern Lights 2019" }),
    ).not.toBeInTheDocument();
  });
});
