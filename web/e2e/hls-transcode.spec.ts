import { expect, test, type Page, type Response } from "@playwright/test";

import { STREAM_MODES } from "../src/lib/constants";
import { loginPageViaApi } from "./e2e-auth";
import { intEnv, requireRealInstance } from "./e2e-env";
import {
  clearWatchProgress,
  expectVideoAdvances,
  fetchTechnicalDetails,
  mediaApiPath,
  mediaPlayPath,
  playButton,
  realMediaTimeouts,
  type E2EMedia,
} from "./media-e2e-helpers";

// Opt-in real-media suite for HLS transcoding. Runs against a live Igloo
// instance: set E2E_BASE_URL, E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD and:
//   E2E_HLS_4K_MOVIE_ID      a 2160p movie, transcoded with E2E_HLS_4K_PROFILE
//                            (default 2160p_16mbps)
//   E2E_HLS_SECOND_MOVIE_ID  another movie, transcoded with a different
//                            profile, E2E_HLS_SECOND_PROFILE (default 720p_3mbps);
//                            it proves profile and movie isolation, so the two
//                            movie cases only run as a pair
//   E2E_EPISODE_ID           optional: a TV episode transcoded through the
//                            episode routes with E2E_HLS_EPISODE_PROFILE
//                            (default 720p_3mbps)
//   E2E_HLS_AUDIO_TRACK      the audio track to request (default 0)

type TranscodeMode = Extract<(typeof STREAM_MODES)[number], { type: "transcode" }>;
type HlsProfile = TranscodeMode["id"];

const transcodeProfiles = STREAM_MODES.filter(
  (mode): mode is TranscodeMode => mode.type === "transcode",
);

type VideoStream = {
  codec: string;
  height: number;
};

type MediaTechnicalDetails = {
  video_streams: VideoStream[];
  audio_streams: unknown[];
};

type HlsCase = {
  media: E2EMedia;
  profile: HlsProfile;
  minimumSourceHeight?: number;
};

type HlsEnv = {
  audioTrack: number;
  fourK?: HlsCase;
  second?: HlsCase;
  episode?: HlsCase;
};

const coverArtCodecs = new Set(["mjpeg", "png", "gif", "bmp"]);
const playbackSessionPattern =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

function profileEnv(name: string, fallback: HlsProfile): HlsProfile {
  const raw = process.env[name];
  if (!raw) return fallback;

  const profile = transcodeProfiles.find(mode => mode.id === raw);
  if (profile) return profile.id;

  throw new Error(
    `${name} must be one of ${transcodeProfiles.map(mode => mode.id).join(", ")}`,
  );
}

function profileMaxHeight(profile: HlsProfile) {
  return transcodeProfiles.find(mode => mode.id === profile)!.maxHeight;
}

function readHlsEnv(): HlsEnv {
  const fourKMovieId = intEnv("E2E_HLS_4K_MOVIE_ID");
  const secondMovieId = intEnv("E2E_HLS_SECOND_MOVIE_ID");
  const episodeId = intEnv("E2E_EPISODE_ID");
  const movies = fourKMovieId && secondMovieId ? { fourKMovieId, secondMovieId } : undefined;

  return {
    audioTrack: intEnv("E2E_HLS_AUDIO_TRACK", 0, 0),
    fourK: movies && {
      media: { kind: "movie", id: movies.fourKMovieId },
      profile: profileEnv("E2E_HLS_4K_PROFILE", "2160p_16mbps"),
      minimumSourceHeight: 2160,
    },
    second: movies && {
      media: { kind: "movie", id: movies.secondMovieId },
      profile: profileEnv("E2E_HLS_SECOND_PROFILE", "720p_3mbps"),
    },
    episode: episodeId
      ? {
          media: { kind: "episode", id: episodeId },
          profile: profileEnv("E2E_HLS_EPISODE_PROFILE", "720p_3mbps"),
        }
      : undefined,
  };
}

function hlsAssetPath(media: E2EMedia, profile: HlsProfile) {
  return `${mediaApiPath(media)}/hls/${profile}/`;
}

function responsePath(response: Response) {
  return new URL(response.url()).pathname;
}

