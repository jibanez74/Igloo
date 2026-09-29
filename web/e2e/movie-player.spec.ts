import { expect, test, type Page, type Route } from "@playwright/test";
import { PLAYBACK_PREFERENCES_STORAGE_PREFIX } from "../src/lib/playback-preferences";
import type { MovieTechnicalDetailsResponse } from "../src/types";
import { gateRoute, nullableString, readJSON } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";
import {
  expectURLParams,
  openHeldPlayer,
  playButton,
} from "./media-e2e-helpers";
import { MOCK_MOVIE_ID } from "./fixtures/movies";

// Drives the movie play route against the mock API server. The stream and HLS
// requests are held pending: the player chrome reaches its ready state from
// the metadata queries alone, and with the media stuck at HAVE_NOTHING a seek
// sets the element's default playback start position, which currentTime
// reads back — enough to prove the chapter-seek flow without real media.

const movie = { kind: "movie", id: MOCK_MOVIE_ID } as const;
const moviePath = `/movies/${MOCK_MOVIE_ID}`;
const movieApiPath = `/api/movies/${MOCK_MOVIE_ID}`;

requireMockApi();

test.beforeEach(async ({ page }) => {
  await loginPageViaApi(page);
});

function openMoviePlayer(
  page: Page,
  search: string,
  options?: Parameters<typeof openHeldPlayer>[3],
) {
  return openHeldPlayer(page, movie, `${moviePath}/play?${search}`, options);
}

/**
 * Holds the technical details until `release()`, then answers with the mock
 * server's body after `mutate` has edited it.
 */
function gateTechnicalDetails(
  page: Page,
  mutate: (details: MovieTechnicalDetailsResponse) => void,
) {
  return gateRoute(page, `**${movieApiPath}/technical-details`, async (route: Route) => {
    const response = await route.fetch();
    const body = await readJSON<MovieTechnicalDetailsResponse>(response);
    mutate(body.data!);
    await route.fulfill({ response, json: body });
  });
}

/** While a gate holds, the player shows its placeholder and asks for no media. */
async function expectPreparingPlayback(page: Page, mediaRequests: string[]) {
  await expect(page.getByText("Preparing playback...")).toBeVisible();
  await expect(page.locator("video")).toHaveCount(0);
  expect(mediaRequests).toEqual([]);
}

for (const { label, mode, expectedRequestPath } of [
  {
    label: "direct",
    mode: "direct",
    expectedRequestPath: `${movieApiPath}/stream`,
  },
  {
    label: "HLS",
    mode: "720p_3mbps",
    expectedRequestPath: `${movieApiPath}/hls/720p_3mbps/playlist.m3u8`,
  },
]) {
  test(`${label} playback waits for the server catalog when device speed needs it`, async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    // Device language/profile preferences are synchronous. A configured speed
    // with no stored profile is the path that needs the asynchronous catalog.
    await page.addInitScript(key => {
      localStorage.setItem(key, JSON.stringify({
        preferredProfile: null,
        downloadMbps: 5,
        preferredAudioLanguage: null,
        preferredSubtitleLanguage: null,
      }));
    }, `${PLAYBACK_PREFERENCES_STORAGE_PREFIX}1`);

    const playbackSettings = await gateRoute(page, "**/api/settings/playback");
    const mediaRequests = await openMoviePlayer(page, `mode=${mode}&audio_track=0&start=0`);

    await expect.poll(playbackSettings.requested).toBe(true);
    await expectPreparingPlayback(page, mediaRequests);

    playbackSettings.release();

    await expect.poll(() => mediaRequests.length).toBe(1);
    expect(new URL(mediaRequests[0]).pathname).toBe(expectedRequestPath);
    browserIssues.assertClean();
  });
}

for (const { label, search } of [
  { label: "an omitted subtitle track", search: "mode=direct&audio_track=0&start=0" },
  { label: "the loader's default-settings redirect", search: "start=0" },
]) {
  test(`${label} settles on the canonical subtitle-off URL`, async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);

    await openMoviePlayer(page, search);

    await expect(playButton(page)).toBeVisible();
    await expectURLParams(page, {
      mode: "direct",
      audio_track: "0",
      subtitle_track: "off",
    });
    browserIssues.assertClean();
  });
}

