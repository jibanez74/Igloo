import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { apiURL, readE2EEnv } from "./e2e-env";

// Drives the movie play route against the mock API server. The stream request
// is intentionally left pending: the player chrome reaches its ready state
// from the metadata queries alone, and with the media stuck at HAVE_NOTHING a
// seek sets the element's default playback start position, which currentTime
// reads back — enough to prove the chapter-seek flow without real media.
async function openMoviePlayer(page: Page) {
  const env = readE2EEnv();
  const loginResponse = await page.context().request.post(
    apiURL(env, "/api/auth/login"),
    {
      data: { email: env.email, password: env.password },
      failOnStatusCode: false,
    },
  );
  expect(loginResponse.status()).toBe(200);

  await page.route("**/api/movies/*/stream*", () => {
    // Never fulfilled: keeps the player ready without firing a media error.
  });

  await page.goto("/movies/101/play?mode=direct&audio_track=0&start=0");

  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
}

test("chapter menu lists chapters with spoken labels and marks the current one", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page);

  const chapterTrigger = page.getByRole("button", {
    name: "Chapters, 2 chapters",
  });
  await expect(chapterTrigger).toBeVisible();
  await chapterTrigger.click();

  const firstChapter = page.getByRole("menuitem", {
    name: "Chapter 1 of 2, Opening Credits, starts at 0 seconds, current chapter",
  });
  await expect(firstChapter).toBeVisible();
  await expect(firstChapter).toHaveAttribute("aria-current", "true");

  const secondChapter = page.getByRole("menuitem", {
    name: "Chapter 2 of 2, The Journey, starts at 6 minutes 12 seconds",
  });
  await expect(secondChapter).toBeVisible();
  await expect(secondChapter).not.toHaveAttribute("aria-current", "true");

  await page.keyboard.press("Escape");
  browserIssues.assertClean();
});

test("selecting a chapter seeks to its start and announces the jump", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page);

  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await page
    .getByRole("menuitem", { name: /The Journey/ })
    .click();

  await expect
    .poll(() =>
      page
        .locator("video")
        .evaluate(video => (video as HTMLVideoElement).currentTime),
    )
    .toBe(372);

  // The announcement lands in an assertive sr-only live region (1px clipped),
  // so assert attachment rather than visibility.
  await expect(
    page.getByText("Jumped to chapter: The Journey"),
  ).toBeAttached();

  // Reopening the menu shows the active chapter marker moved to chapter 2.
  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await expect(
    page.getByRole("menuitem", {
      name: "Chapter 2 of 2, The Journey, starts at 6 minutes 12 seconds, current chapter",
    }),
  ).toHaveAttribute("aria-current", "true");
  await expect(
    page.getByRole("menuitem", { name: /Opening Credits/ }),
  ).not.toHaveAttribute("aria-current", "true");

  await page.keyboard.press("Escape");
  browserIssues.assertClean();
});
