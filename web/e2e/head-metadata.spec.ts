import { expect, test, type Page } from "@playwright/test";

import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";

const BOOTSTRAP_DESCRIPTION =
  "Igloo is your personal media center for movies, TV Shows, music, personal videos, photos and so much more. Stream and organize your entire media library.";
const MOVIES_DESCRIPTION =
  "Browse and organize your personal movie collection in your Igloo media library.";
const SETTINGS_DESCRIPTION =
  "Configure your Igloo media center settings and preferences.";

async function readActiveHeadMetadata(page: Page) {
  return page.evaluate(() => ({
    description:
      document
        .querySelector('meta[name="description"]')
        ?.getAttribute("content") ?? null,
    title: document.title,
  }));
}

test("restores bootstrap metadata on routes without page-specific head tags", async ({
  page,
}) => {
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
      title: "Movies - Igloo",
    });

  // A client-side navigation: a full load would restore the bootstrap tags
  // from index.html no matter what the router does.
  const playLink = page.getByRole("link", { name: /^Play Signal Fire/ });
  await playLink.hover();
  await playLink.click();

  await expect(page).toHaveURL(/\/movies\/\d+\/play(\?|$)/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: BOOTSTRAP_DESCRIPTION,
      title: "Igloo",
    });

  await page.getByRole("link", { name: "Settings" }).click();

  await expect(page).toHaveURL(/\/settings$/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: SETTINGS_DESCRIPTION,
      title: "Settings - Igloo",
    });

  tracker.assertClean();
});
