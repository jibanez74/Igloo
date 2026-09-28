import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll, VIEWPORTS } from "./e2e-layout";
import { playButton } from "./media-e2e-helpers";
import { mockYouTubePlayer } from "./mock-youtube-player";
import { MOVIE_SEEK_STEP_SEC } from "../src/lib/constants";
import { loginPageViaApi } from "./e2e-auth";

async function openTrailer(
  page: Page,
  options: Parameters<typeof mockYouTubePlayer>[1] = {},
) {
  await mockYouTubePlayer(page, options);
  await page.goto("/trailer?videoKey=signal-fire-trailer&returnTo=/", {
    waitUntil: "domcontentloaded",
  });
}

async function expectTrailerChrome(page: Page) {
  await expect(page.getByRole("dialog", { name: "Trailer" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Close trailer (Escape)" }),
  ).toBeVisible();
  await expect(
    page.getByRole("slider", { name: "Seek through trailer" }),
  ).toBeVisible();
  await expect(playButton(page)).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: `Rewind ${MOVIE_SEEK_STEP_SEC} seconds (J or Left Arrow)`,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: `Forward ${MOVIE_SEEK_STEP_SEC} seconds (L or Right Arrow)`,
    }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Mute (M)" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Fullscreen (F)" }),
  ).toBeVisible();
  await expectPageHasNoHorizontalScroll(page);
}

test.describe("Trailer playback chrome", () => {
  test.beforeEach(async ({ page }) => {
    await loginPageViaApi(page);
  });

  // The desktop layout is audited by the focus test below.
  test("renders labelled trailer controls on a phone", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);

    await page.setViewportSize(VIEWPORTS.phone);
    await openTrailer(page);

    await expectTrailerChrome(page);
    browserIssues.assertClean();
  });

  test("keeps keyboard focus in the trailer dialog and closes on Escape", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await page.setViewportSize(VIEWPORTS.desktop);
    await openTrailer(page);

    await expectTrailerChrome(page);

    const closeButton = page.getByRole("button", {
      name: "Close trailer (Escape)",
    });
    const fullscreenButton = page.getByRole("button", { name: "Fullscreen (F)" });

    await expect(closeButton).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(fullscreenButton).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(closeButton).toBeFocused();

    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/$/);
    browserIssues.assertClean();
  });

  test("focuses the retry button after a playback error and lets Space activate it", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await page.setViewportSize(VIEWPORTS.desktop);
    await openTrailer(page, { failFirstLoad: true });

    const errorDialog = page.getByRole("dialog", { name: "Unable to Play Trailer" });
    await expect(errorDialog).toBeVisible();
    await expect(page.getByRole("button", { name: "Try Again" })).toBeFocused();

    await page.keyboard.press("Space");
    await expect(errorDialog).toBeHidden();
    await expectTrailerChrome(page);
    await expect(
      page.getByRole("button", { name: "Close trailer (Escape)" }),
    ).toBeFocused();
    browserIssues.assertClean();
  });

  test("keeps Space as a playback shortcut outside interactive controls", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await page.setViewportSize(VIEWPORTS.desktop);
    await openTrailer(page);

    await expectTrailerChrome(page);

    await page.evaluate(() => {
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }
    });
    await page.keyboard.press("Space");
    await expect(
      page.getByRole("button", { name: "Pause (Space or K)" }),
    ).toBeVisible();
    browserIssues.assertClean();
  });
});
