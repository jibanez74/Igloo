import { expect, test, type Locator, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";

type NullableString = { String: string; Valid: boolean };
type NullableInt64 = { Int64: number; Valid: boolean };

function nullableString(value = ""): NullableString {
  return { String: value, Valid: value.length > 0 };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return { Int64: value ?? 0, Valid: value != null };
}

function apiResponse(data: unknown) {
  return { error: false, data };
}

const ALBUM_ID = 42;

function makeTrack(overrides: Record<string, unknown>) {
  return {
    id: 0,
    title: "Untitled",
    sort_title: "Untitled",
    file_path: "/music/untitled.flac",
    file_name: "untitled.flac",
    container: "flac",
    mime_type: "audio/flac",
    codec: "flac",
    size: 12_000_000,
    track_index: 1,
    duration: 200_000,
    disc: 1,
    channels: "2",
    channel_layout: "stereo",
    bit_rate: 900_000,
    profile: "",
    release_date: nullableString(),
    year: nullableInt64(2026),
    composer: nullableString(),
    copyright: nullableString(),
    language: nullableString(),
    album_id: nullableInt64(ALBUM_ID),
    musician_id: nullableInt64(7),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

const albumDetails = {
  album: {
    id: ALBUM_ID,
    title: "Glacier Sessions",
    sort_title: "Glacier Sessions",
    musician: nullableString("Aurora Pines"),
    release_date: nullableString("2026-02-14"),
    year: nullableInt64(2026),
    total_tracks: nullableInt64(3),
    cover: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  tracks: [
    makeTrack({ id: 101, title: "Northern Drift", track_index: 1, disc: 1, duration: 214_000 }),
    makeTrack({ id: 102, title: "Cold Current", track_index: 2, disc: 1, duration: 198_000 }),
    makeTrack({ id: 103, title: "Second Disc Opener", track_index: 1, disc: 2, duration: 245_000 }),
  ],
  artists: [
    { id: 7, name: "Aurora Pines", thumb: nullableString() },
  ],
  track_genres: [
    { track_id: 101, genre_id: 1, tag: "Ambient" },
    { track_id: 102, genre_id: 2, tag: "Electronic" },
  ],
  album_genres: ["Ambient", "Electronic"],
  total_duration: 657_000,
};

const EMPTY_ALBUM_ID = 43;

const emptyAlbumDetails = {
  album: {
    ...albumDetails.album,
    id: EMPTY_ALBUM_ID,
    title: "Silent Sessions",
    sort_title: "Silent Sessions",
    total_tracks: nullableInt64(0),
  },
  tracks: [],
  artists: albumDetails.artists,
  track_genres: [],
  album_genres: [],
  total_duration: 0,
};

const musicianDetails = {
  musician: {
    id: 7,
    name: "Aurora Pines",
    sort_name: "Aurora Pines",
    summary: nullableString("Aurora Pines makes ambient music."),
    thumb: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  albums: [],
  tracks: [],
  genres: ["Ambient"],
  total_duration: 0,
};

async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function mockAlbumDetailsApi(page: Page, { isAdmin = true }: { isAdmin?: boolean } = {}) {
  const unexpectedApiRequests: string[] = [];
  const likedTrackIds = new Set([101]);

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Album User",
          email: "album@example.com",
          is_admin: isAdmin,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/notifications/unread-count" && method === "GET") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

    if (url.pathname === `/api/music/albums/details/${ALBUM_ID}` && method === "GET") {
      await fulfillJSON(route, apiResponse(albumDetails));
      return;
    }

    if (url.pathname === `/api/music/albums/details/${EMPTY_ALBUM_ID}` && method === "GET") {
      await fulfillJSON(route, apiResponse(emptyAlbumDetails));
      return;
    }

    if (url.pathname === "/api/music/musicians/7" && method === "GET") {
      await fulfillJSON(route, apiResponse(musicianDetails));
      return;
    }

    if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [...likedTrackIds] }));
      return;
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
      return;
    }

    // Starting playback makes the audio element request the stream; playback
    // itself is irrelevant here, the request just must not count as unexpected.
    if (/^\/api\/music\/tracks\/\d+\/stream$/.test(url.pathname) && method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "audio/flac",
        body: Buffer.alloc(0),
      });
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
  });

  return unexpectedApiRequests;
}

async function expectElementInsideViewport(page: Page, locator: Locator, label: string) {
  const viewport = page.viewportSize();
  const box = await locator.boundingBox();

  expect(box, `${label} should have a layout box`).not.toBeNull();
  expect(viewport, "viewport should be set before measuring layout").not.toBeNull();

  if (!box || !viewport) return;

  expect(box.x, `${label} left edge`).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width, `${label} right edge`).toBeLessThanOrEqual(viewport.width);
}

const breakpoints = [
  { label: "mobile", width: 375, height: 812 },
  { label: "tablet", width: 768, height: 1024 },
  { label: "desktop", width: 1280, height: 900 },
];

test("album details renders hero, tracklist, and details without console issues", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${ALBUM_ID}`);

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

  // Album details section.
  await expect(page.getByRole("heading", { name: "Album Details" })).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("audio player like button toggles the current track's liked state", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${ALBUM_ID}`);

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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("artist links navigate to the musician details page", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${ALBUM_ID}`);

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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
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

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("album details stays inside the viewport across breakpoints", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize(breakpoints[0]);
  await page.goto(`/music/album/${ALBUM_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  for (const bp of breakpoints) {
    await page.setViewportSize({ width: bp.width, height: bp.height });

    const playAlbum = page.getByRole("button", { name: "Play Album", exact: true });
    const shuffle = page.getByRole("button", { name: "Shuffle play album" });
    await expect(playAlbum).toBeVisible();
    await expect(shuffle).toBeVisible();

    await expectPageHasNoHorizontalScroll(page);
    await expectElementInsideViewport(page, playAlbum, `${bp.label} Play Album button`);
    await expectElementInsideViewport(page, shuffle, `${bp.label} Shuffle button`);
    await expectNoHorizontalOverflow(
      page.getByRole("heading", { level: 1, name: "Glacier Sessions" }),
      `${bp.label} title`,
    );

    // Screenshot artifact for manual visual review at each breakpoint.
    await page.screenshot({ path: `test-results/album-details-${bp.label}.png`, fullPage: true });
  }

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("non-admin users do not see the delete album menu", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page, { isAdmin: false });

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/album/${ALBUM_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Album", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "More options" })).toHaveCount(0);

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});
