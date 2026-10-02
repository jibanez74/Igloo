import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll, VIEWPORTS } from "./e2e-layout";
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

// A touch-first device drops the keyboard hints from every control name and
// the shortcut list from the dialog description (design-system §1.7).
async function expectTrailerChrome(page: Page, { touch = false } = {}) {
  const control = (label: string, keys: string) =>
    page.getByRole("button", {
      name: touch ? label : `${label} (${keys})`,
      exact: true,
    });

  const dialog = page.getByRole("dialog", { name: "Trailer" });
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAccessibleDescription(
    touch
      ? ""
      : new RegExp(`^Keyboard shortcuts: .* rewind ${MOVIE_SEEK_STEP_SEC} seconds`),
  );
  await expect(control("Close trailer", "Escape")).toBeVisible();
  await expect(
    page.getByRole("slider", { name: "Seek through trailer" }),
  ).toBeVisible();
  await expect(control("Play", "Space or K")).toBeVisible();
  await expect(
    control(`Rewind ${MOVIE_SEEK_STEP_SEC} seconds`, "J or Left Arrow"),
  ).toBeVisible();
  await expect(
    control(`Forward ${MOVIE_SEEK_STEP_SEC} seconds`, "L or Right Arrow"),
  ).toBeVisible();
  await expect(control("Mute", "M")).toBeVisible();
  await expect(control("Fullscreen", "F")).toBeVisible();
  await expectPageHasNoHorizontalScroll(page);
}

test.describe("Trailer playback chrome", () => {
  test.beforeEach(async ({ page }) => {
    await loginPageViaApi(page);
  });

  // The desktop layout is audited by the focus test below.
  test.describe("on a touch phone", () => {
    test.use({ viewport: VIEWPORTS.phone, hasTouch: true, isMobile: true });

    test("renders labelled trailer controls without keyboard hints", async ({
      page,
    }) => {
      const browserIssues = trackBrowserIssues(page);

      await openTrailer(page);

      // The hints key off this query; fail loudly if emulation stops setting it.
      expect(
        await page.evaluate(
          () => matchMedia("(hover: none) and (pointer: coarse)").matches,
        ),
      ).toBe(true);
      await expectTrailerChrome(page, { touch: true });
      browserIssues.assertClean();
    });
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