test("cold non-first audio waits for metadata and never requests the raw stream", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  const technicalDetails = await gateTechnicalDetails(page, details => {
    details.audio_streams.push({
      ...details.audio_streams[0],
      id: 2,
      stream_index: 2,
      language: nullableString("spa"),
      title: nullableString("Spanish"),
    });
  });
  const mediaRequests = await openMoviePlayer(
    page,
    "mode=direct&audio_track=1&subtitle_track=off&start=0",
  );

  await expect.poll(technicalDetails.requested).toBe(true);
  await expectPreparingPlayback(page, mediaRequests);

  technicalDetails.release();

  // Direct play can only deliver the first audio track, so the player goes
  // straight to remux instead of asking for the raw file first.
  await expect.poll(() => mediaRequests.length).toBe(1);
  const requestUrl = new URL(mediaRequests[0]);
  expect(requestUrl.pathname).toBe(`${movieApiPath}/hls/remux/playlist.m3u8`);
  expect(requestUrl.searchParams.get("audio_track")).toBe("1");
  browserIssues.assertClean();
});

test("cold direct deep link to an ineligible file never requests the raw stream", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  // A bookmarked ?mode=direct link to an MKV must wait for the technical
  // details to learn the browser cannot play it, rather than optimistically
  // asking for the raw file.
  const technicalDetails = await gateTechnicalDetails(page, details => {
    details.movie.container = "mkv";
    details.movie.mime_type = "video/x-matroska";
  });
  const mediaRequests = await openMoviePlayer(
    page,
    "mode=direct&audio_track=0&subtitle_track=off&start=0",
  );

  await expect.poll(technicalDetails.requested).toBe(true);
  await expectPreparingPlayback(page, mediaRequests);

  technicalDetails.release();

  await expectURLParams(page, { mode: "remux" });
  await expect.poll(() => mediaRequests.length).toBe(1);
  expect(new URL(mediaRequests[0]).pathname).toBe(
    `${movieApiPath}/hls/remux/playlist.m3u8`,
  );
  // The pre-emptive mode resolution is silent: it must not read as an
  // error-driven fallback announcement.
  await expect(page.getByText(/can't be played directly/)).toHaveCount(0);
  browserIssues.assertClean();
});

test("failed direct play falls back to remux once at the preserved position", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  // The mock movie is direct-play eligible (MP4, H.264 High 4.1, AAC LC —
  // Chromium's canPlayType approves it), but the bytes are not a video, so the
  // media element raises a source or decode error and the player must switch
  // to remux exactly once, keeping the position and tracks.
  const mediaRequests = await openMoviePlayer(
    page,
    "mode=direct&audio_track=0&subtitle_track=off&start=120",
    { streamBody: "this is not an mp4 file" },
  );

  await expectURLParams(page, {
    mode: "remux",
    start: "120",
    audio_track: "0",
    subtitle_track: "off",
  });

  // The switch is announced, not silently swallowed.
  await expect(
    page
      .getByRole("region", { name: /^Notifications/ })
      .getByText(/can't be played directly by your browser/),
  ).toBeVisible();

  // Exactly one direct attempt, then only remux manifests: no bounce back.
  await expect
    .poll(() => mediaRequests.filter(url => url.includes("/hls/")).length)
    .toBeGreaterThan(0);
  const [streamRequest, ...hlsRequests] = mediaRequests;
  expect(new URL(streamRequest).pathname).toBe(`${movieApiPath}/stream`);
  expect(
    hlsRequests.every(url => new URL(url).pathname.endsWith("/hls/remux/playlist.m3u8")),
  ).toBe(true);
  await expectURLParams(page, { mode: "remux" });
  browserIssues.assertClean();
});

test("HLS seek navigation preserves canonical subtitle-off", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page, "mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0");
  await expect(playButton(page)).toBeVisible();

  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await page.getByRole("menuitem", { name: /The Journey/ }).click();

  await expectURLParams(page, { start: "372", subtitle_track: "off" });
  browserIssues.assertClean();
});

test("selecting a chapter seeks to its start and announces the jump", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page, "mode=direct&audio_track=0&subtitle_track=off&start=0");
  await expect(playButton(page)).toBeVisible();

  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await page.getByRole("menuitem", { name: /The Journey/ }).click();

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
