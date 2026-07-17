import { expect, test, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";

type NullableString = {
  String: string;
  Valid: boolean;
};

type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

function nullableString(value = ""): NullableString {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function apiResponse(data: unknown) {
  return {
    error: false,
    data,
  };
}

async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function placeholderSvg(label: string, background: string, foreground = "#f8fafc") {
  return `<?xml version="1.0" encoding="UTF-8"?>
  <svg xmlns="http://www.w3.org/2000/svg" width="500" height="750" viewBox="0 0 500 750" role="img" aria-label="${label}">
    <rect width="500" height="750" fill="${background}" />
    <text
      x="50%"
      y="50%"
      dominant-baseline="middle"
      text-anchor="middle"
      fill="${foreground}"
      font-family="system-ui, sans-serif"
      font-size="40"
      font-weight="700"
    >${label}</text>
  </svg>`;
}

type MockHomeApiOptions = {
  continueWatching?: unknown[];
};

const defaultContinueWatchingMovies = [
  {
    id: 104,
    title: "Ember Line",
    poster_path: nullableString("/ember-line.jpg"),
    year: nullableInt64(2026),
    progress_sec: 1830,
    duration_sec: 5400,
  },
  {
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
    options.continueWatching ?? defaultContinueWatchingMovies;
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const { pathname } = url;

    if (pathname.startsWith("/api/tmdb/images/")) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: placeholderSvg("Poster", "#1e293b", "#f59e0b"),
      });
      return;
    }

    if (pathname.startsWith("/api/static/")) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: placeholderSvg("Cover", "#334155"),
      });
      return;
    }

    if (pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Admin User",
          email: "admin@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (pathname === "/api/notifications/unread-count") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

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
      return;
    }

    if (pathname === "/api/movies/continue-watching") {
      await fulfillJSON(route, apiResponse({ movies: continueWatching }));
      return;
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
      return;
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
        ],
      }));
      return;
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
      return;
    }

    if (/^\/api\/movies\/details\/\d+$/.test(pathname)) {
      const movieId = Number(pathname.split("/").pop());

      await fulfillJSON(route, apiResponse({
        movie: {
          id: movieId,
          title: movieId === 101 ? "Signal Fire" : "Prefetched Movie",
        },
      }));
      return;
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
      return;
    }

    unexpectedApiRequests.push(`${route.request().method()} ${pathname}`);
    await fulfillJSON(route, apiResponse({}));
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
    "Recently Added Albums",
    "Now Playing in Theaters",
  ]) {
    const region = page.getByRole("region", { name });

    await expect(region).toBeVisible();
    await expect(region).toHaveClass(/delay-75/);
  }

  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", { name: "Skip to content" });
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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("continue watching section is hidden when there are no in-progress movies", async ({
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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});