function isSuccessfulHlsAssetResponse(response: Response) {
  return response.status() === 200 || response.status() === 206;
}

function assertSuccessfulHlsAssetResponse(response: Response) {
  expect([200, 206]).toContain(response.status());

  if (response.status() !== 206) return;

  expect(response.request().headers()["range"]).toMatch(/^bytes=/i);
  expect(response.headers()["accept-ranges"]).toBe("bytes");

  const contentRange = response.headers()["content-range"];
  const match = /^bytes (\d+)-(\d+)\/(\d+)$/.exec(contentRange ?? "");
  expect(match, "206 response must contain a valid Content-Range").not.toBeNull();

  if (match) {
    const start = Number.parseInt(match[1], 10);
    const end = Number.parseInt(match[2], 10);
    const size = Number.parseInt(match[3], 10);
    expect(start).toBeLessThanOrEqual(end);
    expect(end).toBeLessThan(size);
  }
}

function segmentName(url: string) {
  const match = new URL(url).pathname.match(/\/(segment_\d+\.m4s)$/);
  return match?.[1] ?? null;
}

function assertHlsQuery(
  response: Response,
  expected: { playbackSession: string; audioTrack: number; start: string },
) {
  const url = new URL(response.url());
  expect(url.searchParams.get("playback_session")).toBe(
    expected.playbackSession,
  );
  expect(url.searchParams.get("start")).toBe(expected.start);
  expect(url.searchParams.get("audio_track")).toBe(String(expected.audioTrack));
}

function primaryVideoStream(streams: VideoStream[]) {
  return (
    streams.find(stream => !coverArtCodecs.has(stream.codec.toLowerCase())) ??
    streams[0]
  );
}

async function expectMediaSupportsCase(
  page: Page,
  hlsCase: HlsCase,
  audioTrack: number,
) {
  const details = await fetchTechnicalDetails<MediaTechnicalDetails>(
    page,
    hlsCase.media,
  );
  const primaryVideo = primaryVideoStream(details.video_streams);
  const label = `${hlsCase.media.kind} ${hlsCase.media.id}`;

  expect(primaryVideo, `${label} must have a primary video stream`).toBeTruthy();
  expect(
    details.audio_streams.length,
    `${label} must have audio track ${audioTrack}`,
  ).toBeGreaterThan(audioTrack);
  expect(
    primaryVideo.height,
    `${label} source height must support ${hlsCase.profile}`,
  ).toBeGreaterThanOrEqual(profileMaxHeight(hlsCase.profile));

  if (hlsCase.minimumSourceHeight) {
    expect(
      primaryVideo.height,
      `${label} must be a 4K source`,
    ).toBeGreaterThanOrEqual(hlsCase.minimumSourceHeight);
  }
}

/**
 * The successful response for one asset of the session, whether it already
 * arrived or is still to come. hls.js fetches the init segment and the first
 * media segment as soon as the manifest loads, before Play is pressed, so a
 * wait registered after the click would miss them.
 */
function waitForHlsAsset(
  page: Page,
  responses: Response[],
  assetPath: string,
  filename: string,
  timeout: number,
) {
  const matches = (response: Response) =>
    responsePath(response) === `${assetPath}${filename}` &&
    isSuccessfulHlsAssetResponse(response);

  const seen = responses.find(matches);
  return seen ? Promise.resolve(seen) : page.waitForResponse(matches, { timeout });
}

async function waitForUniqueSegments(
  page: Page,
  responses: Response[],
  assetPath: string,
  count: number,
  timeout: number,
) {
  const found = new Map<string, Response>();

  const remember = (response: Response) => {
    if (!responsePath(response).startsWith(assetPath)) return;
    if (!isSuccessfulHlsAssetResponse(response)) return;

    const name = segmentName(response.url());
    if (name) found.set(name, response);
  };

  responses.forEach(remember);

  while (found.size < count) {
    const response = await page.waitForResponse(
      candidate => {
        if (!responsePath(candidate).startsWith(assetPath)) return false;
        if (!isSuccessfulHlsAssetResponse(candidate)) return false;

        const name = segmentName(candidate.url());
        return !!name && !found.has(name);
      },
      { timeout },
    );
    remember(response);
  }

  return [...found.values()].slice(0, count);
}

