import type React from "react";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
} from "@/lib/constants";
import { authUserData } from "../helpers/fixtures";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";

vi.mock("@/components/app/AppShell", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <main id="main">{children}</main>
  ),
}));

function authUser() {
  return authUserData({
    name: "Admin User",
    email: "admin@example.com",
    is_admin: true,
  });
}

type MockHomeFetchOptions = {
  continueWatching?: unknown[];
  /** The first latest-movies request answers 500; the retry succeeds. */
  latestMoviesFailsFirst?: boolean;
};

const defaultContinueWatchingItems = [
  {
    kind: "movie",
    id: 104,
    title: "Ember Line",
    poster_path: { String: "", Valid: false },
    year: { Int64: 2026, Valid: true },
    progress_sec: 1830,
    duration_sec: 5400,
  },
  {
    kind: "episode",
    id: 70103,
    title: "Frost Harbor",
    poster_path: { String: "", Valid: false },
    year: { Int64: 2025, Valid: true },
    progress_sec: 600,
    duration_sec: 2400,
    show_id: 301,
    season_number: 1,
    episode_number: 4,
    episode_name: "Thin Ice",
  },
];

function mockHomeFetch(options: MockHomeFetchOptions = {}) {
  const continueWatching =
    options.continueWatching ?? defaultContinueWatchingItems;
  let latestMoviesRequests = 0;
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

    if (url === "/api/continue-watching") {
      return jsonResponse({
        error: false,
        data: { items: continueWatching },
      });
    }

    if (url === "/api/movies/latest") {
      latestMoviesRequests += 1;
      if (options.latestMoviesFailsFirst && latestMoviesRequests === 1) {
        return jsonResponse({ error: true, message: "latest exploded" }, 500);
      }
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

    if (url === "/api/shows/latest") {
      return jsonResponse({
        error: false,
        data: {
          shows: [
            {
              id: 301,
              name: "Frost Harbor",
              poster_path: { String: "", Valid: false },
              premiere_year: { Int64: 2025, Valid: true },
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

async function renderHomeRoute(options: MockHomeFetchOptions = {}) {
  mockHomeFetch(options);

  return renderRoute("/");
}

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
      "Recently Added Shows",
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

    expect(
      screen.getByText("1 show available in recently added shows."),
    ).toBeInTheDocument();
    expect(screen.getAllByText("1 show").length).toBeGreaterThan(0);
    expect(screen.queryByText(/1 shows/)).not.toBeInTheDocument();
  });
});

describe("home section errors", () => {
  it("words a failed section itself and recovers through Try again", async () => {
    const user = userEvent.setup();
    await renderHomeRoute({ latestMoviesFailsFirst: true });

    const region = await screen.findByRole("region", {
      name: "Recently Added Movies",
    });
    const alert = await within(region).findByRole("alert");
    expect(alert).toHaveTextContent("Couldn’t load movies.");
    expect(alert).not.toHaveTextContent("exploded");

    await user.click(within(region).getByRole("button", { name: "Try again" }));

    expect(
      await within(region).findByRole("link", { name: "Signal Fire 2026" }),
    ).toBeInTheDocument();
    expect(within(region).queryByRole("alert")).not.toBeInTheDocument();
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

  it("shows in-progress episodes beside the movies", async () => {
    await renderHomeRoute();

    // 600 / 2400 is 25%.
    expect(
      await screen.findByRole("link", {
        name: "Frost Harbor, S1 E4 · Thin Ice, 25% watched",
      }),
    ).toHaveAttribute("href", "/tv-shows/301");
    expect(
      screen.getByRole("link", {
        name: "Resume Frost Harbor S1 E4 · Thin Ice",
      }),
    ).toHaveAttribute("href", "/tv-shows/301/episodes/70103/play");
    // Both kinds share one row, counted with a neutral noun.
    expect(
      screen.getByText("2 titles available in continue watching."),
    ).toBeInTheDocument();
  });

  it("names every play action in the row as a resume", async () => {
    await renderHomeRoute();

    const watchingRegion = await screen.findByRole("region", {
      name: "Continue Watching",
    });
    expect(
      within(watchingRegion).getByRole("link", {
        name: "Resume Ember Line 2026",
      }),
    ).toHaveAttribute("href", "/movies/104/play");
    // Movies and episodes share the row, so they share the verb.
    const playActions = within(watchingRegion).getAllByRole("link", {
      name: /^(Play|Resume) /,
    });
    expect(playActions.map(link => link.getAttribute("aria-label"))).toEqual([
      "Resume Ember Line 2026",
      "Resume Frost Harbor S1 E4 · Thin Ice",
    ]);
  });

  it("does not render when nothing is in progress", async () => {
    await renderHomeRoute({ continueWatching: [] });

    expect(
      await screen.findByRole("region", { name: "Recently Added Movies" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Continue Watching" }),
    ).not.toBeInTheDocument();
  });
});
