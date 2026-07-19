import type React from "react";
import { act, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
} from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";

vi.mock("@/components/app/AppShell", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <main id="main">{children}</main>
  ),
}));

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

function authUser() {
  return {
    id: 1,
    name: "Admin User",
    email: "admin@example.com",
    is_admin: true,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

type MockHomeFetchOptions = {
  continueWatching?: unknown[];
};

const defaultContinueWatchingMovies = [
  {
    id: 104,
    title: "Ember Line",
    poster_path: { String: "", Valid: false },
    year: { Int64: 2026, Valid: true },
    progress_sec: 1830,
    duration_sec: 5400,
  },
];

function mockHomeFetch(options: MockHomeFetchOptions = {}) {
  const continueWatching =
    options.continueWatching ?? defaultContinueWatchingMovies;
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: { user: authUser() },
      });
    }

    if (url === "/api/watch-rooms") {
      return jsonResponse({
        error: false,
        data: {
          rooms: [
            {
              id: 7,
              movie_id: 101,
              movie_title: "Signal Fire",
              movie_poster: null,
              owner: {
                id: 1,
                name: "Admin User",
                avatar: null,
              },
              members: [
                {
                  id: 1,
                  name: "Admin User",
                  avatar: null,
                },
              ],
              playback_mode: "direct",
              is_owner: false,
              created_at: "2026-01-01T00:00:00Z",
            },
          ],
        },
      });
    }

    if (url === "/api/movies/continue-watching") {
      return jsonResponse({
        error: false,
        data: { movies: continueWatching },
      });
    }

    if (url === "/api/movies/latest") {
      return jsonResponse({
        error: false,
        data: {
          movies: [
            {
              id: 101,
              title: "Signal Fire",
              poster_path: { String: "", Valid: false },
              year: { Int64: 2026, Valid: true },
            },
          ],
        },
      });
    }

    if (url === "/api/music/albums/latest") {
      return jsonResponse({
        error: false,
        data: { albums: [] },
      });
    }

    if (url === "/api/tmdb/movies/in-theaters") {
      return jsonResponse({
        error: false,
        data: { movies: [] },
      });
    }

    return jsonResponse({
      error: true,
      message: `Unexpected request: ${method} ${url}`,
    }, 500);
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function createHomeQueryClient() {
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

async function renderHomeRoute(options: MockHomeFetchOptions = {}) {
  vi.stubGlobal("scrollTo", vi.fn());
  mockHomeFetch(options);
  const queryClient = createHomeQueryClient();
  const history = createMemoryHistory({
    initialEntries: ["/"],
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
    queryClient,
    router,
    ...view,
  };
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("home route motion", () => {
  it("renders the home route with section-level motion contracts", async () => {
    await renderHomeRoute();

    const dashboardHeading = await screen.findByRole("heading", {
      name: "Welcome to Igloo",
    });
    expect(dashboardHeading).toBeInTheDocument();

    expect(dashboardHeading.closest("section")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );

    for (const regionName of [
      "Watch Rooms",
      "Continue Watching",
      "Recently Added Movies",
      "Recently Added Albums",
      "Now Playing in Theaters",
    ]) {
      expect(screen.getByRole("region", { name: regionName }).className).toContain(
        MOTION_SECTION_ENTER_DELAYED_CLASS,
      );
    }

    expect(MOTION_SECTION_ENTER_CLASS).toContain("motion-reduce:animate-none");
    expect(MOTION_SECTION_ENTER_CLASS).toContain("motion-reduce:opacity-100");
    expect(MOTION_SECTION_ENTER_DELAYED_CLASS).toContain(
      "motion-reduce:delay-0",
    );
  });
});

describe("home section summaries", () => {
  it("pluralizes item counts in the summary and count badge", async () => {
    await renderHomeRoute();

    expect(
      await screen.findByText("1 movie available in recently added movies."),
    ).toBeInTheDocument();
    // Count badges for the single-item sections render the singular noun.
    expect(screen.getAllByText("1 movie").length).toBeGreaterThan(0);
    expect(screen.queryByText(/1 movies/)).not.toBeInTheDocument();
  });
});

describe("home continue watching section", () => {
  it("announces watch progress on the card link", async () => {
    await renderHomeRoute();

    const watchingRegion = await screen.findByRole("region", {
      name: "Continue Watching",
    });
    expect(watchingRegion).toBeInTheDocument();

    // 1830 / 5400 rounds to 34%.
    expect(
      screen.getByRole("link", { name: "Ember Line 2026, 34% watched" }),
    ).toBeInTheDocument();
  });

  it("does not render when there are no in-progress movies", async () => {
    await renderHomeRoute({ continueWatching: [] });

    expect(
      await screen.findByRole("region", { name: "Recently Added Movies" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Continue Watching" }),
    ).not.toBeInTheDocument();
  });
});
