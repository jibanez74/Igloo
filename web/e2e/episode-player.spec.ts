import { expect, test, type Page } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";
import {
  expectURLParams,
  holdMediaRequests,
  mediaApiPath,
  openHeldPlayer,
  playButton,
  trackStreamRequests,
} from "./media-e2e-helpers";
import {
  MOCK_EPISODE_ID,
  MOCK_NEXT_EPISODE_ID,
  MOCK_SHOW_ID,
} from "./fixtures/shows";

// Drives the episode play route against the mock API server, the way
// movie-player.spec.ts drives the movie route: the media requests are held
// pending so the player chrome reaches its ready state from the metadata
// queries alone. The mock server knows two episodes of show 401: 70103 (S1 E3)
// and 70104 (S1 E4), which 70103 hands off to.
const episode = { kind: "episode", id: MOCK_EPISODE_ID } as const;
const nextEpisode = { kind: "episode", id: MOCK_NEXT_EPISODE_ID } as const;
const playPath = `/tv-shows/${MOCK_SHOW_ID}/episodes/${MOCK_EPISODE_ID}/play`;
const nextPlayPath = `/tv-shows/${MOCK_SHOW_ID}/episodes/${MOCK_NEXT_EPISODE_ID}/play`;
const streamPath = `${mediaApiPath(episode)}/stream`;
const directSearch = "mode=direct&audio_track=0&subtitle_track=off&start=0";

requireMockApi();

test.beforeEach(async ({ page }) => {
  await loginPageViaApi(page);
});

function openEpisodePlayer(page: Page, search: string) {
  return openHeldPlayer(page, episode, `${playPath}?${search}`);
}

test("direct play titles the player and streams from the episode route", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  // An episode id sent down the movie route would still reach a server.
  const movieRouteRequests = trackStreamRequests(
    page,
    `${mediaApiPath({ kind: "movie", id: MOCK_EPISODE_ID })}/stream`,
  );
  const mediaRequests = await openEpisodePlayer(page, directSearch);

  await expect(playButton(page)).toBeVisible();
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

  await expect.poll(() => mediaRequests.length).toBeGreaterThan(0);
  expect(new URL(mediaRequests[0]).pathname).toBe(streamPath);
  expect(movieRouteRequests).toEqual([]);

  browserIssues.assertClean();
});

test("HLS mode requests the episode manifest with the shared query contract", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const mediaRequests = await openEpisodePlayer(
    page,
    "mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0",
  );

  await expect(playButton(page)).toBeVisible();
  await expect.poll(() => mediaRequests.length).toBeGreaterThan(0);

  const manifest = new URL(mediaRequests[0]);
  expect(manifest.pathname).toBe(
    `${mediaApiPath(episode)}/hls/720p_3mbps/playlist.m3u8`,
  );
  expect(manifest.searchParams.get("playback_session")).toMatch(
    /^[0-9a-f-]{36}$/i,
  );
  expect(manifest.searchParams.get("start")).toBe("0");
  expect(manifest.searchParams.get("audio_track")).toBe("0");
  // HLS never touches the raw stream route.
  expect(mediaRequests.filter(url => new URL(url).pathname === streamPath)).toEqual([]);

  browserIssues.assertClean();
});

// Playback never really runs here, so the end of the episode is the media
// element's own `ended` event, exactly the signal the player listens for.
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
  await holdMediaRequests(page, nextEpisode);
  await openEpisodePlayer(page, directSearch);
  await expect(playButton(page)).toBeVisible();
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
  await expect(page).toHaveURL(new RegExp(`${nextPlayPath}\\?`));
  await expectURLParams(page, { mode: "direct", autoplay: "true", start: "0" });
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Frost Harbor · S1 E4 · The Long Night",
    }),
  ).toBeVisible();
  await expect(page.getByRole("region", { name: "Up next" })).toHaveCount(0);

  browserIssues.assertClean();
});
