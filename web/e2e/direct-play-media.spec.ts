import { randomUUID } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { intEnv, requireRealInstance } from "./e2e-env";
import {
  clearWatchProgress,
  expectURLParams,
  expectVideoAdvances,
  fetchTechnicalDetails,
  mediaPlayPath,
  playButton,
  realMediaTimeouts,
  trackStreamRequests,
  type E2EMedia,
} from "./media-e2e-helpers";

// Opt-in real-media suite for direct-play eligibility (audit §10.2 matrix
// rows 7, 22 and the multi-audio rows). Runs against a live Igloo instance:
// set E2E_BASE_URL, E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD and the movie IDs
// below, with matching files present in the library:
//   E2E_DIRECT_MKV_MOVIE_ID        MKV with H.264 video + AAC audio
//   E2E_DIRECT_10BIT_MOVIE_ID      MP4 with 10-bit H.264 (High 10)
//   E2E_DIRECT_MULTIAUDIO_MOVIE_ID MP4 with 8-bit H.264 and two or more AAC
//                                  audio streams, so that only the audio
//                                  dispositions decide direct play
//   E2E_DIRECT_MP4_MOVIE_ID        optional happy-path control: plain
//                                  8-bit H.264 + AAC MP4
//   E2E_DIRECT_SUBTITLE_MOVIE_ID   optional: direct-eligible MP4 with an
//                                  embedded text subtitle stream at least
//                                  90 seconds long (audit D11)
//   E2E_EPISODE_ID                 optional: any scanned TV episode; plays
//                                  it through the episode routes, in
//                                  whichever mode its file allows

// The movie matrix needs its three ids together; the episode case only needs
// its own id, so a TV-only library can still run it.
type DirectMovieMatrix = {
  mkvMovieId: number;
  tenBitMovieId: number;
  multiAudioMovieId: number;
};

type DirectMediaEnv = {
  movies?: DirectMovieMatrix;
  mp4MovieId?: number;
  subtitleMovieId?: number;
  episodeId?: number;
};

type TechnicalDetails = {
  movie: {
    container: string;
    mime_type: string;
    duration: { Float64: number; Valid: boolean } | null;
  };
  video_streams: Array<{
    codec: string;
    codec_profile: { String: string; Valid: boolean } | null;
    bit_depth: { Int64: number; Valid: boolean } | null;
  }>;
  audio_streams: Array<{
    codec: string;
    is_default: boolean;
  }>;
  subtitles: Array<{
    codec: string;
  }>;
};

function readDirectMediaEnv(): DirectMediaEnv {
  const mkvMovieId = intEnv("E2E_DIRECT_MKV_MOVIE_ID");
  const tenBitMovieId = intEnv("E2E_DIRECT_10BIT_MOVIE_ID");
  const multiAudioMovieId = intEnv("E2E_DIRECT_MULTIAUDIO_MOVIE_ID");

  return {
    movies:
      mkvMovieId && tenBitMovieId && multiAudioMovieId
        ? { mkvMovieId, tenBitMovieId, multiAudioMovieId }
        : undefined,
    mp4MovieId: intEnv("E2E_DIRECT_MP4_MOVIE_ID"),
    subtitleMovieId: intEnv("E2E_DIRECT_SUBTITLE_MOVIE_ID"),
    episodeId: intEnv("E2E_EPISODE_ID"),
  };
}

const movieMatrixSkipReason =
  "Set E2E_DIRECT_MKV_MOVIE_ID, E2E_DIRECT_10BIT_MOVIE_ID and E2E_DIRECT_MULTIAUDIO_MOVIE_ID to run the direct-play movie matrix.";

const directEnv = readDirectMediaEnv();
const timeouts = realMediaTimeouts();

function isTenBitVideo(details: TechnicalDetails) {
  const video = details.video_streams[0];
  const bitDepth = video?.bit_depth?.Valid ? video.bit_depth.Int64 : null;
  const profile = video?.codec_profile?.Valid ? video.codec_profile.String : "";
  return bitDepth === 10 || profile.includes("10");
}

async function openPlayerWithDefaults(page: Page, movieId: number) {
  // No mode in the URL: the loader redirect canonicalises the app's own
  // default choice, which is exactly the decision under test. The play button
  // only renders once that redirect has landed.
  await page.goto(`/movies/${movieId}/play?start=0`);
  await expect(playButton(page)).toBeVisible({ timeout: timeouts.response });

  const mode = new URL(page.url()).searchParams.get("mode");
  expect(mode, "the player route must carry a mode").toBeTruthy();
  return mode!;
}

