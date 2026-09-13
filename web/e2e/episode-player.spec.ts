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

const streamPath = `/api/shows/episodes/${episodeId}/stream`;
const manifestPath = `/api/shows/episodes/${episodeId}/hls/720p_3mbps/playlist.m3u8`;

// Every request for the episode's direct stream, whether or not a route
// handler answers it, so a test can prove the stream was (or was not) asked
// for.
function trackStreamRequests(page: Page) {
  const streamRequests: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === streamPath) {
      streamRequests.push(request.url());
    }
  });
  return streamRequests;
}

async function openEpisodePlayer(page: Page, search: string) {
  await loginWithCredentials(page, readE2EEnv());

  const streamRequests = trackStreamRequests(page);
  await page.route("**/api/shows/episodes/*/stream*", () => {
    // Never fulfilled: keeps the player ready without firing a media error.
  });

  await page.goto(`${playPath}?${search}`);

  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();

  return streamRequests;
}

test("episode player titles itself after the show and episode", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const streamRequests = await openEpisodePlayer(
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

  // Direct play asks the episode route, not the movie one, for the bytes.
  await expect.poll(() => streamRequests.length).toBeGreaterThan(0);
  expect(new URL(streamRequests[0]).pathname).toBe(streamPath);

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

  const streamRequests = trackStreamRequests(page);
  const usesNativeHls = await page.evaluate(() => {
    const video = document.createElement("video");
    return (
      video.canPlayType("application/vnd.apple.mpegurl") !== "" ||
      video.canPlayType("application/x-mpegURL") !== ""
    );
  });
  const manifestRequests: string[] = [];
  await page.route("**/api/shows/episodes/*/hls/**", async route => {
    manifestRequests.push(route.request().url());
    // hls.js is happy with a pending manifest; a native HLS engine needs a
    // real (empty) playlist or its metadata preflight never settles.
    if (usesNativeHls && route.request().resourceType() === "fetch") {
      await route.fulfill({
        status: 200,
        contentType: "application/vnd.apple.mpegurl",
        body: "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXT-X-ENDLIST\n",
      });
    }
  });

  await page.goto(
    `${playPath}?mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0`,
  );
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();

  await expect.poll(() => manifestRequests.length).toBeGreaterThan(0);
  const manifest = new URL(manifestRequests[0]);
  expect(manifest.pathname).toBe(manifestPath);
  expect(manifest.searchParams.get("playback_session")).toMatch(
    /^[0-9a-f-]{36}$/i,
  );
  expect(manifest.searchParams.get("start")).toBe("0");
  expect(manifest.searchParams.get("audio_track")).toBe("0");
  // HLS never touches the raw stream route.
  expect(streamRequests).toEqual([]);
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
