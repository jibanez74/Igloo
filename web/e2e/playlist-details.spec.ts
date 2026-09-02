import { expect, test, type Page } from "@playwright/test";
import { assertMockSuiteClean, trackBrowserIssues } from "./e2e-browser-issues";
import { apiResponse, fulfillJSON, nullableInt64, nullableString } from "./e2e-api";

const PLAYLIST_ID = 55;
const PAGE_SIZE = 50;
// Deliberately more than one page: the header buttons used to queue only the
// pages the virtual list had scrolled in, so a playlist this size shuffled just
// its first 50 tracks while the button promised all 120.
const TOTAL_TRACKS = 120;

function playlistTrack(position: number) {
  const id = 1000 + position;

  return {
    playlist_track_id: 9000 + position,
    position,
    added_at: "2026-01-01T00:00:00Z",
    added_by: nullableInt64(1),
    id,
    title: `Track ${position}`,
    duration: 200_000,
    file_path: `/music/track-${position}.flac`,
    codec: "flac",
    bit_rate: 900_000,
    album_id: nullableInt64(300 + (position % 3)),
    musician_id: nullableInt64(400 + (position % 3)),
    album_title: nullableString(`Album ${position % 3}`),
    album_cover: nullableString(),
    musician_name: nullableString(`Artist ${position % 3}`),
  };
}

const allPlaylistTracks = Array.from({ length: TOTAL_TRACKS }, (_, index) =>
  playlistTrack(index + 1),
);

const playlistDetails = {
  playlist: {
    id: PLAYLIST_ID,
    user_id: 1,
    name: "Long Haul",
    description: nullableString("A playlist that spans several pages."),
    cover_image: nullableString(),
    is_public: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  track_count: TOTAL_TRACKS,
  duration: TOTAL_TRACKS * 200_000,
  is_owner: true,
  can_edit: true,
  collaborators: null,
};

type MockOptions = {
  // Hold every page after the first for this long, so the test can watch
  // playback start on what is already loaded while the rest is still coming.
  pageDelayMs?: number;
  // Serve an error envelope for this offset, as a mid-drain network blip does.
  failAtOffset?: number;
};

async function mockPlaylistApi(page: Page, options: MockOptions = {}) {
  const unexpectedApiRequests: string[] = [];
  const trackPageRequests: number[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Playlist User",
          email: "playlist@example.com",
          is_admin: false,
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

    if (url.pathname === `/api/music/playlists/${PLAYLIST_ID}` && method === "GET") {
      await fulfillJSON(route, apiResponse(playlistDetails));
      return;
    }

    if (
      url.pathname === `/api/music/playlists/${PLAYLIST_ID}/tracks` &&
      method === "GET"
    ) {
      const offset = Number(url.searchParams.get("offset") ?? 0);
      const limit = Number(url.searchParams.get("limit") ?? PAGE_SIZE);
      const slice = allPlaylistTracks.slice(offset, offset + limit);

      trackPageRequests.push(offset);

      if (offset === options.failAtOffset) {
        await fulfillJSON(
          route,
          { error: true, message: "Failed to fetch playlist tracks" },
          500,
        );
        return;
      }

      if (offset > 0 && options.pageDelayMs) {
        await new Promise(resolve => setTimeout(resolve, options.pageDelayMs));
      }

      await fulfillJSON(route, apiResponse({
        tracks: slice,
        total: TOTAL_TRACKS,
        has_more: offset + slice.length < TOTAL_TRACKS,
        next_offset: offset + slice.length,
      }));
      return;
    }

    if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
      return;
    }

    // Playback starts as soon as a queue is built; the stream itself is
    // irrelevant here, it just must not count as an unexpected request.
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

  return { unexpectedApiRequests, trackPageRequests };
}

function trackCounter(page: Page) {
  return page.getByText(/^Track \d+ of \d+$/);
}

test("shuffle queues every track in a multi-page playlist", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, trackPageRequests } = await mockPlaylistApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/playlist/${PLAYLIST_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  const shuffle = page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` });
  await expect(shuffle).toBeVisible();

  // Only the first page has been fetched at this point.
  expect(trackPageRequests).toEqual([0]);

  await shuffle.click();

  // The counter's denominator is the real proof: the queue holds the whole
  // playlist, not just the page the list had loaded.
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);

  // Getting there required draining the remaining pages.
  expect(trackPageRequests).toEqual([0, 50, 100]);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("shuffle starts somewhere other than the playlist's first track", async ({ page }) => {
  await mockPlaylistApi(page);

  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  // Pin Math.random so the shuffle is deterministic: always drawing index 0
  // rotates the queue left by one, so "Track 1" moves to the end and "Track 2"
  // opens. Without a real shuffle the first track would still be "Track 1".
  await page.evaluate(() => {
    Math.random = () => 0;
  });

  await page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` }).click();

  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);
  await expect(
    page.getByRole("heading", { level: 1, name: "Track 2" }),
  ).toBeVisible();
});

test("each shuffled track keeps its own artist rather than the playlist name", async ({ page }) => {
  await mockPlaylistApi(page);

  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  await page.evaluate(() => {
    Math.random = () => 0;
  });

  await page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` }).click();

  // Track 2 -> position 2 -> "Album 2" / "Artist 2". The player used to show
  // the playlist's own name in both slots for every track in the queue.
  // exact, or these also match the dialog's own sr-only title ("Now playing:
  // Track 2 by Artist 2") - which, note, is itself the assertion in miniature.
  const player = page.getByRole("dialog");
  await expect(player.getByText("Album 2", { exact: true })).toBeVisible();
  await expect(player.getByText("Artist 2", { exact: true })).toBeVisible();
  await expect(player.getByText("Long Haul", { exact: true })).toHaveCount(0);
});

test("play all queues every track too", async ({ page }) => {
  const { trackPageRequests } = await mockPlaylistApi(page);

  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  await page.getByRole("button", { name: `Play all ${TOTAL_TRACKS} tracks` }).click();

  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);
  await expect(page.getByRole("heading", { level: 1, name: "Track 1" })).toBeVisible();
  expect(trackPageRequests).toEqual([0, 50, 100]);
});

test("playback starts on the loaded page while the rest downloads behind it", async ({
  page,
}) => {
  const { trackPageRequests } = await mockPlaylistApi(page, { pageDelayMs: 1500 });

  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  await page.getByRole("button", { name: `Play all ${TOTAL_TRACKS} tracks` }).click();

  // The first note must not wait on the drain: a long playlist is dozens of
  // sequential round trips, and the header buttons used to sit disabled behind
  // all of them.
  // (The header buttons are behind the expanded player at this point, so the
  // counter is what proves playback began on the first page alone.)
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${PAGE_SIZE}`);

  // ...and the rest still lands.
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`, {
    timeout: 15_000,
  });
  expect(trackPageRequests).toEqual([0, 50, 100]);
});

test("says so when a page of the drain fails instead of quietly playing a short queue", async ({
  page,
}) => {
  await mockPlaylistApi(page, { failAtOffset: 50 });

  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();

  await page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` }).click();

  // The button promised 120. Playing 50 without a word is the defect this whole
  // feature was audited for, so a failed page has to surface.
  await expect(page.getByText("Failed to shuffle playlist")).toBeVisible();
  await expect(
    page.getByText(`Only ${PAGE_SIZE} of ${TOTAL_TRACKS} tracks could be loaded.`),
  ).toBeVisible();
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${PAGE_SIZE}`);
});
