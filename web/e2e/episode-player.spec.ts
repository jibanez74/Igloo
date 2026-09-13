import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { readE2EEnv } from "./e2e-env";
import { loginWithCredentials } from "./media-e2e-helpers";

// Drives the episode play route against the mock API server, the way
// movie-player.spec.ts drives the movie route: the stream request is left
// pending so the player chrome reaches its ready state from the metadata
// queries alone. The mock server knows one episode (70103 of show 401).
const showId = 401;
const episodeId = 70103;
const playPath = `/tv-shows/${showId}/episodes/${episodeId}/play`;

async function openEpisodePlayer(page: Page, search: string) {
  await loginWithCredentials(page, readE2EEnv());

  await page.route("**/api/shows/episodes/*/stream*", () => {
    // Never fulfilled: keeps the player ready without firing a media error.
  });

  await page.goto(`${playPath}?${search}`);

  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
}

test("episode player titles itself after the show and episode", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  await openEpisodePlayer(
    page,
    "mode=direct&audio_track=0&subtitle_track=off&start=0",
  );

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Frost Harbor · S1 E3 · The Thaw",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", {
      name: "Video player for Frost Harbor · S1 E3 · The Thaw",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Chapters, 2 chapters" }),
  ).toBeVisible();

  browserIssues.assertClean();
});

test("loader redirect canonicalizes the episode's default settings", async ({
  page,
}) => {
  await openEpisodePlayer(page, "start=0");

  const url = new URL(page.url());
  expect(url.pathname).toBe(playPath);
  expect(url.searchParams.get("mode")).toBe("direct");
  expect(url.searchParams.get("audio_track")).toBe("0");
  expect(url.searchParams.get("subtitle_track")).toBe("off");
});

test("HLS mode requests the episode manifest with the shared query contract", async ({
  page,
}) => {
  await loginWithCredentials(page, readE2EEnv());

  const manifestRequests: string[] = [];
  await page.route("**/api/shows/episodes/*/hls/**", route => {
    manifestRequests.push(route.request().url());
    // Keep HLS pending; the chrome and the request shape are what is tested.
  });

  await page.goto(
    `${playPath}?mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0`,
  );
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();

  await expect.poll(() => manifestRequests.length).toBeGreaterThan(0);
  const manifest = new URL(manifestRequests[0]);
  expect(manifest.pathname).toBe(
    `/api/shows/episodes/${episodeId}/hls/720p_3mbps/playlist.m3u8`,
  );
  expect(manifest.searchParams.get("playback_session")).toMatch(
    /^[0-9a-f-]{36}$/i,
  );
  expect(manifest.searchParams.get("start")).toBe("0");
  expect(manifest.searchParams.get("audio_track")).toBe("0");
  expect(
    manifestRequests.some(url => new URL(url).pathname.endsWith("/stream")),
  ).toBe(false);
});

test("an unknown episode lands on the player's not-found screen", async ({
  page,
}) => {
  await loginWithCredentials(page, readE2EEnv());

  await page.goto(
    `/tv-shows/${showId}/episodes/999999/play?mode=direct&audio_track=0&subtitle_track=off&start=0`,
  );

  await expect(page.getByRole("alert")).toContainText("Episode not found");
  await expect(
    page.getByRole("button", { name: "Back to previous page" }),
  ).toBeVisible();
});
