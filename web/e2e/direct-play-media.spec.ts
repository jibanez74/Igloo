import { expect, test, type Page } from "@playwright/test";
import { readE2EEnv, type E2EEnv } from "./e2e-env";
import { directPlayAudioSelectionEligible } from "../src/lib/playback";
import {
  clearMovieWatchProgress,
  expectVideoAdvances,
  fetchMovieTechnicalDetails,
  loginWithCredentials,
} from "./media-e2e-helpers";

// Opt-in real-media suite for direct-play eligibility (audit §10.2 matrix
// rows 7, 22 and the multi-audio rows). Runs against a live Igloo instance:
// set E2E_BASE_URL, E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD and the movie IDs
// below, with matching files present in the library:
//   E2E_DIRECT_MKV_MOVIE_ID        MKV with H.264 video + AAC audio
//   E2E_DIRECT_10BIT_MOVIE_ID      MP4 with 10-bit H.264 (High 10)
//   E2E_DIRECT_MULTIAUDIO_MOVIE_ID MP4 with two or more audio streams
//   E2E_DIRECT_MP4_MOVIE_ID        optional happy-path control: plain
//                                  8-bit H.264 + AAC MP4
//   E2E_DIRECT_SUBTITLE_MOVIE_ID   optional: direct-eligible MP4 with an
//                                  embedded text subtitle stream at least
//                                  90 seconds long (audit D11)

type DirectMediaEnv = Pick<E2EEnv, "email" | "password"> & {
  responseTimeoutMs: number;
  mkvMovieId: number;
  tenBitMovieId: number;
  multiAudioMovieId: number;
  mp4MovieId?: number;
  subtitleMovieId?: number;
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

function positiveIntEnv(name: string) {
  const raw = process.env[name];
  if (!raw) return undefined;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function readDirectMediaEnv(): DirectMediaEnv | null {
  const e2eEnv = readE2EEnv();
  const mkvMovieId = positiveIntEnv("E2E_DIRECT_MKV_MOVIE_ID");
  const tenBitMovieId = positiveIntEnv("E2E_DIRECT_10BIT_MOVIE_ID");
  const multiAudioMovieId = positiveIntEnv("E2E_DIRECT_MULTIAUDIO_MOVIE_ID");

  if (!mkvMovieId || !tenBitMovieId || !multiAudioMovieId) {
    return null;
  }

  return {
    email: e2eEnv.email,
    password: e2eEnv.password,
    responseTimeoutMs: 240_000,
    mkvMovieId,
    tenBitMovieId,
    multiAudioMovieId,
    mp4MovieId: positiveIntEnv("E2E_DIRECT_MP4_MOVIE_ID"),
    subtitleMovieId: positiveIntEnv("E2E_DIRECT_SUBTITLE_MOVIE_ID"),
  };
}

const directEnv = readDirectMediaEnv();

// The app's own rule, so the multi-audio expectation cannot drift from it
// while still adapting to the operator's actual file.
const audioSelectionEligible = directPlayAudioSelectionEligible;

function trackStreamRequests(page: Page, movieId: number) {
  const streamRequests: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === `/api/movies/${movieId}/stream`) {
      streamRequests.push(request.url());
    }
  });
  return streamRequests;
}

async function openPlayerWithDefaults(page: Page, movieId: number) {
  // No mode in the URL: the loader redirect canonicalises the app's own
  // default choice, which is exactly the decision under test.
  await page.goto(`/movies/${movieId}/play?start=0`);
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible({ timeout: directEnv?.responseTimeoutMs });
  return new URL(page.url()).searchParams.get("mode");
}

async function pressPlay(page: Page) {
  await page.getByRole("button", { name: "Play (Space or K)" }).click();
}

test.describe.configure({ mode: "serial" });

