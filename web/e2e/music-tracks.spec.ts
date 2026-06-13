import { expect, test, type Page, type Route } from "@playwright/test";

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

function track(id: number) {
  return {
    id,
    title: `Track ${id.toString().padStart(4, "0")}`,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: `/music/track-${id}.flac`,
    album_id: nullableInt64(1),
    album_title: nullableString("Mock Album"),
    album_cover: nullableString(),
    musician_id: nullableInt64(1),
    musician_name: nullableString("Mock Artist"),
  };
}

const mockAlbum = {
  id: 1,
  title: "Mock Album",
  cover: nullableString("/api/static/albums/mock-album.svg"),
  musician: nullableString("Mock Artist"),
  year: nullableInt64(2026),
};

const coverlessAlbum = {
  id: 2,
  title: "Coverless Album",
  cover: nullableString(),
  musician: nullableString("No Cover Artist"),
  year: nullableInt64(2026),
};

const pageTwoAlbum = {
  id: 3,
  title: "Page Two Album",
  cover: nullableString("/api/static/albums/page-two-album.svg"),
  musician: nullableString("Second Page Artist"),
  year: nullableInt64(2026),
};

const mockMusician = {
  id: 1,
  name: "Mock Artist",
  sort_name: "Mock Artist",
  thumb: nullableString(),
  album_count: 1,
  track_count: 2267,
};

const mockPlaylist = {
  id: 1,
  user_id: 1,
  name: "Mock Playlist",
  description: nullableString("A deterministic playlist for music E2E tests"),
  cover_image: nullableString(),
  is_public: false,
  folder_id: nullableInt64(),
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  track_count: 2,
  total_duration: 360000,
  is_owner: true,
  can_edit: true,
};

async function fulfillJSON(route: Route, body: unknown) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function mockMusicApi(
  page: Page,
  requestedOffsets: number[],
  requestedAlbumRequests: string[] = [],
) {
  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());

    if (url.pathname.startsWith("/api/static/albums/")) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" fill="#f59e0b"/><circle cx="32" cy="32" r="18" fill="#0f172a"/></svg>`,
      });
      return;
    }

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Music User",
          email: "music@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/music/stats") {
      await fulfillJSON(route, apiResponse({
        total_albums: 3,
        total_tracks: 2267,
        total_musicians: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/albums") {
      const albumPage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      requestedAlbumRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        albums: albumPage === 2 ? [pageTwoAlbum] : [mockAlbum, coverlessAlbum],
        total: 3,
        page: albumPage,
        per_page: perPage,
        total_pages: 2,
      }));
      return;
    }

    if (url.pathname === "/api/music/musicians") {
      await fulfillJSON(route, apiResponse({
        musicians: [mockMusician],
        total: 1,
        page: 1,
        per_page: 24,
        total_pages: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/playlists") {
      await fulfillJSON(route, apiResponse({
        playlists: [mockPlaylist],
      }));
      return;
    }

    if (url.pathname === "/api/music/tracks/liked-ids") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
      return;
    }

    if (url.pathname === "/api/music/tracks") {
      const limit = Number(url.searchParams.get("limit") ?? "50");
      const offset = Number(url.searchParams.get("offset") ?? "0");
      const total = 2267;
      const trackCount = Math.max(0, Math.min(limit, total - offset));
      requestedOffsets.push(offset);

      await fulfillJSON(route, apiResponse({
        tracks: Array.from({ length: trackCount }, (_, index) => track(offset + index + 1)),
        total,
        offset,
        limit,
        has_more: offset + limit < total,
      }));
      return;
    }

    await fulfillJSON(route, {
      error: true,
      message: `Unexpected API request: ${url.pathname}`,
    });
  });
}

async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => {
    const root = document.scrollingElement ?? document.documentElement;
    const offenders = Array.from(document.querySelectorAll<HTMLElement>("body *"))
      .filter(element => {
        const rect = element.getBoundingClientRect();
        return rect.width > 0 && rect.right > window.innerWidth + 1;
      })
      .slice(0, 5)
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        className: element.className.toString(),
        right: element.getBoundingClientRect().right,
      }));

    return {
      clientWidth: root.clientWidth,
      offenders,
      scrollWidth: root.scrollWidth,
    };
  });

  expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1);
  expect(overflow.offenders).toEqual([]);
}