const hlsEnv = readHlsEnv();
const timeouts = realMediaTimeouts();

async function runHlsCase(page: Page, hlsCase: HlsCase) {
  const { audioTrack } = hlsEnv;
  const assetPath = hlsAssetPath(hlsCase.media, hlsCase.profile);
  const hlsResponses: Response[] = [];
  page.on("response", response => {
    if (responsePath(response).startsWith(assetPath)) {
      hlsResponses.push(response);
    }
  });

  await expectMediaSupportsCase(page, hlsCase, audioTrack);
  await clearWatchProgress(page, hlsCase.media);
  const playPath = await mediaPlayPath(page, hlsCase.media);

  await page.goto(
    `${playPath}?mode=${hlsCase.profile}&audio_track=${audioTrack}&start=0`,
  );
  await expect(page.locator("video")).toBeVisible({ timeout: timeouts.response });

  const manifestResponse = await waitForHlsAsset(
    page,
    hlsResponses,
    assetPath,
    "playlist.m3u8",
    timeouts.response,
  );
  expect(manifestResponse.status()).toBe(200);
  expect(manifestResponse.headers()["content-type"]).toContain(
    "application/vnd.apple.mpegurl",
  );

  const playbackSession = new URL(manifestResponse.url()).searchParams.get(
    "playback_session",
  );
  expect(playbackSession).toMatch(playbackSessionPattern);

  // Every asset of the session carries the same query, from the manifest on.
  const query = { playbackSession: playbackSession!, audioTrack, start: "0" };
  assertHlsQuery(manifestResponse, query);

  await expect(playButton(page)).toBeVisible({ timeout: timeouts.response });
  await playButton(page).click();

  const initResponse = await waitForHlsAsset(
    page,
    hlsResponses,
    assetPath,
    "init.mp4",
    timeouts.response,
  );
  assertSuccessfulHlsAssetResponse(initResponse);
  expect(initResponse.headers()["content-type"]).toContain("video/mp4");
  assertHlsQuery(initResponse, query);

  const segmentResponses = await waitForUniqueSegments(
    page,
    hlsResponses,
    assetPath,
    2,
    timeouts.response,
  );
  for (const response of segmentResponses) {
    assertSuccessfulHlsAssetResponse(response);
    expect(response.headers()["content-type"]).toContain("video/mp4");
    assertHlsQuery(response, query);
  }

  await expectVideoAdvances(page, timeouts.response);
  await expect(page.getByText(/playback failed|stream error/i)).toHaveCount(0);
}

test.describe("HLS transcoding playback", () => {
  requireRealInstance("HLS transcoding needs real media and ffmpeg");
  test.describe.configure({ timeout: timeouts.test });

  test.beforeEach(async ({ page }) => {
    await loginPageViaApi(page);
  });

  const movieCasesReason =
    "Set E2E_HLS_4K_MOVIE_ID and E2E_HLS_SECOND_MOVIE_ID to run the HLS movie cases.";

  test("4K movie transcodes through the configured profile", async ({ page }) => {
    test.skip(!hlsEnv.fourK, movieCasesReason);
    await runHlsCase(page, hlsEnv.fourK!);
  });

  test("second movie transcodes with a different profile", async ({ page }) => {
    test.skip(!hlsEnv.second, movieCasesReason);
    expect(
      hlsEnv.second!.profile,
      "second movie must use a different profile from the 4K case",
    ).not.toBe(hlsEnv.fourK!.profile);
    expect(
      hlsEnv.second!.media.id,
      "second movie must use a different movie from the 4K case",
    ).not.toBe(hlsEnv.fourK!.media.id);
    await runHlsCase(page, hlsEnv.second!);
  });

  test("TV episode transcodes through the episode routes", async ({ page }) => {
    test.skip(!hlsEnv.episode, "Set E2E_EPISODE_ID to run the episode HLS case.");
    await runHlsCase(page, hlsEnv.episode!);
  });
});