test.describe("Direct-play eligibility with real media", () => {
  test.skip(
    !directEnv,
    "Set E2E_DIRECT_MKV_MOVIE_ID, E2E_DIRECT_10BIT_MOVIE_ID and E2E_DIRECT_MULTIAUDIO_MOVIE_ID to run direct-play media tests.",
  );

  test.beforeEach(async ({ page }) => {
    await loginWithCredentials(page, directEnv!);
  });

  // Matrix row 7 — the highest-value regression guard: MKV must never reach
  // /stream. Chromium fails MKV silently at 0ms with no MediaError.
  test("MKV H.264 is never offered direct play and never requests the raw stream", async ({
    page,
  }) => {
    const movieId = directEnv!.mkvMovieId;
    const details = await fetchMovieTechnicalDetails<TechnicalDetails>(
      page,
      movieId,
    );
    expect(
      details.movie.container,
      "E2E_DIRECT_MKV_MOVIE_ID must point at an MKV movie",
    ).toBe("mkv");

    const streamRequests = trackStreamRequests(page, movieId);
    await clearMovieWatchProgress(page, movieId);

    const mode = await openPlayerWithDefaults(page, movieId);
    expect(mode).not.toBe("direct");

    await pressPlay(page);
    await expectVideoAdvances(page, directEnv!.responseTimeoutMs);
    expect(streamRequests).toEqual([]);
  });

  // Matrix row 22 — 10-bit H.264 passes the codec-name gate but browsers
  // cannot decode it; it must be refused pre-emptively and still play via HLS.
  test("10-bit H.264 MP4 is refused direct play and plays via HLS", async ({
    page,
  }) => {
    const movieId = directEnv!.tenBitMovieId;
    const details = await fetchMovieTechnicalDetails<TechnicalDetails>(
      page,
      movieId,
    );
    const video = details.video_streams[0];
    const bitDepth = video?.bit_depth?.Valid ? video.bit_depth.Int64 : null;
    const profile = video?.codec_profile?.Valid
      ? video.codec_profile.String
      : "";
    expect(
      bitDepth === 10 || profile.includes("10"),
      "E2E_DIRECT_10BIT_MOVIE_ID must point at a 10-bit H.264 movie",
    ).toBe(true);

    const streamRequests = trackStreamRequests(page, movieId);
    await clearMovieWatchProgress(page, movieId);

    const mode = await openPlayerWithDefaults(page, movieId);
    expect(mode).not.toBe("direct");

    await pressPlay(page);
    await expectVideoAdvances(page, directEnv!.responseTimeoutMs);
    expect(streamRequests).toEqual([]);
  });

  // Matrix rows 2 / 16 / 16b / 16c — the expectation adapts to the file's
  // actual dispositions via the same refuse-on-ambiguity table the app uses.
  test("multi-audio MP4 follows the disposition ambiguity table", async ({
    page,
  }) => {
    const movieId = directEnv!.multiAudioMovieId;
    const details = await fetchMovieTechnicalDetails<TechnicalDetails>(
      page,
      movieId,
    );
    expect(
      details.audio_streams.length,
      "E2E_DIRECT_MULTIAUDIO_MOVIE_ID must point at a movie with two or more audio streams",
    ).toBeGreaterThanOrEqual(2);

    const streamRequests = trackStreamRequests(page, movieId);
    await clearMovieWatchProgress(page, movieId);

    const mode = await openPlayerWithDefaults(page, movieId);
    const ambiguousAudio = !audioSelectionEligible(details.audio_streams);
    if (ambiguousAudio) {
      expect(mode).not.toBe("direct");
    }

    await pressPlay(page);
    await expectVideoAdvances(page, directEnv!.responseTimeoutMs);

    if (mode === "direct") {
      expect(streamRequests.length).toBeGreaterThan(0);
    } else {
      expect(streamRequests).toEqual([]);
    }
    // A working non-direct mode must never have touched the raw stream, and a
    // direct mode must never bounce away from it mid-playback.
    expect(new URL(page.url()).searchParams.get("mode")).toBe(mode);
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
      !directEnv!.subtitleMovieId,
      "Set E2E_DIRECT_SUBTITLE_MOVIE_ID to run the subtitle-persistence test.",
    );

    const movieId = directEnv!.subtitleMovieId!;
    const details = await fetchMovieTechnicalDetails<TechnicalDetails>(
      page,
      movieId,
    );
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
    await clearMovieWatchProgress(page, movieId);
    const saveResponse = await page
      .context()
      .request.put(`/api/movies/${movieId}/watch-progress`, {
        data: {
          progress_sec: resumeTargetSec,
          duration_sec: durationSec,
          save_session_id: `e2e-subtitle-persistence-${Date.now()}`,
          save_sequence: 1,
        },
        failOnStatusCode: false,
      });
    expect(saveResponse.status()).toBe(200);

    await page.goto(
      `/movies/${movieId}/play?mode=direct&audio_track=0&subtitle_track=0&start=0`,
    );

    const subtitleShowing = () =>
      page.locator("video").evaluate(video => {
        const track = video.querySelector<HTMLTrackElement>(
          "track[data-subtitle]",
        );
        return track?.track.mode ?? null;
      });

    // The dialog only opens once watch progress resolves with start=0.
    await page.getByRole("button", { name: "Resume" }).click();
    await expect(page).toHaveURL(new RegExp(`start=${resumeTargetSec}(&|$)`), {
      timeout: directEnv!.responseTimeoutMs,
    });

    await pressPlay(page);
    await expectVideoAdvances(page, directEnv!.responseTimeoutMs);

    // The seek landed instead of restarting from byte 0...
    const currentTime = await page
      .locator("video")
      .evaluate(video =>
        video instanceof HTMLVideoElement ? video.currentTime : 0,
      );
    expect(currentTime).toBeGreaterThanOrEqual(resumeTargetSec - 15);
    // ...the mode never fell back away from direct...
    expect(new URL(page.url()).searchParams.get("mode")).toBe("direct");
    // ...and the subtitle track survived the start change (audit D11).
    expect(await subtitleShowing()).toBe("showing");
  });

  // Matrix row 1 — happy-path control: an eligible MP4 direct-plays for real.
  test("plain H.264+AAC MP4 direct-plays and advances", async ({ page }) => {
    test.skip(
      !directEnv!.mp4MovieId,
      "Set E2E_DIRECT_MP4_MOVIE_ID to run the direct-play happy-path test.",
    );

    const movieId = directEnv!.mp4MovieId!;
    const streamRequests = trackStreamRequests(page, movieId);
    await clearMovieWatchProgress(page, movieId);

    const mode = await openPlayerWithDefaults(page, movieId);
    expect(mode).toBe("direct");

    await pressPlay(page);
    await expectVideoAdvances(page, directEnv!.responseTimeoutMs);
    expect(streamRequests.length).toBeGreaterThan(0);
    // No fallback fired: the mode is still direct after real playback.
    expect(new URL(page.url()).searchParams.get("mode")).toBe("direct");
  });
});
