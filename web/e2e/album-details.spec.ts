import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { BREAKPOINTS, VIEWPORTS, expectNoOverflowingElements } from "./e2e-layout";
import { apiResponse, fulfillJSON } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  AURORA_PINES_ID,
  GLACIER_SESSIONS_ID,
  auroraPines,
  glacierSessions,
} from "./fixtures/music";

async function mockAlbumDetailsApi(page: Page) {
  const likedTrackIds = new Set([101]);

  const { unexpectedApiRequests } = await mockApi(page, {
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/music/albums/details/${GLACIER_SESSIONS_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(glacierSessions));
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

test("audio player like button toggles the current track's liked state", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
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
  await page.setViewportSize(VIEWPORTS.phone);
  await expect(
    miniBar.getByRole("button", { name: "Remove Northern Drift from liked" }),
  ).toBeVisible();
  await expectNoOverflowingElements(page);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("artist links navigate to the musician details page", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);

  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();

  // The hero name is the first of the artist links; the rest share its target.
  await page.getByRole("link", { name: "Aurora Pines" }).first().click();
  await expect(page).toHaveURL(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("album details holds its layout at every breakpoint", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockAlbumDetailsApi(page);

  await page.goto(`/music/album/${GLACIER_SESSIONS_ID}`);
  const title = page.getByRole("heading", { level: 1, name: "Glacier Sessions" });
  await expect(title).toBeVisible();

  for (const { label, size } of BREAKPOINTS) {
    await test.step(label, async () => {
      await page.setViewportSize(size);

      await expect(title).toBeVisible();
      await expect(page.getByRole("button", { name: "Play Album", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Shuffle play album" })).toBeVisible();
      await expectNoOverflowingElements(page);
    });
  }

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
