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
import {
  apiResponse,
  fulfillJSON,
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  AURORA_PINES_ID,
  GLACIER_SESSIONS_ID,
  auroraPines,
  glacierSessions,
} from "./fixtures/music";

const EMPTY_MUSICIAN_ID = 8;

const emptyMusicianDetails = {
  musician: {
    ...auroraPines.musician,
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

async function mockMusicianDetailsApi(page: Page) {
  const { unexpectedApiRequests } = await mockApi(page, {
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/music/musicians/${AURORA_PINES_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(auroraPines));
        return true;
      }

      if (url.pathname === `/api/music/musicians/${EMPTY_MUSICIAN_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(emptyMusicianDetails));
        return true;
      }

      if (url.pathname === `/api/music/albums/details/${GLACIER_SESSIONS_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(glacierSessions));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [101] }));
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("musician details renders hero, discography, and tracks without console issues", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();
  await expect(page.getByText("Aurora Pines makes ambient music.")).toBeVisible();

  // Genre badges and stat chips.
  await expect(page.getByRole("list", { name: "Genres: Ambient, Electronic" })).toBeVisible();
  const stats = page.getByRole("list", { name: "Musician statistics" });
  await expect(stats.getByText("1 album", { exact: true })).toBeVisible();
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
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("skip links surface on keyboard focus and target the page sections", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  const skipNav = page.getByRole("navigation", { name: "Skip to section" });
  const skipToDiscography = skipNav.getByRole("link", { name: "Skip to discography" });

  await skipToDiscography.focus();
  await expect(skipToDiscography).toBeVisible();
  await expect(skipNav.getByRole("link", { name: "Skip to musician info" })).toHaveAttribute("href", /#musician-name$/);
  await expect(skipNav.getByRole("link", { name: "Skip to all tracks" })).toHaveAttribute("href", /#tracks-heading$/);

  await skipToDiscography.click();
  await expect(page).toHaveURL(
    new RegExp(`/music/musician/${AURORA_PINES_ID}#discography-heading$`),
  );
  await expect(page.getByRole("heading", { name: "Discography" })).toBeInViewport();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("discography cards navigate to the album details page", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  const albumLink = page.getByRole("link", { name: "Glacier Sessions, 2026 · 2 tracks" });
  await albumLink.focus();
  await expect(albumLink).toBeFocused();

  // Click the card's title line: the centered hover overlay is the play
  // button, so a center click would start playback instead of navigating.
  await albumLink.getByRole("heading", { name: "Glacier Sessions" }).click();
  await expect(page).toHaveURL(`/music/album/${GLACIER_SESSIONS_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
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

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("musician details stays inside the viewport across breakpoints", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMusicianDetailsApi(page);

  await page.setViewportSize(BREAKPOINTS[0].size);
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  for (const bp of BREAKPOINTS) {
    await page.setViewportSize(bp.size);

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

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
