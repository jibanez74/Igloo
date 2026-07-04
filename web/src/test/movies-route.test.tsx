import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTENT_FADE_TRANSITION_MS,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MOVIES_PER_PAGE,
  MOVIES_STATS_KEY,
} from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";
import { runContentFadeTransitionTimeout } from "./content-fade-transition";

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

const defaultMatchMedia = window.matchMedia;

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

function nullableString(value = "") {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function movie(id: number, title: string, year: number) {
  return {
    id,
    title,
    poster_path: nullableString(),
    year: nullableInt64(year),
  };
}

function playlist(id: number, name: string, movieCount: number) {
  return {
    id,
    user_id: 1,
    name,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    folder_id: {
      Int64: 0,
      Valid: false,
    },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    movie_count: movieCount,
    total_duration: 0,
    is_owner: true,
    can_edit: true,
  };
}

function mockMoviesFetch(options?: {
  statsRefreshFailure?: boolean;
  tmdbAvailable?: boolean;
}) {
  const libraryMovies = [movie(1, "Arrival", 2016), movie(2, "Heat", 1995)];
  const likedMovies = [movie(3, "Moonlight", 2016)];
  const playlists = [playlist(11, "Weekend Picks", 2)];
  const tmdbAvailable = options?.tmdbAvailable ?? true;
  let statsRequestCount = 0;

  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Movie User",
            email: "movies@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url === "/api/movies/stats") {
      statsRequestCount += 1;

      if (options?.statsRefreshFailure && statsRequestCount > 1) {
        return jsonResponse({
          error: true,
          message: "Movie stats are unavailable.",
        });
      }

      return jsonResponse({
        error: false,
        data: {
          total_movies: 42,
        },
      });
    }

    if (url === `/api/movies/library?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`) {
      return jsonResponse({
        error: false,
        data: {
          movies: libraryMovies,
          total: libraryMovies.length,
          page: 1,
          per_page: MOVIES_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/movies/genres") {
      return jsonResponse({
        error: false,
        data: {
          genres: [
            {
              genre_id: 10,
              genre_tag: "Action",
              movie_count: 2,
            },
            {
              genre_id: 20,
              genre_tag: "Drama",
              movie_count: 1,
            },
          ],
        },
      });
    }

    if (url === `/api/movies/genres/10/movies?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`) {
      return jsonResponse({
        error: false,
        data: {
          movies: [movie(4, "Die Hard", 1988)],
          total: 1,
          page: 1,
          per_page: MOVIES_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/movies/playlists") {
      return jsonResponse({
        error: false,
        data: {
          playlists,
        },
      });
    }

    if (url === `/api/movies/liked?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`) {
      return jsonResponse({
        error: false,
        data: {
          movies: likedMovies,
          total: likedMovies.length,
          page: 1,
          per_page: MOVIES_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/tmdb/status") {
      return jsonResponse({
        error: false,
        data: {
          available: tmdbAvailable,
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

function countFetchRequests(
  fetchMock: ReturnType<typeof mockMoviesFetch>,
  url: string,
) {
  return fetchMock.mock.calls.filter(([input]) => requestURL(input) === url)
    .length;
}

function createMoviesQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: false,
      },
      queries: {
        retry: false,
      },
    },
  });
}

async function renderMoviesRoute(
  initialEntry: string,
  options?: {
    statsRefreshFailure?: boolean;
    tmdbAvailable?: boolean;
  },
) {
  vi.stubGlobal("scrollTo", vi.fn());
  const fetchMock = mockMoviesFetch(options);

  const queryClient = createMoviesQueryClient();
  const history = createMemoryHistory({
    initialEntries: [initialEntry],
  });
  const router = createRouter({
    routeTree,
    context: {
      queryClient,
    },
    history,
  });

  await act(async () => {
    await router.load();
  });

  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ queryClient }} />
    </QueryClientProvider>,
  );

  return {
    fetchMock,
    queryClient,
    router,
    ...view,
  };
}

function setReducedMotionPreference(prefersReducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches:
        query === "(prefers-reduced-motion: reduce)" && prefersReducedMotion,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: defaultMatchMedia,
  });
});

describe("movies route tab transitions", () => {
  it("delays swapping from all movies to genres until the fade-out completes", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMoviesRoute("/movies/");

    expect(screen.getByText("Arrival")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Genres" }));

    expect(screen.getByText("Arrival")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Action/i }),
    ).not.toBeInTheDocument();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Action/i })).toBeInTheDocument();
    });
  }, 10_000);

  it("delays swapping from genres to playlists until the fade-out completes", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMoviesRoute("/movies/?tab=genres");

    expect(screen.getByRole("button", { name: /Action/i })).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Playlists" }));

    expect(screen.getByRole("button", { name: /Action/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Liked movies" }),
    ).not.toBeInTheDocument();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Liked movies" })).toBeInTheDocument();
    });
  }, 10_000);

  it("switches tabs without waiting when reduced motion is enabled", async () => {
    setReducedMotionPreference(true);
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMoviesRoute("/movies/");

    expect(screen.getByText("Arrival")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Genres" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Action/i })).toBeInTheDocument();
    });
    expect(
      setTimeoutSpy.mock.calls.some(([, delay]) => delay === CONTENT_FADE_TRANSITION_MS),
    ).toBe(false);
  });
});

