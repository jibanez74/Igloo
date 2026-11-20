import { expect, test } from "./e2e-test";
import type { Page } from "@playwright/test";
import {
  BREAKPOINTS,
  VIEWPORTS,
  expectFlushUnderAppHeader,
  expectNoOverflowingElements,
  tabTo,
} from "./e2e-layout";
import { apiResponse, fulfillJSON } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  AURORA_PINES_ID,
  GLACIER_SESSIONS_ID,
  auroraPines,
  glacierSessions,
} from "./fixtures/music";

async function mockMusicianDetailsApi(page: Page) {
  await mockApi(page, {
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/music/musicians/${AURORA_PINES_ID}` && method === "GET") {
        await fulfillJSON(route, apiResponse(auroraPines));
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

      // Play all starts the queue; the audio itself is irrelevant here.
      if (/^\/api\/music\/tracks\/\d+\/stream$/.test(url.pathname) && method === "GET") {
        await route.fulfill({ status: 200, contentType: "audio/flac", body: Buffer.alloc(0) });
        return true;
      }

      return false;
    },
  });
}

test("skip links surface on keyboard focus and target the page sections", async ({ page }) => {
  await mockMusicianDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  const skipNav = page.getByRole("navigation", { name: "Skip to section" });
  const skipToDiscography = skipNav.getByRole("link", { name: "Skip to discography" });

  // Jump past the sidebar with the app-wide skip link first: its library
  // links preload their loaders on focus, which this mock does not serve.
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to page content" })).toBeFocused();
  await page.keyboard.press("Enter");

  // The section nav stays visually hidden until something inside it has focus,
  // and it follows the header controls in the tab order.
  await tabTo(page, skipToDiscography, 10);
  await expect(skipToDiscography).toBeVisible();

  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(
    new RegExp(`/music/musician/${AURORA_PINES_ID}#discography-heading$`),
  );
  await expect(page.getByRole("heading", { name: "Discography" })).toBeInViewport();
});

test("discography cards navigate to the album details page", async ({ page }) => {
  await mockMusicianDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  // Click the card's title line: the centered hover overlay is the play
  // button, so a center click would start playback instead of navigating.
  const albumLink = page.getByRole("link", { name: "Glacier Sessions, 2026 · 2 tracks" });
  await albumLink.getByRole("heading", { name: "Glacier Sessions" }).click();
  await expect(page).toHaveURL(`/music/album/${GLACIER_SESSIONS_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Glacier Sessions" })).toBeVisible();
});

test("musician details holds its layout at every breakpoint", async ({ page }) => {
  await mockMusicianDetailsApi(page);

  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  const title = page.getByRole("heading", { level: 1, name: "Aurora Pines" });
  await expect(title).toBeVisible();

  for (const { label, size } of BREAKPOINTS) {
    await test.step(label, async () => {
      await page.setViewportSize(size);

      await expect(title).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Play all 2 tracks by Aurora Pines", exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Shuffle play all 2 tracks by Aurora Pines", exact: true }),
      ).toBeVisible();
      await expectNoOverflowingElements(page);
      // The artwork band runs from the app header down, edge to edge.
      await expectFlushUnderAppHeader(
        page,
        page.locator('[data-slot="music-detail-band"]'),
      );
    });
  }
});

test("Play all queues the artist's tracks in order and the player steps through them", async ({
  page,
}) => {
  await mockMusicianDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(`/music/musician/${AURORA_PINES_ID}`);
  await expect(page.getByRole("heading", { level: 1, name: "Aurora Pines" })).toBeVisible();

  await page.getByRole("button", { name: "Play all 2 tracks by Aurora Pines", exact: true }).click();

  const player = page.getByRole("dialog");
  const counter = player.getByText(/^Track \d+ of \d+$/);
  await expect(counter).toHaveText("Track 1 of 2");
  await expect(player.getByRole("heading", { level: 1, name: "Northern Drift" })).toBeVisible();

  await player.getByRole("button", { name: "Next track" }).click();
  await expect(counter).toHaveText("Track 2 of 2");
  await expect(player.getByRole("heading", { level: 1, name: "Cold Current" })).toBeVisible();
  // The last track has nowhere further to go.
  await expect(player.getByRole("button", { name: "No next track" })).toBeDisabled();

  await player.getByRole("button", { name: "Previous track" }).click();
  await expect(counter).toHaveText("Track 1 of 2");
  await expect(player.getByRole("heading", { level: 1, name: "Northern Drift" })).toBeVisible();

  // A track that plays to its end moves the queue on by itself.
  await page.locator("audio").evaluate(audio => audio.dispatchEvent(new Event("ended")));
  await expect(counter).toHaveText("Track 2 of 2");
  await expect(player.getByRole("heading", { level: 1, name: "Cold Current" })).toBeVisible();
});
