import { expect, test } from "./e2e-test";
import type { Page } from "@playwright/test";
import type { TrailerPreferencesData } from "../src/types";
import { getApiData, putApiData } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";
import { requireMockApi } from "./e2e-env";
import { MOCK_MOVIE_ID } from "./fixtures/movies";
import { openHeldPlayer, playButton } from "./media-e2e-helpers";
import { mockYouTubePlayer } from "./mock-youtube-player";

// Drives the trailer pre-roll against the mock API server with the fake
// YouTube IFrame API installed, so nothing reaches YouTube. The movie's
// stream and HLS requests are held, which also proves the pre-roll never asks
// for the movie while trailers play.

requireMockApi();

const movie = { kind: "movie", id: MOCK_MOVIE_ID } as const;
const playUrl = `/movies/${MOCK_MOVIE_ID}/play?mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0`;
const preferencesPath = "/api/user/preferences/trailers";

function saveTrailerPreferences(page: Page, prefs: TrailerPreferencesData) {
  return putApiData(page.context().request, preferencesPath, prefs);
}

const prerollRegion = (page: Page) =>
  page.getByRole("region", { name: "Trailers before the movie" });

test.beforeEach(async ({ page }) => {
  await loginPageViaApi(page);
});

test.describe("Trailer pre-roll", () => {
  test("plays muted and offers Unmute when the browser blocks autoplay", async ({
    page,
  }) => {
    await saveTrailerPreferences(page, { enabled: true, count: 2, source: "both" });
    await mockYouTubePlayer(page, { autoplayBlocked: true });
    const mediaRequests = await openHeldPlayer(page, movie, playUrl);

    const region = prerollRegion(page);
    await expect(region).toBeVisible();
    // The muted retry played the trailer, so the control reads Pause.
    await expect(
      page.getByRole("button", { name: "Pause trailer (Space or K)" }),
    ).toBeVisible();
    const unmute = page.getByRole("button", { name: "Unmute trailer (M)" });
    await expect(unmute).toBeVisible();

    await unmute.click();
    await expect(unmute).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Skip trailer (N)" })).toBeFocused();
    expect(mediaRequests).toEqual([]);
  });

  test("plays the queued trailers before the movie without touching the stream", async ({
    page,
  }) => {
    await saveTrailerPreferences(page, { enabled: true, count: 2, source: "both" });
    await mockYouTubePlayer(page);
    const mediaRequests = await openHeldPlayer(page, movie, playUrl);

    const region = prerollRegion(page);
    await expect(region).toBeVisible();
    await expect(region).toContainText("Trailer 1 of 2");
    await expect(region).toContainText("Glacier Run");
    await expect(page.locator("video")).toHaveCount(0);
    const skip = page.getByRole("button", { name: "Skip trailer (N)" });
    await expect(skip).toBeFocused();
    // The fake player stays cued until told to play; the real one autoplays.
    await expect(
      page.getByRole("button", { name: /^(Play|Pause) trailer \(Space or K\)$/ }),
    ).toBeVisible();
    expect(mediaRequests).toEqual([]);

    await skip.click();
    await expect(region).toContainText("Trailer 2 of 2");
    await expect(region).toContainText("North Light");
    expect(mediaRequests).toEqual([]);

    await page.getByRole("button", { name: "Start movie (S)" }).click();
    await expect(region).toHaveCount(0);
    await expect(playButton(page)).toBeVisible();
    // Mounting the movie is what asks for its stream, exactly once.
    await expect.poll(() => mediaRequests.length).toBe(1);
    expect(new URL(mediaRequests[0]).pathname).toBe(
      `/api/movies/${MOCK_MOVIE_ID}/hls/720p_3mbps/playlist.m3u8`,
    );
  });

  test("answers the keyboard: N skips a trailer and S starts the movie", async ({
    page,
  }) => {
    await saveTrailerPreferences(page, { enabled: true, count: 2, source: "library" });
    await mockYouTubePlayer(page);
    await openHeldPlayer(page, movie, playUrl);

    const region = prerollRegion(page);
    await expect(region).toContainText("Trailer 1 of 2");

    await page.keyboard.press("n");
    await expect(region).toContainText("Trailer 2 of 2");

    await page.keyboard.press("s");
    await expect(region).toHaveCount(0);
    await expect(playButton(page)).toBeVisible();
  });

  test("goes straight to the movie while the feature is off", async ({ page }) => {
    expect(
      (await getApiData<TrailerPreferencesData>(page.context().request, preferencesPath))
        .enabled,
    ).toBe(false);

    await mockYouTubePlayer(page);
    const mediaRequests = await openHeldPlayer(page, movie, playUrl);

    await expect(playButton(page)).toBeVisible();
    await expect(prerollRegion(page)).toHaveCount(0);
    await expect.poll(() => mediaRequests.length).toBe(1);
  });
});
