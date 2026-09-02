import { expect, type Page } from "@playwright/test";

// Shared media helpers. `loginWithCredentials` is used by the default mocked
// specs too (movie-player, direct-play-fallback); the fetch/progress helpers
// below serve only the opt-in real-media suites (hls-transcode,
// direct-play-media), which run against a live instance via E2E_BASE_URL.

type MediaApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

export async function loginWithCredentials(
  page: Page,
  credentials: { email: string; password: string },
) {
  const loginResponse = await page.context().request.post("/api/auth/login", {
    data: {
      email: credentials.email,
      password: credentials.password,
    },
    failOnStatusCode: false,
  });
  expect(loginResponse.status()).toBe(200);

  const loginBody = (await loginResponse.json()) as MediaApiResponse<unknown>;
  expect(loginBody.error, loginBody.message).toBe(false);

  const authResponse = await page.context().request.get("/api/auth/user", {
    failOnStatusCode: false,
  });
  expect(authResponse.status()).toBe(200);
}

export async function fetchMovieTechnicalDetails<T>(
  page: Page,
  movieId: number,
): Promise<T> {
  const response = await page
    .context()
    .request.get(`/api/movies/${movieId}/technical-details`, {
      failOnStatusCode: false,
    });
  expect(response.status()).toBe(200);

  const body = (await response.json()) as MediaApiResponse<T>;
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

export async function clearMovieWatchProgress(page: Page, movieId: number) {
  await page.context().request.delete(`/api/movies/${movieId}/watch-progress`, {
    failOnStatusCode: false,
  });
}

// Both waits throw on MediaError rather than polling until the timeout: a
// decode or source failure can never satisfy the condition, so reporting the
// error code straight away beats a "waitForFunction timed out" after the
// suite's several-minute budget. Every lookup goes through
// document.querySelector so a page with more than one <video> cannot trip
// Playwright's strict-mode locator check partway through.
export async function expectVideoAdvances(page: Page, timeout: number) {
  await page.waitForFunction(
    () => {
      const video = document.querySelector("video");
      if (video instanceof HTMLVideoElement && video.error) {
        throw new Error(
          `media error ${video.error.code}: ${video.error.message || "no message"}`,
        );
      }
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

  const currentTime = await page.evaluate(
    () => document.querySelector("video")?.currentTime ?? 0,
  );

  await page.waitForFunction(
    previousTime => {
      const video = document.querySelector("video");
      if (video instanceof HTMLVideoElement && video.error) {
        throw new Error(
          `media error ${video.error.code}: ${video.error.message || "no message"}`,
        );
      }
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
