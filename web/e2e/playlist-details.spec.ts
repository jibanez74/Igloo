import { expect, test, type Page } from "@playwright/test";
import type { components } from "../src/types/openapi.gen";
import { PLAYLIST_TRACKS_PAGE_SIZE } from "../src/lib/constants";
import { assertMockSuiteClean, trackBrowserIssues } from "./e2e-browser-issues";
import { apiResponse, fulfillJSON, gateRoute, nullableInt64, nullableString } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";

type Schema = components["schemas"];

const PLAYLIST_ID = 55;
// Deliberately more than one page: the header buttons used to queue only the
// pages the virtual list had scrolled in, so a playlist this size shuffled just
// its first page while the button promised all 120.
const TOTAL_TRACKS = 120;
const TRACKS_PATH = `/api/music/playlists/${PLAYLIST_ID}/tracks`;
const PAGE_OFFSETS = Array.from(
  { length: Math.ceil(TOTAL_TRACKS / PLAYLIST_TRACKS_PAGE_SIZE) },
  (_, index) => index * PLAYLIST_TRACKS_PAGE_SIZE,
);

function playlistTrack(position: number): Schema["PlaylistTrack"] {
  const id = 1000 + position;

  return {
    playlist_track_id: 9000 + position,
    position,
    added_at: "2026-01-01T00:00:00Z",
    added_by: nullableInt64(1),
    id,
    title: `Track ${position}`,
    duration: 200_000,
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
    movie_id: nullableInt64(),
    content_type: "track",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  } satisfies Schema["Playlist"],
  track_count: TOTAL_TRACKS,
  duration: TOTAL_TRACKS * 200_000,
  is_owner: true,
  can_edit: true,
  collaborators: null,
};

type MockOptions = {
  // Serve an error envelope for this offset, as a mid-drain network blip does.
  failAtOffset?: number;
};

