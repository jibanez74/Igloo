import { expect, test, type Page, type Route } from "@playwright/test";
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

const MUSICIAN_ID = 7;
const ALBUM_ID = 42;

function makeMusicianTrack(overrides: Record<string, unknown>) {
  return {
    id: 0,
    title: "Untitled",
    sort_title: "Untitled",
    duration: 200_000,
    codec: "flac",
    bit_rate: 900_000,
    file_path: "/music/untitled.flac",
    track_index: nullableInt64(1),
    disc: nullableInt64(1),
    album_id: nullableInt64(ALBUM_ID),
    album_title: nullableString("Glacier Sessions"),
    album_cover: nullableString(),
    ...overrides,
  };
}

const musicianDetails = {
  musician: {
    id: MUSICIAN_ID,
    name: "Aurora Pines",
    sort_name: "Aurora Pines",
    summary: nullableString("Aurora Pines makes ambient music."),
    spotify_popularity: nullableFloat64(82),
    spotify_followers: nullableInt64(1_234_567),
    spotify_id: nullableString("sp-aurora"),
    thumb: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  albums: [
    {
      id: ALBUM_ID,
      title: "Glacier Sessions",
      cover: nullableString(),
      year: nullableInt64(2026),
      release_date: nullableString("2026-02-14"),
      spotify_popularity: nullableFloat64(73),
      track_count: 2,
    },
  ],
  tracks: [
    makeMusicianTrack({ id: 101, title: "Northern Drift", duration: 214_000 }),
    makeMusicianTrack({ id: 102, title: "Cold Current", duration: 198_000 }),
  ],
  genres: ["Ambient", "Electronic"],
  total_duration: 412_000,
};

const EMPTY_MUSICIAN_ID = 8;

const emptyMusicianDetails = {
  musician: {
    ...musicianDetails.musician,
    id: EMPTY_MUSICIAN_ID,
    name: "Silent Pines",
    sort_name: "Silent Pines",
    summary: nullableString(),
    spotify_popularity: nullableFloat64(null),
    spotify_followers: nullableInt64(null),
  },
  albums: [],
  tracks: [],
  genres: [],
  total_duration: 0,
};

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
    total_tracks: nullableInt64(0),
    cover: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  tracks: [],
  artists: [
    { id: MUSICIAN_ID, name: "Aurora Pines", thumb: nullableString(), spotify_id: nullableString("sp-aurora") },
  ],
  track_genres: [],
  album_genres: [],
  total_duration: 0,
};

async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function mockMusicianDetailsApi(page: Page) {
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Musician User",
          email: "musician@example.com",
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

    if (url.pathname === `/api/music/musicians/${MUSICIAN_ID}` && method === "GET") {
      await fulfillJSON(route, apiResponse(musicianDetails));
      return;
    }

    if (url.pathname === `/api/music/musicians/${EMPTY_MUSICIAN_ID}` && method === "GET") {
      await fulfillJSON(route, apiResponse(emptyMusicianDetails));
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

const breakpoints = [
  { label: "mobile", width: 375, height: 812 },
  { label: "tablet", width: 768, height: 1024 },
  { label: "desktop", width: 1280, height: 900 },
];

test("musician details renders hero, discography, and tracks without console issues", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${MUSICIAN_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();
  await expect(page.getByText("Aurora Pines makes ambient music.")).toBeVisible();

  // Genre badges and stat chips.
  await expect(page.getByRole("list", { name: "Genres: Ambient, Electronic" })).toBeVisible();
  const stats = page.getByRole("list", { name: "Musician statistics" });
  await expect(stats.getByText("1 album")).toBeVisible();
  await expect(stats.getByText("2 tracks")).toBeVisible();

  // Hero actions carry accessible names and keyboard focus.
  const playAll = page.getByRole("button", { name: "Play all 2 tracks by Aurora Pines", exact: true });
  await expect(playAll).toBeVisible();
  await expect(page.getByRole("button", { name: "Shuffle play all 2 tracks by Aurora Pines" })).toBeVisible();
  await playAll.focus();
  await expect(playAll).toBeFocused();

  // Spotify block renders when the fields are populated.
  await expect(page.getByRole("group", { name: "Spotify popularity 82 out of 100" })).toBeVisible();
  await expect(page.getByText("1.2M")).toBeVisible();

  // Discography uses the shared album card: full label, link, and play overlay.
  await expect(page.getByRole("heading", { name: "Discography" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Glacier Sessions, 2026 · 2 tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Glacier Sessions, 2026 · 2 tracks" })).toHaveCount(1);

  // Track list with liked state derived from liked-ids (track 101 is liked).
  await expect(page.getByRole("heading", { name: "All Tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Northern Drift" })).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Remove Northern Drift from liked" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Add Cold Current to liked" })).toBeVisible();

  await expect(page.getByRole("link", { name: "Back to Musicians library" })).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("skip links surface on keyboard focus and target the page sections", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${MUSICIAN_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  const skipNav = page.getByRole("navigation", { name: "Skip to section" });
  const skipToDiscography = skipNav.getByRole("link", { name: "Skip to discography" });

  await skipToDiscography.focus();
  await expect(skipToDiscography).toBeVisible();
  await expect(skipNav.getByRole("link", { name: "Skip to musician info" })).toHaveAttribute("href", /#musician-name$/);
  await expect(skipNav.getByRole("link", { name: "Skip to all tracks" })).toHaveAttribute("href", /#tracks-heading$/);

  await skipToDiscography.click();
  await expect(page).toHaveURL(new RegExp(`/music/musician/${MUSICIAN_ID}#discography-heading$`));
  await expect(page.getByRole("heading", { name: "Discography" })).toBeInViewport();

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("discography cards navigate to the album details page", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${MUSICIAN_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  const albumLink = page.getByRole("link", { name: "Glacier Sessions, 2026 · 2 tracks" });
  await albumLink.focus();
  await expect(albumLink).toBeFocused();

  // Click the card's title line: the centered hover overlay is the play
  // button, so a center click would start playback instead of navigating.
  await albumLink.getByRole("heading", { name: "Glacier Sessions" }).click();
  await expect(page).toHaveURL(`/music/album/${ALBUM_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("musician with no albums or tracks hides those sections and playback actions", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${EMPTY_MUSICIAN_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Silent Pines" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Discography" })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "All Tracks" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /Play all/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /Shuffle play/ })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Back to Musicians library" })).toBeVisible();

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});

test("musician details stays inside the viewport across breakpoints", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize(breakpoints[0]);
  await page.goto(`/music/musician/${MUSICIAN_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  for (const bp of breakpoints) {
    await page.setViewportSize({ width: bp.width, height: bp.height });

    const playAll = page.getByRole("button", { name: "Play all 2 tracks by Aurora Pines", exact: true });
    await expect(playAll).toBeVisible();

    await expectPageHasNoHorizontalScroll(page);
    await expectNoHorizontalOverflow(
      page.getByRole("heading", { level: 1, name: "Aurora Pines" }),
      `${bp.label} title`,
    );

    // Screenshot artifact for manual visual review at each breakpoint.
    await page.screenshot({ path: `test-results/musician-details-${bp.label}.png`, fullPage: true });
  }

  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});
