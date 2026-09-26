import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";
import {
  apiResponse,
  fulfillJSON,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import type { SimpleAlbumType } from "../src/types";

type MockHomeApiOptions = {
  continueWatching?: unknown[];
};

const defaultContinueWatchingItems = [
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

async function mockHomeApi(page: Page, options: MockHomeApiOptions = {}) {
  const continueWatching =
    options.continueWatching ?? defaultContinueWatchingItems;

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
              is_owner: true,
            },
          ],
        }));
        return true;
      }

      if (pathname === "/api/continue-watching") {
        await fulfillJSON(route, apiResponse({ items: continueWatching }));
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
          ],
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
          ],
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
            {
              id: 301,
              title: "Northbound",
              poster_path: "/northbound.jpg",
              vote_average: 7.6,
              release_date: "2026-06-01",
            },
            {
              id: 302,
              title: "Glass Harbor",
              poster_path: "/glass-harbor.jpg",
              vote_average: 6.2,
              release_date: "2026-05-16",
            },
            {
              id: 303,
              title: "Red Echo",
              poster_path: "",
              vote_average: 4.8,
              release_date: "2026-04-08",
            },
          ],
        }));
        return true;
      }

      if (/^\/api\/movies\/details\/\d+$/.test(pathname)) {
        const movieId = Number(pathname.split("/").pop());

        await fulfillJSON(route, apiResponse({
          movie: {
            id: movieId,
            title: movieId === 101 ? "Signal Fire" : "Prefetched Movie",
          },
        }));
        return true;
      }

      if (/^\/api\/music\/albums\/details\/\d+$/.test(pathname)) {
        const albumId = Number(pathname.split("/").pop());

        await fulfillJSON(route, apiResponse({
          album: {
            id: albumId,
            title: albumId === 201 ? "Blue Record" : "Prefetched Album",
            cover: nullableString("albums/blue-record.jpg"),
            musician: nullableString("Aurora Pines"),
          },
          tracks: [
            {
              id: 1,
              title: "Alabaster",
              duration: 180,
              codec: "flac",
              bit_rate: 900000,
              file_path: "/music/alabaster.flac",
              album_id: nullableInt64(albumId),
              album_title: nullableString("Blue Record"),
              album_cover: nullableString("albums/blue-record.jpg"),
              musician_id: nullableInt64(1),
              musician_name: nullableString("Aurora Pines"),
            },
          ],
        }));
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("home page is clean, responsive, and accessible", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockHomeApi(page);

  await page.addInitScript(() => {
    window.localStorage.removeItem("igloo-theme");
  });
  await page.goto("/");

  await expect(page).toHaveTitle("Home - Igloo");
  await expect(
    page.getByRole("heading", { name: "Welcome to Igloo" }),
  ).toBeVisible();
  const dashboardHero = page
    .getByRole("heading", { name: "Welcome to Igloo" })
    .locator("xpath=ancestor::section[1]");
  await expect(dashboardHero).toHaveClass(/animate-in/);
  await expect(dashboardHero).toHaveClass(/fade-in-0/);
  await expect(page.getByRole("main")).toBeVisible();
  await expect(
    page.getByRole("search", { name: "Search library" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Notifications" }),
  ).toBeVisible();
  const themeToggle = page.getByRole("button", {
    name: "Switch to light theme",
  });
  await expect(themeToggle).toBeVisible();

  for (const name of [
    "Watch Rooms",
    "Continue Watching",
    "Recently Added Movies",
    "Recently Added Shows",
    "Recently Added Albums",
    "Now Playing in Theaters",
  ]) {
    const region = page.getByRole("region", { name });

    await expect(region).toBeVisible();
    await expect(region).toHaveClass(/delay-75/);
  }

  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", { name: "Skip to page content" });
  await expect(skipLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();

  await themeToggle.click();
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("igloo-theme")))
    .toBe("light");
  await expect(
    page.getByRole("button", { name: "Switch to dark theme" }),
  ).toBeVisible();

  const main = page.getByRole("main");
  for (const viewport of [
    { width: 375, height: 900, label: "mobile" },
    { width: 768, height: 1024, label: "tablet" },
    { width: 1280, height: 900, label: "desktop" },
  ]) {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    });

    await expectPageHasNoHorizontalScroll(page);
    await expectNoHorizontalOverflow(
      main,
      `main content at ${viewport.label} width`,
    );
    await expect(
      page.getByRole("heading", { name: "Welcome to Igloo" }),
    ).toBeVisible();
  }

  await page.setViewportSize({ width: 375, height: 900 });
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

test("continue watching section announces progress", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockHomeApi(page);

  await page.goto("/");

  const watchingRegion = page.getByRole("region", {
    name: "Continue Watching",
  });
  await expect(watchingRegion).toBeVisible();
  const emberCard = watchingRegion.getByRole("link", {
    name: "Ember Line 2026, 34% watched",
  });
  await expect(emberCard).toBeVisible();

  // Sparse sections must not stretch posters across the content column —
  // the auto-fill grid keeps cards near the track min width.
  const box = await emberCard.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.width).toBeLessThan(300);

  // Episodes share the row with the movies, ordered by the server.
  await expect(
    watchingRegion.getByRole("link", {
      name: "Frost Harbor, S1 E4 · Thin Ice, 25% watched",
    }),
  ).toBeVisible();
  await expect(
    watchingRegion.getByRole("link", {
      name: "Resume Frost Harbor S1 E4 · Thin Ice",
    }),
  ).toHaveAttribute("href", "/tv-shows/301/episodes/70103/play");

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("continue watching section is hidden when nothing is in progress", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockHomeApi(page, {
    continueWatching: [],
  });

  await page.goto("/");

  await expect(
    page.getByRole("region", { name: "Recently Added Movies" }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Continue Watching" }),
  ).toHaveCount(0);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
