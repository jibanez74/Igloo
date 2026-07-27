import { expect, test, type Page } from "@playwright/test";
import { readE2EEnv, type E2EEnv } from "./e2e-env";
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

type DirectMediaEnv = Pick<E2EEnv, "email" | "password"> & {
  responseTimeoutMs: number;
  mkvMovieId: number;
  tenBitMovieId: number;
  multiAudioMovieId: number;
  mp4MovieId?: number;
};

type TechnicalDetails = {
  movie: {
    container: string;
    mime_type: string;
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
  };
}

const directEnv = readDirectMediaEnv();

/**
 * Mirrors directPlayAudioSelectionEligible (web/src/lib/playback.ts) so the
 * multi-audio expectation adapts to the operator's actual file.
 */
function audioSelectionEligible(audioStreams: TechnicalDetails["audio_streams"]) {
  if (audioStreams.length <= 1) return true;
  const defaults = audioStreams.filter(stream => stream.is_default);
  if (defaults.length === 0) return true;
  return defaults.length === 1 && audioStreams[0].is_default;
}

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
