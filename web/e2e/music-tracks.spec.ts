import { expect, test, type Page } from "@playwright/test";
import {
  ALBUMS_PER_PAGE,
  MUSICIANS_PER_PAGE,
  TRACKS_INFINITE_PAGE_SIZE,
} from "../src/lib/constants";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { VIEWPORTS, expectNoOverflowingElements } from "./e2e-layout";
import { apiResponse, fulfillJSON, nullableInt64, nullableString } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  playlistSummary,
  simpleAlbum,
  simpleMusician,
  trackListItem,
} from "./fixtures/music";

function track(id: number) {
  return trackListItem({
    id,
    title: `Track ${id.toString().padStart(4, "0")}`,
    album_id: nullableInt64(1),
    album_title: nullableString("Mock Album"),
    musician_id: nullableInt64(1),
    musician_name: nullableString("Mock Artist"),
  });
}

const mockAlbum = simpleAlbum({
  id: 1,
  title: "Mock Album",
  cover: nullableString("/api/static/albums/mock-album.svg"),
  musician: nullableString("Mock Artist"),
});

const coverlessAlbum = simpleAlbum({
  id: 2,
  title: "Coverless Album",
  musician: nullableString("No Cover Artist"),
});

const pageTwoAlbum = simpleAlbum({
  id: 3,
  title: "Page Two Album",
  cover: nullableString("/api/static/albums/page-two-album.svg"),
  musician: nullableString("Second Page Artist"),
});

const mockMusician = simpleMusician({
  id: 1,
  name: "Mock Artist",
  album_count: 1,
  track_count: 2267,
});

const mockPlaylist = playlistSummary({
  id: 1,
  name: "Mock Playlist",
  description: nullableString("A deterministic playlist for music E2E tests"),
  track_count: 2,
  total_duration: 360000,
});

async function mockMusicApi(
  page: Page,
  requestedOffsets: number[],
  requestedAlbumRequests: string[] = [],
) {
  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url }) => {
      if (url.pathname === "/api/music/stats") {
        await fulfillJSON(route, apiResponse({
          total_albums: 3,
          total_tracks: 2267,
          total_musicians: 1,
        }));
        return true;
      }

      // The library menu's Spotify request items read this on every tab.
      if (url.pathname === "/api/spotify/status") {
        await fulfillJSON(route, apiResponse({ available: false }));
        return true;
      }

      if (url.pathname === "/api/music/albums") {
        const albumPage = Number(url.searchParams.get("page") ?? "1");
        const perPage = Number(
          url.searchParams.get("per_page") ?? String(ALBUMS_PER_PAGE),
        );
        requestedAlbumRequests.push(`${url.pathname}${url.search}`);

        await fulfillJSON(route, apiResponse({
          albums: albumPage === 2 ? [pageTwoAlbum] : [mockAlbum, coverlessAlbum],
          total: 3,
          page: albumPage,
          per_page: perPage,
          total_pages: 2,
        }));
        return true;
      }

      if (url.pathname === "/api/music/musicians") {
        await fulfillJSON(route, apiResponse({
          musicians: [mockMusician],
          total: 1,
          page: 1,
          per_page: MUSICIANS_PER_PAGE,
          total_pages: 1,
        }));
        return true;
      }

      if (url.pathname === "/api/music/playlists") {
        await fulfillJSON(route, apiResponse({
          playlists: [mockPlaylist],
        }));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
        return true;
      }

      if (url.pathname === "/api/music/tracks") {
        const limit = Number(
          url.searchParams.get("limit") ?? String(TRACKS_INFINITE_PAGE_SIZE),
        );
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
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("music library shell and URL-backed tabs render accessibly", async ({ page }) => {
  const requestedOffsets: number[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music");

  await expect(page).toHaveTitle("Music Library - Igloo");
  await expect(page.getByRole("heading", { name: "Music Library", level: 1 })).toBeVisible();
  await expect(page.getByLabel("Library statistics: 3 albums, 2267 tracks, 1 musician", { exact: true })).toBeVisible();

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
  await expect(page.getByRole("link", { name: "Mock Artist, 1 album, 2267 tracks" })).toBeVisible();

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
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("albums tab renders accessible album cards and URL-backed pagination", async ({ page }) => {
  const requestedOffsets: number[] = [];
  const requestedAlbumRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicApi(
    page,
    requestedOffsets,
    requestedAlbumRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music");

  const albumsTab = page.getByRole("tab", { name: "Albums" });
  await expect(albumsTab).toHaveAttribute("aria-selected", "true");

  const mockAlbumLink = page.getByRole("link", { name: "Mock Album by Mock Artist" });
  await expect(mockAlbumLink).toBeVisible();
  // The link already names the album, so its cover is decorative (alt="").
  await expect(mockAlbumLink.locator("img")).toBeVisible();
  await expect(mockAlbumLink.locator("img")).toHaveAttribute("alt", "");

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
          parsed.searchParams.get("per_page") === String(ALBUMS_PER_PAGE)
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Page Two Album by Second Page Artist" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("tracks tab keeps fetching pages while the virtualized list grows", async ({ page }) => {
  const requestedOffsets: number[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize(VIEWPORTS.desktop);
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
    .toBeGreaterThanOrEqual(TRACKS_INFINITE_PAGE_SIZE);
  await expectNoOverflowingElements(page);

  for (let index = 0; index < 8; index += 1) {
    await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight);

      for (const element of document.querySelectorAll<HTMLElement>("*")) {
        if (element.scrollHeight > element.clientHeight) {
          element.scrollTop = element.scrollHeight;
        }
      }
    });

    if (requestedOffsets.includes(TRACKS_INFINITE_PAGE_SIZE * 2)) {
      break;
    }
    await page.waitForTimeout(100);
  }

  await expect
    .poll(() => requestedOffsets)
    .toContainEqual(TRACKS_INFINITE_PAGE_SIZE * 2);
  expect(requestedOffsets).toEqual(
    expect.arrayContaining([
      0,
      TRACKS_INFINITE_PAGE_SIZE,
      TRACKS_INFINITE_PAGE_SIZE * 2,
    ]),
  );
  expect(requestedOffsets).toEqual([...new Set(requestedOffsets)]);

  await page.evaluate(() => {
    window.scrollTo(0, 0);

    for (const element of document.querySelectorAll<HTMLElement>("*")) {
      if (element.scrollTop > 0) {
        element.scrollTop = 0;
      }
    }
  });

  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoOverflowingElements(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("tracks tab fits on mobile", async ({ page }) => {
  const requestedOffsets: number[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize(VIEWPORTS.phone);
  await page.goto("/music?tab=tracks");

  await expect(page.getByRole("list", { name: "Tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoOverflowingElements(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