describe("movies route section motion", () => {
  it("applies section entrance contracts without changing tab panel fade behavior", async () => {
    await renderMoviesRoute("/movies/");

    const heading = await screen.findByRole("heading", {
      name: "Movie Library",
    });
    const stats = screen.getByRole("region", {
      name: "Library statistics: 42 movies",
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
      screen.getByRole("tabpanel", { name: "All Movies" }).firstElementChild
        ?.className,
    ).toContain(MOTION_SECTION_ENTER_CLASS);
  });
});

describe("movies route library refresh", () => {
  it("refreshes active movie library queries without starting a scan", async () => {
    const user = userEvent.setup();

    const { fetchMock, queryClient } = await renderMoviesRoute("/movies/");

    const statsUrl = "/api/movies/stats";
    const libraryUrl = `/api/movies/library?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`;
    const tmdbStatusUrl = "/api/tmdb/status";
    const scanUrl = "/api/settings/scan/movies";
    const inactiveDetailsKey = [LIBRARY_MOVIE_DETAILS_KEY, 999] as const;

    queryClient.setQueryData(inactiveDetailsKey, {
      error: false,
      data: { movie: { id: 999, title: "Cached detail" } },
    });

    const initialStatsCalls = countFetchRequests(fetchMock, statsUrl);
    const initialLibraryCalls = countFetchRequests(fetchMock, libraryUrl);
    const initialTmdbStatusCalls = countFetchRequests(fetchMock, tmdbStatusUrl);

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
    expect(countFetchRequests(fetchMock, tmdbStatusUrl)).toBe(
      initialTmdbStatusCalls,
    );
    expect(countFetchRequests(fetchMock, scanUrl)).toBe(0);
  });

  it("reports refresh failure when an active movie query returns an API error envelope", async () => {
    const user = userEvent.setup();

    const { queryClient } = await renderMoviesRoute("/movies/", {
      statsRefreshFailure: true,
    });

    await screen.findByRole("region", {
      name: "Library statistics: 42 movies",
    });

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Refresh Library" }),
    );

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "refresh library",
        "Unable to refresh the movie library. Please try again.",
      );
    });

    expect(toastMocks.showSuccess).not.toHaveBeenCalled();
    expect(queryClient.getQueryData([MOVIES_STATS_KEY])).toEqual({
      error: true,
      message: "Movie stats are unavailable.",
    });
  });
});

describe("movies route focus restoration", () => {
  it("restores focus to the selected genre button after clearing the filter", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=genres&genreId=10");

    await screen.findByRole("button", { name: "Clear genre filter" });
    await user.click(screen.getByRole("button", { name: "Clear genre filter" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Action/i })).toHaveFocus();
    });
  });

  it("moves focus to Back to playlists when liked movies is opened from the playlists toolbar", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=playlists");

    await user.click(screen.getByRole("button", { name: "Liked movies" }));

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Back to playlists" }),
      ).toHaveFocus();
    });
  });

  it("restores focus to the playlists toolbar liked movies button after returning", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=playlists&view=liked");

    await user.click(
      await screen.findByRole("button", { name: "Back to playlists" }),
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Liked movies" })).toHaveFocus();
    });
  });

  it("does not override dropdown focus behavior when liked movies is opened from More options", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMoviesRoute("/movies/");

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Liked movies" }),
    );

    expect(
      screen.queryByRole("button", { name: "Back to playlists" }),
    ).not.toBeInTheDocument();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Back to playlists" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  }, 10_000);

  it("disables Request Movie when TMDB search is unavailable", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/", { tmdbAvailable: false });

    await user.click(screen.getByRole("button", { name: "More options" }));

    const requestMovieItem = await screen.findByRole("menuitem", {
      name: /Request Movie unavailable/i,
    });

    expect(requestMovieItem).toHaveAttribute("data-disabled");
    expect(requestMovieItem).toHaveAttribute(
      "title",
      "TMDB search is unavailable on this server.",
    );
  });
});