test("music library shell and URL-backed tabs render accessibly", async ({ page }) => {
  const requestedOffsets: number[] = [];

  await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/music");

  await expect(page).toHaveTitle("Music Library - Igloo");
  await expect(page.getByRole("heading", { name: "Music Library", level: 1 })).toBeVisible();
  await expect(page.getByLabel("Library statistics: 3 albums, 2267 tracks, 1 musicians")).toBeVisible();

  const tablist = page.getByRole("tablist");
  await expect(tablist).toBeVisible();

  const tabs = page.getByRole("tab");
  await expect(tabs).toHaveCount(4);

  const musiciansTab = page.getByRole("tab", { name: "Musicians" });
  const albumsTab = page.getByRole("tab", { name: "Albums" });
  const tracksTab = page.getByRole("tab", { name: "Tracks" });
  const playlistsTab = page.getByRole("tab", { name: "Playlists" });

  await expect(albumsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel")).toBeVisible();
  await expect(page.getByRole("link", { name: "Mock Album by Mock Artist" })).toBeVisible();

  await musiciansTab.click();
  await expect(page).toHaveURL(/tab=musicians/);
  await expect(musiciansTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("link", { name: "Mock Artist, 1 albums, 2267 tracks" })).toBeVisible();

  await albumsTab.click();
  await expect(page).toHaveURL(/tab=albums/);
  await expect(albumsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("link", { name: "Mock Album by Mock Artist" })).toBeVisible();

  await tracksTab.click();
  await expect(page).toHaveURL(/tab=tracks/);
  await expect(tracksTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("list", { name: "Tracks" })).toBeVisible();

  await playlistsTab.click();
  await expect(page).toHaveURL(/tab=playlists/);
  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("link", { name: "Mock Playlist, 2 tracks, 6m 0s" })).toBeVisible();
});

test("albums tab renders accessible album cards and URL-backed pagination", async ({ page }) => {
  const requestedOffsets: number[] = [];
  const requestedAlbumRequests: string[] = [];

  await mockMusicApi(page, requestedOffsets, requestedAlbumRequests);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/music");

  const albumsTab = page.getByRole("tab", { name: "Albums" });
  await expect(albumsTab).toHaveAttribute("aria-selected", "true");

  const mockAlbumLink = page.getByRole("link", { name: "Mock Album by Mock Artist" });
  await expect(mockAlbumLink).toBeVisible();
  await expect(mockAlbumLink.getByRole("img", { name: "Album cover for Mock Album" })).toBeVisible();

  const coverlessAlbumLink = page.getByRole("link", { name: "Coverless Album by No Cover Artist" });
  await expect(coverlessAlbumLink).toBeVisible();
  await expect(coverlessAlbumLink.locator("img")).toHaveCount(0);
  await expect(coverlessAlbumLink.locator("svg")).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/albumsPage=2/);
  await expect
    .poll(() =>
      requestedAlbumRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/music/albums" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === "24"
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Page Two Album by Second Page Artist" })).toBeVisible();
});

test("tracks tab keeps fetching pages while the virtualized list grows", async ({ page }) => {
  const consoleIssues: string[] = [];
  const requestedOffsets: number[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });

  await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/music?tab=tracks");

  const tracksList = page.getByRole("list", { name: "Tracks" });

  await expect(tracksList).toBeVisible();
  const loadedStatus = page
    .getByText(/\d+ of 2267 tracks loaded/)
    .first();
  await expect(loadedStatus).toBeVisible();
  await expect
    .poll(async () => {
      const statusText = await loadedStatus.textContent();
      return Number(statusText?.match(/^\d+/)?.[0] ?? 0);
    })
    .toBeGreaterThanOrEqual(50);
  await expectNoHorizontalOverflow(page);

  for (let index = 0; index < 8; index += 1) {
    await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight);

      for (const element of document.querySelectorAll<HTMLElement>("*")) {
        if (element.scrollHeight > element.clientHeight) {
          element.scrollTop = element.scrollHeight;
        }
      }
    });

    if (requestedOffsets.includes(100)) {
      break;
    }
    await page.waitForTimeout(100);
  }

  await expect.poll(() => requestedOffsets).toContainEqual(100);
  expect(requestedOffsets).toEqual(expect.arrayContaining([0, 50, 100]));

  await page.evaluate(() => {
    window.scrollTo(0, 0);

    for (const element of document.querySelectorAll<HTMLElement>("*")) {
      if (element.scrollTop > 0) {
        element.scrollTop = 0;
      }
    }
  });

  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  expect(consoleIssues).toEqual([]);
});

test("tracks tab fits on mobile", async ({ page }) => {
  const consoleIssues: string[] = [];
  const requestedOffsets: number[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });

  await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/music?tab=tracks");

  await expect(page.getByRole("list", { name: "Tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  expect(consoleIssues).toEqual([]);
});
