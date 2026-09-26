import { expect, test, type Page } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { playButton, trackStreamRequests } from "./media-e2e-helpers";
import {
  MOCK_EPISODE_ID,
  MOCK_NEXT_EPISODE_ID,
  MOCK_SHOW_ID,
} from "./fixtures/shows";

// Drives the episode play route against the mock API server, the way
// movie-player.spec.ts drives the movie route: the stream request is left
// pending so the player chrome reaches its ready state from the metadata
// queries alone. The mock server knows one episode (70103 of show 401).
const showId = MOCK_SHOW_ID;
const episodeId = MOCK_EPISODE_ID;
const playPath = `/tv-shows/${showId}/episodes/${episodeId}/play`;

const streamPath = `/api/shows/episodes/${episodeId}/stream`;
const manifestPath = `/api/shows/episodes/${episodeId}/hls/720p_3mbps/playlist.m3u8`;

async function openEpisodePlayer(page: Page, search: string) {
  await loginPageViaApi(page);

  const streamRequests = trackStreamRequests(page, streamPath);
  await page.route("**/api/shows/episodes/*/stream*", () => {
    // Never fulfilled: keeps the player ready without firing a media error.
  });

  await page.goto(`${playPath}?${search}`);

  await expect(playButton(page)).toBeVisible();

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
  await loginPageViaApi(page);

  const streamRequests = trackStreamRequests(page, streamPath);
  const manifestRequests: string[] = [];
  await page.route("**/api/shows/episodes/*/hls/**", route => {
    // hls.js is happy with a pending manifest.
    manifestRequests.push(route.request().url());
  });

  await page.goto(
    `${playPath}?mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0`,
  );
  await expect(playButton(page)).toBeVisible();

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
  await loginPageViaApi(page);

  await page.goto(
    `/tv-shows/${showId}/episodes/999999/play?mode=direct&audio_track=0&subtitle_track=off&start=0`,
  );

  await expect(page.getByRole("alert")).toContainText("Episode not found");
  await expect(
    page.getByRole("button", { name: "Back to previous page" }),
  ).toBeVisible();
});

// The mock server hands 70103 off to 70104 (S1 E4). Playback never really
// runs here, so the end of the episode is the media element's own `ended`
// event, exactly the signal the player listens for.
const nextEpisodeId = MOCK_NEXT_EPISODE_ID;
const nextPlayPath = `/tv-shows/${showId}/episodes/${nextEpisodeId}/play`;

async function endEpisode(page: Page) {
  await page.evaluate(() => {
    const video = document.querySelector("video");
    if (!video) {
      throw new Error("no video element to end");
    }
    video.dispatchEvent(new Event("ended"));
  });
}

test("a finished episode offers the next one and hands off with autoplay", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  await openEpisodePlayer(
    page,
    "mode=direct&audio_track=0&subtitle_track=off&start=0",
  );
  await expect(page.getByRole("region", { name: "Up next" })).toHaveCount(0);

  await endEpisode(page);

  const card = page.getByRole("region", { name: "Up next" });
  await expect(card).toBeVisible();
  await expect(card).toContainText("S1 E4 · The Long Night");
  const playNow = page.getByRole("button", { name: "Play now" });
  await expect(playNow).toBeFocused();
  await playNow.click();

  // The loader resolves the new file's defaults with a redirect that keeps
  // the hand-off's own params.
  await expect(page).toHaveURL(new RegExp(`${nextPlayPath}\\?.*mode=direct`));
  const url = new URL(page.url());
  expect(url.searchParams.get("autoplay")).toBe("true");
  expect(url.searchParams.get("start")).toBe("0");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Frost Harbor · S1 E4 · The Long Night",
    }),
  ).toBeVisible();
  await expect(page.getByRole("region", { name: "Up next" })).toHaveCount(0);

  browserIssues.assertClean();
});

test("cancelling the up-next card keeps the finished episode", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  await openEpisodePlayer(
    page,
    "mode=direct&audio_track=0&subtitle_track=off&start=0",
  );

  await endEpisode(page);
  await page.getByRole("button", { name: "Cancel" }).click();

  await expect(page.getByRole("region", { name: "Up next" })).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`${playPath}\\?`));
  await expect(
    page.getByRole("region", {
      name: "Video player for Frost Harbor · S1 E3 · The Thaw",
    }),
  ).toBeFocused();
  // The transport chrome, which yields the bottom edge while the card
  // stands, is back.
  await expect(playButton(page)).toBeVisible();

  browserIssues.assertClean();
});
