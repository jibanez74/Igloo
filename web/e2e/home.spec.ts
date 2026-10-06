import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  BREAKPOINTS,
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
  VIEWPORTS,
} from "./e2e-layout";
import {
  apiResponse,
  fulfillJSON,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import type {
  ContinueWatchingItemType,
  LatestMovieType,
  LatestShowType,
  SimpleAlbumType,
  TheaterMovieType,
  WatchRoomType,
} from "../src/types";

// The home sections' content, motion and empty states are unit-tested
// (src/test/app/home-route.test.tsx); this covers what needs a browser: the
// shell's landmarks and skip link, the layout at every breakpoint, and the
// mobile navigation sheet.

const continueWatchingItems: ContinueWatchingItemType[] = [
  {
    kind: "movie",
    id: 104,
    title: "Ember Line",
    poster_path: nullableString("/ember-line.jpg"),
    year: nullableInt64(2026),
    progress_sec: 1830,
    duration_sec: 5400,
  },
  {
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
  },
  {
    kind: "movie",
    id: 105,
    title: "Quiet Orbit",
    poster_path: nullableString(),
    year: nullableInt64(2023),
    progress_sec: 300,
    duration_sec: 6000,
  },
];

function theaterMovie(
  id: number,
  title: string,
  posterPath: string,
  voteAverage: number,
  releaseDate: string,
): TheaterMovieType {
  return {
    id,
    title,
    original_title: title,
    overview: `${title} overview`,
    release_date: releaseDate,
    poster_path: posterPath,
    backdrop_path: "",
    popularity: 10,
    vote_average: voteAverage,
    vote_count: 100,
    adult: false,
    original_language: "en",
    genre_ids: [],
    video: false,
  };
}

async function mockHomeApi(page: Page) {
  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url }) => {
      const { pathname } = url;

      if (pathname === "/api/watch-rooms") {
        await fulfillJSON(route, apiResponse({
          rooms: [
            {
              id: 7,
              movie_id: 101,
              movie_title: "Signal Fire",
              movie_poster: "/signal-fire.jpg",
              owner: {
                id: 1,
                name: "Admin User",
                avatar: null,
              },
              members: [
                { id: 1, name: "Admin User", avatar: null },
                { id: 2, name: "Maya Chen", avatar: null },
                { id: 3, name: "Rowan Price", avatar: null },
                { id: 4, name: "Luis Ortiz", avatar: null },
                { id: 5, name: "Ava Bell", avatar: null },
              ],
              playback_mode: "direct",
              is_owner: true,
              created_at: "2026-01-01T00:00:00Z",
            },
          ] satisfies WatchRoomType[],
        }));
        return true;
      }

      if (pathname === "/api/continue-watching") {
        await fulfillJSON(route, apiResponse({ items: continueWatchingItems }));
        return true;
      }

      if (pathname === "/api/movies/latest") {
        await fulfillJSON(route, apiResponse({
          movies: [
            {
              id: 101,
              title: "Signal Fire",
              poster_path: nullableString("/signal-fire.jpg"),
              year: nullableInt64(2026),
            },
            {
              id: 102,
              title: "Cinder Vale",
              poster_path: nullableString("/cinder-vale.jpg"),
              year: nullableInt64(2025),
            },
            {
              id: 103,
              title: "Mercury Harbor",
              poster_path: nullableString(),
              year: nullableInt64(2024),
            },
          ] satisfies LatestMovieType[],
        }));
        return true;
      }

      if (pathname === "/api/shows/latest") {
        await fulfillJSON(route, apiResponse({
          shows: [
            {
              id: 301,
              name: "Frost Harbor",
              poster_path: nullableString("/frost-harbor.jpg"),
              premiere_year: nullableInt64(2026),
            },
            {
              id: 302,
              name: "Halcyon Drift",
              poster_path: nullableString("/halcyon-drift.jpg"),
              premiere_year: nullableInt64(2024),
            },
            {
              id: 303,
              name: "Lantern Bay",
              poster_path: nullableString(),
              premiere_year: nullableInt64(),
            },
          ] satisfies LatestShowType[],
        }));
        return true;
      }

      if (pathname === "/api/music/albums/latest") {
        await fulfillJSON(route, apiResponse({
          albums: [
            {
              id: 201,
              title: "Blue Record",
              cover: nullableString("albums/blue-record.jpg"),
              musician: nullableString("Aurora Pines"),
              year: nullableInt64(2026),
            },
            {
              id: 202,
              title: "Warm Static",
              cover: nullableString(),
              musician: nullableString("Amber Field"),
              year: nullableInt64(2025),
            },
            {
              id: 203,
              title: "Night Transit",
              cover: nullableString(),
              musician: nullableString("Cedar Room"),
              year: nullableInt64(2024),
            },
          ] satisfies SimpleAlbumType[],
        }));
        return true;
      }

      if (pathname === "/api/tmdb/movies/in-theaters") {
        await fulfillJSON(route, apiResponse({
          movies: [
            theaterMovie(301, "Northbound", "/northbound.jpg", 7.6, "2026-06-01"),
            theaterMovie(302, "Glass Harbor", "/glass-harbor.jpg", 6.2, "2026-05-16"),
            theaterMovie(303, "Red Echo", "", 4.8, "2026-04-08"),
          ],
        }));
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("home page is clean, responsive, and keyboard reachable", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockHomeApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/");

  await expect(page).toHaveTitle("Home - Igloo");
  await expect(
    page.getByRole("heading", { name: "Welcome to Igloo" }),
  ).toBeVisible();
  await expect(page.getByRole("main")).toBeVisible();
  await expect(
    page.getByRole("search", { name: "Search library" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Notifications" }),
  ).toBeVisible();
  // The unit tests mock the top bar, so this is what proves it mounts.
  await expect(
    page.getByRole("button", { name: "Switch to light theme" }),
  ).toBeVisible();

  for (const name of [
    "Watch Rooms",
    "Continue Watching",
    "Recently Added Movies",
    "Recently Added Shows",
    "Recently Added Albums",
    "Now Playing in Theaters",
  ]) {
    await expect(page.getByRole("region", { name })).toBeVisible();
  }

  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", { name: "Skip to page content" });
  await expect(skipLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();

  const main = page.getByRole("main");
  const emberCard = page
    .getByRole("region", { name: "Continue Watching" })
    .getByRole("link", { name: "Ember Line 2026, 34% watched" });
  for (const { label, size } of BREAKPOINTS) {
    await page.setViewportSize(size);

    await expect(
      page.getByRole("heading", { name: "Welcome to Igloo" }),
    ).toBeVisible();
    await expectPageHasNoHorizontalScroll(page);
    await expectNoHorizontalOverflow(main, `main content at ${label} width`);
    // Sparse sections must not stretch posters across the content column:
    // the auto-fill grid keeps cards near the track's minimum width.
    await expect
      .poll(async () => (await emberCard.boundingBox())?.width, {
        message: `continue watching card width at ${label} width`,
      })
      .toBeLessThan(300);
  }

  await page.setViewportSize(VIEWPORTS.phone);
  const sidebarToggle = page.getByRole("button", { name: "Toggle Sidebar" });
  await expect(sidebarToggle).toBeVisible();
  await sidebarToggle.click();

  await expect(page.getByRole("dialog", { name: "Navigation" })).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Main navigation" }),
  ).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("dialog", { name: "Navigation" }),
  ).toBeHidden();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("continue watching reveals the same resume action on movies and episodes", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockHomeApi(page);
  // Hovering a card warms its details page (design-system §PosterCard);
  // registered after the catch-all, this route answers those requests first.
  const prefetched = new Set<string>();
  await page.route(
    /\/api\/(movies\/(details\/)?104|shows\/details\/301)(\/|$)/,
    async route => {
      prefetched.add(new URL(route.request().url()).pathname);
      await fulfillJSON(route, apiResponse(null));
    },
  );

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/");

  const watching = page.getByRole("region", { name: "Continue Watching" });
  // The play control is an icon revealed on hover, so its name is the only
  // wording a viewer gets; both kinds in the row must read the same.
  for (const [card, action] of [
    ["Ember Line 2026, 34% watched", "Resume Ember Line 2026"],
    [
      "Frost Harbor, S1 E4 · Thin Ice, 25% watched",
      "Resume Frost Harbor S1 E4 · Thin Ice",
    ],
  ]) {
    const resume = watching.getByRole("link", { name: action });
    // The hidden play control covers the poster's centre, so enter the card
    // by its corner the way a pointer sweeping across the row would.
    await watching
      .getByRole("link", { name: card })
      .hover({ position: { x: 12, y: 12 } });
    await expect(resume).toHaveCSS("opacity", "1");
  }
  await expect(watching.getByRole("link", { name: /^Play / })).toHaveCount(0);
  await expect
    .poll(() => [...prefetched])
    .toEqual(
      expect.arrayContaining([
        "/api/movies/details/104",
        "/api/shows/details/301",
      ]),
    );

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
