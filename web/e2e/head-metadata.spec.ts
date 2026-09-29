import { expect, test, type Page } from "@playwright/test";

import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";

const ROOT_DESCRIPTION =
  "Igloo is your personal media center for movies, TV Shows, music, personal videos, photos and so much more. Stream and organize your entire media library.";
const MOVIES_DESCRIPTION =
  "Browse and organize your personal movie collection in your Igloo media library.";
const SETTINGS_DESCRIPTION =
  "Configure your Igloo media center settings and preferences.";

async function readActiveHeadMetadata(page: Page) {
  return page.evaluate(() => {
    const descriptions = document.head.querySelectorAll(
      'meta[name="description"]',
    );
    const robots = document.head.querySelectorAll('meta[name="robots"]');

    return {
      description: descriptions[0]?.getAttribute("content") ?? null,
      descriptionCount: descriptions.length,
      robots: [...robots].map(meta => meta.getAttribute("content")),
      title: document.title,
    };
  });
}

test("applies each route's head over the root defaults", async ({ page }) => {
  // Head tags are client-side only; the mock's direct-play movie keeps the
  // player from starting a transcode.
  requireMockApi();
  const tracker = trackBrowserIssues(page);

  await loginPageViaApi(page);
  await page.route("**/api/movies/*/stream*", () => {
    // Never fulfilled: the player stays ready without a media error.
  });
  await page.goto("/movies");

  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: MOVIES_DESCRIPTION,
      descriptionCount: 1,
      robots: ["noindex, nofollow"],
      title: "Movies - Igloo",
    });

  // Client-side navigations from here on: a full load would start from
  // index.html no matter what the router does.
  const playLink = page.getByRole("link", { name: /^Play Signal Fire/ });
  await playLink.hover();
  await playLink.click();

  // The player sets only a title, so the root description comes back.
  await expect(page).toHaveURL(/\/movies\/\d+\/play(\?|$)/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: ROOT_DESCRIPTION,
      descriptionCount: 1,
      robots: ["noindex, nofollow"],
      title: "Playing Signal Fire - Igloo",
    });

  // A settings subpage names itself and inherits the layout's description.
  await page.getByRole("link", { name: "Settings" }).click();

  await expect(page).toHaveURL(/\/settings$/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: SETTINGS_DESCRIPTION,
      descriptionCount: 1,
      robots: ["noindex, nofollow"],
      title: "General Settings - Igloo",
    });

  tracker.assertClean();
});
