import { expect, test, type Locator, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";

type NullableString = { String: string; Valid: boolean };
type NullableInt64 = { Int64: number; Valid: boolean };
type NullableFloat64 = { Float64: number; Valid: boolean };

function nullableString(value = ""): NullableString {
  return { String: value, Valid: value.length > 0 };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return { Int64: value ?? 0, Valid: value != null };
}

function nullableFloat64(value: number | null = null): NullableFloat64 {
  return { Float64: value ?? 0, Valid: value != null };
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
    duration: 200,
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
    spotify_id: nullableString("spotify-glacier"),
    spotify_popularity: nullableFloat64(73),
    release_date: nullableString("2026-02-14"),
    year: nullableInt64(2026),
    total_tracks: nullableInt64(3),
    cover: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  tracks: [
    makeTrack({ id: 101, title: "Northern Drift", track_index: 1, disc: 1, duration: 214 }),
    makeTrack({ id: 102, title: "Cold Current", track_index: 2, disc: 1, duration: 198 }),
    makeTrack({ id: 103, title: "Second Disc Opener", track_index: 1, disc: 2, duration: 245 }),
  ],
  artists: [
    { id: 7, name: "Aurora Pines", thumb: nullableString(), spotify_id: nullableString("sp-aurora") },
  ],
  track_genres: [
    { track_id: 101, genre_id: 1, tag: "Ambient" },
    { track_id: 102, genre_id: 2, tag: "Electronic" },
  ],
  album_genres: ["Ambient", "Electronic"],
  total_duration: 657,
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

    if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [101] }));
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

  // Album details section + Spotify popularity.
  await expect(page.getByRole("heading", { name: "Album Details" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Spotify popularity 73 out of 100" })).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
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