async function mockPlaylistApi(page: Page, options: MockOptions = {}) {
  const trackPageRequests: number[] = [];

  const { unexpectedApiRequests } = await mockApi(page, {
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/music/playlists/${PLAYLIST_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(playlistDetails));
        return true;
      }

      if (url.pathname === TRACKS_PATH && method === "GET") {
        const offset = Number(url.searchParams.get("offset") ?? 0);
        const limit = Number(url.searchParams.get("limit") ?? PLAYLIST_TRACKS_PAGE_SIZE);
        const slice = allPlaylistTracks.slice(offset, offset + limit);

        trackPageRequests.push(offset);

        if (offset === options.failAtOffset) {
          await fulfillJSON(
            route,
            { error: true, message: "Failed to fetch playlist tracks" },
            500,
          );
          return true;
        }

        await fulfillJSON(route, apiResponse({
          tracks: slice,
          total: TOTAL_TRACKS,
          has_more: offset + slice.length < TOTAL_TRACKS,
          next_offset: offset + slice.length,
        }));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
        return true;
      }

      // Playback starts as soon as a queue is built; the stream itself is
      // irrelevant here, it just must not count as an unexpected request.
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

  return { unexpectedApiRequests, trackPageRequests };
}

function trackCounter(page: Page) {
  return page.getByText(/^Track \d+ of \d+$/);
}

async function openPlaylist(page: Page) {
  await page.goto(`/music/playlist/${PLAYLIST_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Long Haul" })).toBeVisible();
}

test("shuffle queues every track in a multi-page playlist and starts off the first one", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, trackPageRequests } = await mockPlaylistApi(page);

  await openPlaylist(page);

  const shuffle = page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` });
  await expect(shuffle).toBeVisible();

  // Only the first page has been fetched at this point.
  await expect.poll(() => trackPageRequests).toEqual([0]);

  // Pin Math.random so the shuffle is deterministic: always drawing index 0
  // rotates the queue left by one, so "Track 1" moves to the end and "Track 2"
  // opens. Without a real shuffle the first track would still be "Track 1".
  await page.evaluate(() => {
    Math.random = () => 0;
  });

  await shuffle.click();

  // The counter's denominator is the real proof: the queue holds the whole
  // playlist, not just the page the list had loaded.
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);
  await expect(page.getByRole("heading", { level: 1, name: "Track 2" })).toBeVisible();

  // Track 2 -> position 2 -> "Album 2" / "Artist 2". The player used to show
  // the playlist's own name in both slots for every track in the queue.
  // exact, or these also match the dialog's own sr-only title ("Now playing:
  // Track 2 by Artist 2") - which, note, is itself the assertion in miniature.
  const player = page.getByRole("dialog");
  await expect(player.getByText("Album 2", { exact: true })).toBeVisible();
  await expect(player.getByText("Artist 2", { exact: true })).toBeVisible();
  await expect(player.getByText("Long Haul", { exact: true })).toHaveCount(0);

  // Getting there required draining the remaining pages.
  expect(trackPageRequests).toEqual(PAGE_OFFSETS);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("play all queues every track too", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, trackPageRequests } = await mockPlaylistApi(page);

  await openPlaylist(page);

  await page.getByRole("button", { name: `Play all ${TOTAL_TRACKS} tracks` }).click();

  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);
  await expect(page.getByRole("heading", { level: 1, name: "Track 1" })).toBeVisible();
  expect(trackPageRequests).toEqual(PAGE_OFFSETS);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playback starts on the loaded page while the rest downloads behind it", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, trackPageRequests } = await mockPlaylistApi(page);
  // Hold the second page, so the test can watch playback start on what is
  // already loaded while the drain is still waiting on the network.
  const secondPage = await gateRoute(
    page,
    new RegExp(`${TRACKS_PATH}\\?.*\\boffset=${PLAYLIST_TRACKS_PAGE_SIZE}(&|$)`),
  );

  await openPlaylist(page);

  await page.getByRole("button", { name: `Play all ${TOTAL_TRACKS} tracks` }).click();

  // The first note must not wait on the drain: a long playlist is dozens of
  // sequential round trips, and the header buttons used to sit disabled behind
  // all of them.
  // (The header buttons are behind the expanded player at this point, so the
  // counter is what proves playback began on the first page alone.)
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${PLAYLIST_TRACKS_PAGE_SIZE}`);
  await expect.poll(secondPage.requested).toBe(true);

  // ...and the rest still lands.
  secondPage.release();
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${TOTAL_TRACKS}`);
  expect(trackPageRequests).toEqual(PAGE_OFFSETS);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("says so when a page of the drain fails instead of quietly playing a short queue", async ({
  page,
}) => {
  // The failed page is the point of the test, so its 500 is expected.
  const browserIssues = trackBrowserIssues(page, {
    ignoreConsole: (_type, text) =>
      text ===
      "Failed to load resource: the server responded with a status of 500 (Internal Server Error)",
    ignoreResponse: response => {
      const url = new URL(response.url());
      return (
        url.pathname === TRACKS_PATH &&
        url.searchParams.get("offset") === String(PLAYLIST_TRACKS_PAGE_SIZE)
      );
    },
  });
  const { unexpectedApiRequests } = await mockPlaylistApi(page, {
    failAtOffset: PLAYLIST_TRACKS_PAGE_SIZE,
  });

  await openPlaylist(page);

  await page.getByRole("button", { name: `Shuffle all ${TOTAL_TRACKS} tracks` }).click();

  // The button promised 120. Playing 50 without a word is the defect this whole
  // feature was audited for, so a failed page has to surface.
  await expect(page.getByText("Failed to shuffle playlist")).toBeVisible();
  await expect(
    page.getByText(`Only ${PLAYLIST_TRACKS_PAGE_SIZE} of ${TOTAL_TRACKS} tracks could be loaded.`),
  ).toBeVisible();
  await expect(trackCounter(page)).toHaveText(`Track 1 of ${PLAYLIST_TRACKS_PAGE_SIZE}`);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
