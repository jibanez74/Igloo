import { expect, test, type Page, type Response } from "@playwright/test";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type VideoStream = {
  codec: string;
  height: number;
};

type MovieTechnicalDetails = {
  video_streams: VideoStream[];
  audio_streams: unknown[];
};

type HlsProfile = keyof typeof transcodeProfiles;

type HlsCase = {
  title: string;
  movieId: number;
  profile: HlsProfile;
  minimumSourceHeight?: number;
};

type HlsEnv = {
  email: string;
  password: string;
  audioTrack: number;
  responseTimeoutMs: number;
  cases: HlsCase[];
};

const transcodeProfiles = {
  "2160p_16mbps": { maxHeight: 2160 },
  "1080p_8mbps": { maxHeight: 1080 },
  "1080p_6mbps": { maxHeight: 1080 },
  "1080p_4mbps": { maxHeight: 1080 },
  "720p_3mbps": { maxHeight: 720 },
} as const;

const coverArtCodecs = new Set(["mjpeg", "png", "gif", "bmp"]);
const playbackSessionPattern =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

function positiveIntEnv(name: string, fallback?: number) {
  const raw = process.env[name];
  if (!raw) return fallback;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

function profileEnv(name: string, fallback: HlsProfile): HlsProfile {
  const raw = process.env[name];
  if (!raw) return fallback;
  if (raw in transcodeProfiles) return raw as HlsProfile;

  throw new Error(
    name + " must be one of " + Object.keys(transcodeProfiles).join(", "),
  );
}

function readHlsEnv(): HlsEnv | null {
  const email = process.env.E2E_ADMIN_EMAIL;
  const password = process.env.E2E_ADMIN_PASSWORD;
  const fourKMovieId = positiveIntEnv("E2E_HLS_4K_MOVIE_ID");
  const secondMovieId = positiveIntEnv("E2E_HLS_SECOND_MOVIE_ID");
  const fourKProfile = profileEnv("E2E_HLS_4K_PROFILE", "2160p_16mbps");
  const secondProfile = profileEnv("E2E_HLS_SECOND_PROFILE", "720p_3mbps");

  if (!email || !password || !fourKMovieId || !secondMovieId) {
    return null;
  }

  return {
    email,
    password,
    audioTrack: positiveIntEnv("E2E_HLS_AUDIO_TRACK", 0) ?? 0,
    responseTimeoutMs:
      positiveIntEnv("E2E_HLS_RESPONSE_TIMEOUT_MS", 240_000) ?? 240_000,
    cases: [
      {
        title: "4K movie transcodes at 2160p",
        movieId: fourKMovieId,
        profile: fourKProfile,
        minimumSourceHeight: 2160,
      },
      {
        title: "second movie transcodes with a different profile",
        movieId: secondMovieId,
        profile: secondProfile,
      },
    ],
  };
}

function hlsAssetPath(movieId: number, profile: HlsProfile) {
  return `/api/movies/${movieId}/hls/${profile}/`;
}

function responsePath(response: Response) {
  return new URL(response.url()).pathname;
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

async function login(page: Page, env: HlsEnv) {
  const loginResponse = await page.context().request.post("/api/auth/login", {
    data: {
      email: env.email,
      password: env.password,
    },
    failOnStatusCode: false,
  });
  expect(loginResponse.status()).toBe(200);

  const loginBody = (await loginResponse.json()) as ApiResponse<unknown>;
  expect(loginBody.error, loginBody.message).toBe(false);

  const authResponse = await page.context().request.get("/api/auth/user", {
    failOnStatusCode: false,
  });
  expect(authResponse.status()).toBe(200);
}

async function fetchTechnicalDetails(page: Page, movieId: number) {
  const response = await page
    .context()
    .request.get(`/api/movies/${movieId}/technical-details`, {
      failOnStatusCode: false,
    });
  expect(response.status()).toBe(200);

  const body = (await response.json()) as ApiResponse<MovieTechnicalDetails>;
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

async function clearWatchProgress(page: Page, movieId: number) {
  await page.context().request.delete(`/api/movies/${movieId}/watch-progress`, {
    failOnStatusCode: false,
  });
}

async function expectMovieSupportsCase(
  page: Page,
  hlsCase: HlsCase,
  audioTrack: number,
) {
  const details = await fetchTechnicalDetails(page, hlsCase.movieId);
  const primaryVideo = primaryVideoStream(details.video_streams);

  expect(primaryVideo, "movie must have a primary video stream").toBeTruthy();
  expect(
    details.audio_streams.length,
    `movie ${hlsCase.movieId} must have audio track ${audioTrack}`,
  ).toBeGreaterThan(audioTrack);
  expect(
    primaryVideo.height,
    `movie ${hlsCase.movieId} source height must support ${hlsCase.profile}`,
  ).toBeGreaterThanOrEqual(transcodeProfiles[hlsCase.profile].maxHeight);

  if (hlsCase.minimumSourceHeight) {
    expect(
      primaryVideo.height,
      `movie ${hlsCase.movieId} must be a 4K source`,
    ).toBeGreaterThanOrEqual(hlsCase.minimumSourceHeight);
  }
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
    if (response.status() !== 200) return;

    const name = segmentName(response.url());
    if (name) found.set(name, response);
  };

  responses.forEach(remember);

  while (found.size < count) {
    const response = await page.waitForResponse(
      candidate => {
        if (!responsePath(candidate).startsWith(assetPath)) return false;
        if (candidate.status() !== 200) return false;

        const name = segmentName(candidate.url());
        return !!name && !found.has(name);
      },
      { timeout },
    );
    remember(response);
  }

  return [...found.values()].slice(0, count);
}

async function expectVideoAdvances(page: Page, timeout: number) {
  await page.waitForFunction(
    () => {
      const video = document.querySelector("video");
      return (
        video instanceof HTMLVideoElement &&
        video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA &&
        Number.isFinite(video.duration) &&
        video.duration > 0
      );
    },
    undefined,
    { timeout },
  );

  const currentTime = await page.locator("video").evaluate(video => {
    return video instanceof HTMLVideoElement ? video.currentTime : 0;
  });

  await page.waitForFunction(
    previousTime => {
      const video = document.querySelector("video");
      return (
        video instanceof HTMLVideoElement &&
        !video.paused &&
        video.currentTime >= Number(previousTime) + 2
      );
    },
    currentTime,
    { timeout },
  );
}

const hlsEnv = readHlsEnv();
const hlsCases: HlsCase[] = hlsEnv?.cases ?? [
  {
    title: "4K movie transcodes at 2160p",
    movieId: 1,
    profile: "2160p_16mbps",
    minimumSourceHeight: 2160,
  },
  {
    title: "second movie transcodes with a different profile",
    movieId: 2,
    profile: "720p_3mbps",
  },
];

test.describe.configure({ mode: "serial" });

test.describe("HLS transcoding playback", () => {
  test.skip(
    !hlsEnv,
    "Set E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD, E2E_HLS_4K_MOVIE_ID, and E2E_HLS_SECOND_MOVIE_ID to run HLS e2e tests.",
  );
  test.beforeEach(async ({ page }) => {
    await login(page, hlsEnv!);
  });

  test.beforeAll(() => {
    expect(
      hlsEnv!.cases[1].profile,
      "second movie must use a different profile from the 4K case",
    ).not.toBe(hlsEnv!.cases[0].profile);
    expect(
      hlsEnv!.cases[1].movieId,
      "second movie must use a different movie from the 4K case",
    ).not.toBe(hlsEnv!.cases[0].movieId);
  });

  for (const hlsCase of hlsCases) {
    test(hlsCase.title, async ({ page }) => {
      const assetPath = hlsAssetPath(hlsCase.movieId, hlsCase.profile);
      const hlsResponses: Response[] = [];
      page.on("response", response => {
        if (responsePath(response).startsWith(assetPath)) {
          hlsResponses.push(response);
        }
      });

      await expectMovieSupportsCase(page, hlsCase, hlsEnv!.audioTrack);
      await clearWatchProgress(page, hlsCase.movieId);

      const manifestResponsePromise = page.waitForResponse(
        response =>
          responsePath(response) === `${assetPath}playlist.m3u8` &&
          response.status() === 200,
        { timeout: hlsEnv!.responseTimeoutMs },
      );

      await page.goto(
        `/movies/${hlsCase.movieId}/play?mode=${hlsCase.profile}&audio_track=${hlsEnv!.audioTrack}&start=0`,
      );
      await expect(page.locator("video")).toBeVisible({
        timeout: hlsEnv!.responseTimeoutMs,
      });

      const manifestResponse = await manifestResponsePromise;
      expect(manifestResponse.headers()["content-type"]).toContain(
        "application/vnd.apple.mpegurl",
      );

      const manifestUrl = new URL(manifestResponse.url());
      const playbackSession = manifestUrl.searchParams.get("playback_session");
      expect(playbackSession).toMatch(playbackSessionPattern);
      expect(manifestUrl.searchParams.get("start")).toBe("0");
      expect(manifestUrl.searchParams.get("audio_track")).toBe(
        String(hlsEnv!.audioTrack),
      );

      const query = {
        playbackSession: playbackSession!,
        audioTrack: hlsEnv!.audioTrack,
        start: "0",
      };

      const playButton = page.getByRole("button", { name: "Play" });
      await expect(playButton).toBeVisible({
        timeout: hlsEnv!.responseTimeoutMs,
      });
      await playButton.click();

      const initResponse = await page.waitForResponse(
        response =>
          responsePath(response) === `${assetPath}init.mp4` &&
          response.status() === 200,
        { timeout: hlsEnv!.responseTimeoutMs },
      );
      expect(initResponse.headers()["content-type"]).toContain("video/mp4");
      assertHlsQuery(initResponse, query);

      const segmentResponses = await waitForUniqueSegments(
        page,
        hlsResponses,
        assetPath,
        2,
        hlsEnv!.responseTimeoutMs,
      );
      for (const response of segmentResponses) {
        expect(response.headers()["content-type"]).toContain("video/mp4");
        assertHlsQuery(response, query);
      }

      await expectVideoAdvances(page, hlsEnv!.responseTimeoutMs);
      await expect(page.getByText(/playback failed|stream error/i)).toHaveCount(
        0,
      );
    });
  }
});
