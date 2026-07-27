import { expect, test, type Page } from "@playwright/test";
import { apiURL, readE2EEnv } from "./e2e-env";

// Drives the one-shot direct→remux fallback (audit D-FB): the mock movie is
// direct-play eligible (MP4, H.264 High 4.1, AAC LC — Chromium's canPlayType
// approves it), but the stream body is garbage, so the media element raises
// MEDIA_ERR_SRC_NOT_SUPPORTED / MEDIA_ERR_DECODE and the player must switch
// to remux exactly once, at the preserved position, with tracks intact.

async function logIn(page: Page) {
  const env = readE2EEnv();
  const loginResponse = await page
    .context()
    .request.post(apiURL(env, "/api/auth/login"), {
      data: { email: env.email, password: env.password },
      failOnStatusCode: false,
    });
  expect(loginResponse.status()).toBe(200);
}

test("failed direct play falls back to remux once at the preserved position", async ({
  page,
}) => {
  await logIn(page);

  const streamRequests: string[] = [];
  await page.route("**/api/movies/101/stream*", async route => {
    streamRequests.push(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "video/mp4",
      body: Buffer.from("this is not an mp4 file"),
    });
  });

  // The mock API server serves no HLS endpoints; keep the remux playlist
  // pending so the player stays mounted without real media.
  const playlistRequests: string[] = [];
  await page.route("**/api/movies/101/hls/*/playlist.m3u8*", route => {
    playlistRequests.push(route.request().url());
  });

  await page.goto(
    "/movies/101/play?mode=direct&audio_track=0&subtitle_track=off&start=120",
  );

  // The fallback rewrites the mode in place, preserving position and tracks.
  await expect
    .poll(() => new URL(page.url()).searchParams.get("mode"), {
      timeout: 15_000,
    })
    .toBe("remux");
  const url = new URL(page.url());
  expect(url.searchParams.get("start")).toBe("120");
  expect(url.searchParams.get("audio_track")).toBe("0");
  expect(url.searchParams.get("subtitle_track")).toBe("off");

  // The switch is announced, not silently swallowed.
  await expect(
    page.getByText(/can't be played directly by your browser/).first(),
  ).toBeVisible();

  // Exactly one direct attempt, one remux playlist request, and no bounce
  // back to direct.
  await expect.poll(() => playlistRequests.length).toBeGreaterThan(0);
  expect(
    playlistRequests.every(u =>
      new URL(u).pathname.endsWith("/hls/remux/playlist.m3u8"),
    ),
  ).toBe(true);
  expect(streamRequests.length).toBe(1);
  expect(new URL(page.url()).searchParams.get("mode")).toBe("remux");
});
