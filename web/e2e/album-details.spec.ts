import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  BREAKPOINTS,
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";
import { apiResponse, fulfillJSON, nullableInt64 } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  AURORA_PINES_ID,
  GLACIER_SESSIONS_ID,
  auroraPines,
  glacierSessions,
} from "./fixtures/music";

const EMPTY_ALBUM_ID = 43;

const emptyAlbumDetails = {
  ...glacierSessions,
  album: {
    ...glacierSessions.album,
    id: EMPTY_ALBUM_ID,
    title: "Silent Sessions",
    sort_title: "Silent Sessions",
    total_tracks: nullableInt64(0),
  },
  tracks: [],
  track_genres: [],
  album_genres: [],
  total_duration: 0,
};

async function mockAlbumDetailsApi(page: Page, { isAdmin = true }: { isAdmin?: boolean } = {}) {
  const likedTrackIds = new Set([101]);

  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: isAdmin },
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/music/albums/details/${GLACIER_SESSIONS_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(glacierSessions));
        return true;
      }

      if (url.pathname === `/api/music/albums/details/${EMPTY_ALBUM_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(emptyAlbumDetails));
        return true;
      }

      if (url.pathname === `/api/music/musicians/${AURORA_PINES_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(auroraPines));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [...likedTrackIds] }));
        return true;
      }

      const likeMatch = url.pathname.match(/^\/api\/music\/tracks\/(\d+)\/like$/);
      if (likeMatch && method === "POST") {
        const trackId = Number(likeMatch[1]);
        if (likedTrackIds.has(trackId)) {
          likedTrackIds.delete(trackId);
        } else {
          likedTrackIds.add(trackId);
        }
        await fulfillJSON(
          route,
          apiResponse({ track_id: trackId, is_liked: likedTrackIds.has(trackId) }),
        );
        return true;
      }

      // Starting playback makes the audio element request the stream; playback
      // itself is irrelevant here, the request just must not count as unexpected.
      if (/^\/api\/music\/tracks\/\d+\/stream$/.test(url.pathname) && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "audio/flac",
          body: Buffer.alloc(0),
        });
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("album details renders hero, tracklist, and details without console issues", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();
  await expect(page.getByText("Aurora Pines").first()).toBeVisible();

  // Hero actions
  const playAlbum = page.getByRole("button", { name: "Play Album", exact: true });
  const shuffle = page.getByRole("button", { name: "Shuffle play album" });
  const more = page.getByRole("button", { name: "More options" });
  await expect(playAlbum).toBeVisible();
  await expect(shuffle).toBeVisible();
  await expect(more).toBeVisible();

  // Hero buttons are keyboard-focusable (they carry the shared Button focus ring).
  await playAlbum.focus();
  await expect(playAlbum).toBeFocused();

  // Tracklist, including multi-disc headers.
  await expect(page.getByRole("heading", { name: "Track List" })).toBeVisible();
  await expect(page.getByText("Disc 1")).toBeVisible();
  await expect(page.getByText("Disc 2")).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Northern Drift" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Second Disc Opener" })).toBeVisible();

  // Liked state derived from liked-ids (track 101 is liked).
  await expect(page.getByRole("button", { name: "Remove Northern Drift from liked" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Add Cold Current to liked" })).toBeVisible();

  // Album details section + Spotify popularity.
  await expect(page.getByRole("heading", { name: "Album Details" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Spotify popularity 73 out of 100" })).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("audio player like button toggles the current track's liked state", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  // Starting playback opens the expanded player with the like button
  // (track 101 is pre-liked in the mock).
  await page.getByRole("button", { name: "Play Northern Drift" }).click();
  const dialog = page.getByRole("dialog");
  const expandedLike = dialog.getByRole("button", { name: "Remove Northern Drift from liked" });
  await expect(expandedLike).toBeVisible();
  await expect(expandedLike).toHaveAttribute("aria-pressed", "true");

  // Unlike from the expanded player.
  await expandedLike.click();
  const expandedUnliked = dialog.getByRole("button", { name: "Add Northern Drift to liked" });
  await expect(expandedUnliked).toBeVisible();
  await expect(expandedUnliked).toHaveAttribute("aria-pressed", "false");

  // The mini bar has its own like button and stays in sync.
  await dialog.getByRole("button", { name: "Minimize player (Escape)" }).click();
  const miniBar = page.getByRole("region", { name: "Audio player" });
  const miniLike = miniBar.getByRole("button", { name: "Add Northern Drift to liked" });
  await expect(miniLike).toBeVisible();

  // Like again from the mini bar; the track row shares the cache and flips too.
  await miniLike.click();
  await expect(
    miniBar.getByRole("button", { name: "Remove Northern Drift from liked" }),
  ).toBeVisible();
  await expect(
    page.getByRole("main").getByRole("button", { name: "Remove Northern Drift from liked" }),
  ).toBeVisible();

  // The extra button must not push the mini bar past a narrow viewport.
  await page.setViewportSize({ width: 375, height: 812 });
  await expect(
    miniBar.getByRole("button", { name: "Remove Northern Drift from liked" }),
  ).toBeVisible();
  await expectPageHasNoHorizontalScroll(page);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("artist links navigate to the musician details page", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  // Hero name, artist badge, and Album Details entry all link to the musician.
  const artistLinks = page.getByRole("link", { name: "Aurora Pines" });
  await expect(artistLinks).toHaveCount(3);

  // Artist links are keyboard-focusable (shared focus-visible ring recipe).
  await artistLinks.first().focus();
  await expect(artistLinks.first()).toBeFocused();

  await artistLinks.first().click();
  await expect(page).toHaveURL("/music/musician/7");
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("album with no tracks shows an empty state and hides playback buttons", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${EMPTY_ALBUM_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Silent Sessions" })).toBeVisible();
  await expect(page.getByText("No tracks in this album")).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Album", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Shuffle play album" })).toHaveCount(0);
  // Admins keep the delete path for empty albums.
  await expect(page.getByRole("button", { name: "More options" })).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("album details stays inside the viewport across breakpoints", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize(BREAKPOINTS[0].size);
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  for (const bp of BREAKPOINTS) {
    await page.setViewportSize(bp.size);

    const playAlbum = page.getByRole("button", { name: "Play Album", exact: true });
    const shuffle = page.getByRole("button", { name: "Shuffle play album" });
    await expect(playAlbum).toBeVisible();
    await expect(shuffle).toBeVisible();

    await expectPageHasNoHorizontalScroll(page);
    await expectNoHorizontalOverflow(playAlbum, `${bp.label} Play Album button`);
    await expectNoHorizontalOverflow(shuffle, `${bp.label} Shuffle button`);
    await expectNoHorizontalOverflow(
      page.getByRole("heading", { level: 1, name: "Glacier Sessions" }),
      `${bp.label} title`,
    );

    // Screenshot artifact for manual visual review at each breakpoint.
    await page.screenshot({ path: `test-results/album-details-${bp.label}.png`, fullPage: true });
  }

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("non-admin users do not see the delete album menu", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page, { isAdmin: false });

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Album", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "More options" })).toHaveCount(0);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