// Files the app must refuse direct play for, whatever the rest of the file
// says: each must play through HLS without ever touching the raw stream.
const refusedDirectPlayCases = [
  {
    // Matrix row 7 — the highest-value regression guard: Chromium fails MKV
    // silently at 0ms with no MediaError.
    title: "MKV H.264 is never offered direct play and never requests the raw stream",
    movieId: () => directEnv.movies?.mkvMovieId,
    precondition: (details: TechnicalDetails) =>
      expect(
        details.movie.container,
        "E2E_DIRECT_MKV_MOVIE_ID must point at an MKV movie",
      ).toBe("mkv"),
  },
  {
    // Matrix row 22 — 10-bit H.264 passes the codec-name gate but browsers
    // cannot decode it; it must be refused pre-emptively and still play via HLS.
    title: "10-bit H.264 MP4 is refused direct play and plays via HLS",
    movieId: () => directEnv.movies?.tenBitMovieId,
    precondition: (details: TechnicalDetails) =>
      expect(
        isTenBitVideo(details),
        "E2E_DIRECT_10BIT_MOVIE_ID must point at a 10-bit H.264 movie",
      ).toBe(true),
  },
];

test.describe("Direct-play eligibility with real media", () => {
  requireRealInstance("direct-play eligibility needs real media files");
  test.describe.configure({ timeout: timeouts.test });

  test.beforeEach(async ({ page }) => {
    await loginPageViaApi(page);
  });

  for (const refused of refusedDirectPlayCases) {
    test(refused.title, async ({ page }) => {
      const movieId = refused.movieId();
      test.skip(!movieId, movieMatrixSkipReason);

      const media: E2EMedia = { kind: "movie", id: movieId! };
      const details = await fetchTechnicalDetails<TechnicalDetails>(page, media);
      refused.precondition(details);

      const streamRequests = trackStreamRequests(page, `/api/movies/${movieId}/stream`);
      await clearWatchProgress(page, media);

      const mode = await openPlayerWithDefaults(page, movieId!);
      expect(mode).not.toBe("direct");

      await playButton(page).click();
      await expectVideoAdvances(page, timeouts.response);
      expect(streamRequests).toEqual([]);
    });
  }

  // Matrix rows 2 / 16 / 16b / 16c — the expectation adapts to the file's
  // actual dispositions via the same refuse-on-ambiguity table the app uses.
  test("multi-audio MP4 follows the disposition ambiguity table", async ({
    page,
  }) => {
    test.skip(!directEnv.movies, movieMatrixSkipReason);
    const movieId = directEnv.movies!.multiAudioMovieId;
    const media: E2EMedia = { kind: "movie", id: movieId };
    const details = await fetchTechnicalDetails<TechnicalDetails>(page, media);
    expect(
      details.audio_streams.length,
      "E2E_DIRECT_MULTIAUDIO_MOVIE_ID must point at a movie with two or more audio streams",
    ).toBeGreaterThanOrEqual(2);
    // Every other direct-play rule must pass, so the audio table alone decides.
    expect(
      details.movie.mime_type,
      "E2E_DIRECT_MULTIAUDIO_MOVIE_ID must point at an MP4",
    ).toBe("video/mp4");
    expect(
      isTenBitVideo(details),
      "E2E_DIRECT_MULTIAUDIO_MOVIE_ID must point at an 8-bit H.264 movie",
    ).toBe(false);
    expect(
      details.audio_streams[0].codec.toLowerCase(),
      "E2E_DIRECT_MULTIAUDIO_MOVIE_ID must have AAC as its first audio stream",
    ).toBe("aac");

    const streamRequests = trackStreamRequests(page, `/api/movies/${movieId}/stream`);
    await clearWatchProgress(page, media);

    const mode = await openPlayerWithDefaults(page, movieId);
    // Restates the ambiguity table from src/lib/playback.ts on purpose, so a
    // regression in the app's rule cannot also move this test's expectation.
    const audio = details.audio_streams;
    const defaultCount = audio.filter(stream => stream.is_default).length;
    const unambiguousAudio =
      defaultCount === 0 || (defaultCount === 1 && audio[0].is_default);
    if (unambiguousAudio) {
      expect(mode, "unambiguous audio dispositions keep direct play").toBe("direct");
    } else {
      expect(mode, "ambiguous audio dispositions refuse direct play").not.toBe("direct");
    }

    await playButton(page).click();
    await expectVideoAdvances(page, timeouts.response);

    if (unambiguousAudio) {
      expect(streamRequests.length).toBeGreaterThan(0);
    } else {
      expect(streamRequests).toEqual([]);
    }
    // A working non-direct mode must never have touched the raw stream, and a
    // direct mode must never bounce away from it mid-playback.
    await expectURLParams(page, { mode });
  });

  // Audit D11 — the deciding browser test the audit could not run: a resume
  // navigation changes `start` while the player is mounted (the D10 trigger),
  // and the sideloaded subtitle track must still be showing afterwards.
  // Neither jsdom (stubbed load()/track) nor the mocked e2e stack (no
  // decodable media, so the direct-play fallback navigates away) can decide
  // this; only a real browser over real media can.
  test("subtitles stay showing across a resume start change in direct play", async ({
    page,
  }) => {
    test.skip(
      !directEnv.subtitleMovieId,
      "Set E2E_DIRECT_SUBTITLE_MOVIE_ID to run the subtitle-persistence test.",
    );

    const movieId = directEnv.subtitleMovieId!;
    const media: E2EMedia = { kind: "movie", id: movieId };
    const details = await fetchTechnicalDetails<TechnicalDetails>(page, media);
    expect(
      details.subtitles.length,
      "E2E_DIRECT_SUBTITLE_MOVIE_ID must point at a movie with an embedded subtitle stream",
    ).toBeGreaterThan(0);
    const durationSec = details.movie.duration?.Valid
      ? details.movie.duration.Float64
      : 0;
    expect(
      durationSec,
      "E2E_DIRECT_SUBTITLE_MOVIE_ID must point at a movie at least 90 seconds long",
    ).toBeGreaterThanOrEqual(90);

    // Seed saved progress so the resume dialog opens: past the eligibility
    // minimum, well short of the completion threshold.
    const resumeTargetSec = Math.min(45, Math.floor(durationSec / 2));
    await clearWatchProgress(page, media);
    const saveResponse = await page
      .context()
      .request.put(`/api/movies/${movieId}/watch-progress`, {
        data: {
          progress_sec: resumeTargetSec,
          duration_sec: durationSec,
          save_session_id: randomUUID(),
          save_sequence: 1,
        },
        failOnStatusCode: false,
      });
    expect(saveResponse.status()).toBe(200);

    await page.goto(
      `/movies/${movieId}/play?mode=direct&audio_track=0&subtitle_track=0&start=0`,
    );

    const video = page.locator("video");
    const subtitleShowing = () =>
      video.evaluate(element => {
        const track = element.querySelector<HTMLTrackElement>("track[data-subtitle]");
        return track?.track.mode ?? null;
      });

    // The dialog only opens once watch progress resolves with start=0.
    await page.getByRole("button", { name: "Resume" }).click();
    await expect(page).toHaveURL(new RegExp(`start=${resumeTargetSec}(&|$)`), {
      timeout: timeouts.response,
    });

    await playButton(page).click();
    await expectVideoAdvances(page, timeouts.response);

    // The seek landed instead of restarting from byte 0...
    await expect
      .poll(() =>
        video.evaluate(element =>
          element instanceof HTMLVideoElement ? element.currentTime : 0,
        ),
      )
      .toBeGreaterThanOrEqual(resumeTargetSec - 15);
    // ...the mode never fell back away from direct...
    await expectURLParams(page, { mode: "direct" });
    // ...and the subtitle track survived the start change (audit D11).
    expect(await subtitleShowing()).toBe("showing");
  });

  // Matrix row 1 — happy-path control: an eligible MP4 direct-plays for real.
  test("plain H.264+AAC MP4 direct-plays and advances", async ({ page }) => {
    test.skip(
      !directEnv.mp4MovieId,
      "Set E2E_DIRECT_MP4_MOVIE_ID to run the direct-play happy-path test.",
    );

    const movieId = directEnv.mp4MovieId!;
    const streamRequests = trackStreamRequests(page, `/api/movies/${movieId}/stream`);
    await clearWatchProgress(page, { kind: "movie", id: movieId });

    const mode = await openPlayerWithDefaults(page, movieId);
    expect(mode).toBe("direct");

    await playButton(page).click();
    await expectVideoAdvances(page, timeouts.response);
    expect(streamRequests.length).toBeGreaterThan(0);
    // No fallback fired: the mode is still direct after real playback.
    await expectURLParams(page, { mode: "direct" });
  });

  // The episode routes reuse the movie pipeline end to end: the same
  // default-settings redirect, the same eligibility gate, the same player.
  test("a TV episode plays through the episode routes", async ({ page }) => {
    test.skip(!directEnv.episodeId, "Set E2E_EPISODE_ID to run the episode playback test.");

    const media: E2EMedia = { kind: "episode", id: directEnv.episodeId! };
    const details = await fetchTechnicalDetails<TechnicalDetails>(page, media);
    expect(details.video_streams.length).toBeGreaterThan(0);

    await clearWatchProgress(page, media);

    const playPath = await mediaPlayPath(page, media);
    await page.goto(`${playPath}?start=0`);
    await expect(playButton(page)).toBeVisible({ timeout: timeouts.response });

    // The loader redirect keeps the route and fills in a mode.
    await expect(page).toHaveURL(new RegExp(`${playPath}\\?.*\\bmode=[a-z0-9_]+`));

    await playButton(page).click();
    await expectVideoAdvances(page, timeouts.response);
  });
});
