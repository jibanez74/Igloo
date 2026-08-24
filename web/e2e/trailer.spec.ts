import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { mockYouTubePlayer } from "./mock-youtube-player";
import { MOVIE_SEEK_STEP_SEC } from "../src/lib/constants";

async function login(page: Page, env: E2EEnv) {
  const response = await page.context().request.post(
    apiURL(env, "/api/auth/login"),
    {
      data: {
        email: env.email,
        password: env.password,
      },
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(200);
}

async function expectTrailerChrome(page: Page) {
  await expect(page.getByRole("dialog", { name: "Trailer" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Close trailer (Escape)" }),
  ).toBeVisible();
  await expect(
    page.getByRole("slider", { name: "Seek through trailer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
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
  for (const { name, viewport, reducedMotion } of [
    {
      name: "desktop",
      viewport: { width: 1440, height: 900 },
      reducedMotion: "no-preference" as const,
    },
    {
      name: "mobile reduced motion",
      viewport: { width: 390, height: 844 },
      reducedMotion: "reduce" as const,
    },
  ]) {
    test(`renders labelled trailer controls on ${name}`, async ({ page }) => {
      const env = readE2EEnv();
      const browserIssues = trackBrowserIssues(page);

      await login(page, env);
      await mockYouTubePlayer(page);
      await page.emulateMedia({ reducedMotion });
      await page.setViewportSize(viewport);
      await page.goto(
        apiURL(
          env,
          "/trailer?videoKey=signal-fire-trailer&returnTo=/movies/101",
        ),
        { waitUntil: "domcontentloaded" },
      );

      await expectTrailerChrome(page);
      browserIssues.assertClean();
    });
  }

  test("keeps keyboard focus in the trailer dialog and closes on Escape", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const browserIssues = trackBrowserIssues(page);

    await login(page, env);
    await mockYouTubePlayer(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(
      apiURL(env, "/trailer?videoKey=signal-fire-trailer&returnTo=/"),
      { waitUntil: "domcontentloaded" },
    );

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

  test("lets Space activate the focused retry button after a playback error", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const browserIssues = trackBrowserIssues(page);

    await login(page, env);
    await mockYouTubePlayer(page, { failFirstLoad: true });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(
      apiURL(env, "/trailer?videoKey=signal-fire-trailer&returnTo=/"),
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByRole("dialog", { name: "Unable to Play Trailer" }),
    ).toBeVisible();

    const retryButton = page.getByRole("button", { name: "Try Again" });

    await retryButton.focus();
    await expect(retryButton).toBeFocused();
    await page.keyboard.press("Space");
    await expectTrailerChrome(page);
    browserIssues.assertClean();
  });

  test("keeps Space as a playback shortcut outside interactive controls", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const browserIssues = trackBrowserIssues(page);

    await login(page, env);
    await mockYouTubePlayer(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(
      apiURL(env, "/trailer?videoKey=signal-fire-trailer&returnTo=/"),
      { waitUntil: "domcontentloaded" },
    );

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
