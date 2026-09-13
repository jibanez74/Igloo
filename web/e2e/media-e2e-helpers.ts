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

/**
 * A movie or a TV episode under test. Movies and episodes expose the same
 * playback endpoints under different prefixes and play on different routes;
 * the helpers below resolve both from this pair.
 */
export type E2EMedia =
  | { kind: "movie"; id: number }
  | { kind: "episode"; id: number };

export function mediaApiPath(media: E2EMedia) {
  return media.kind === "movie"
    ? `/api/movies/${media.id}`
    : `/api/shows/episodes/${media.id}`;
}

/**
 * The player route for the media. An episode's route carries its show id,
 * which only the episode's playback header knows.
 */
export async function mediaPlayPath(page: Page, media: E2EMedia) {
  if (media.kind === "movie") return `/movies/${media.id}/play`;

  const header = await fetchMediaJSON<{ show: { id: number } }>(
    page,
    mediaApiPath(media),
  );
  return `/tv-shows/${header.show.id}/episodes/${media.id}/play`;
}

async function fetchMediaJSON<T>(page: Page, path: string): Promise<T> {
  const response = await page.context().request.get(path, {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = (await response.json()) as MediaApiResponse<T>;
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

export function fetchTechnicalDetails<T>(page: Page, media: E2EMedia) {
  return fetchMediaJSON<T>(page, `${mediaApiPath(media)}/technical-details`);
}

export async function clearWatchProgress(page: Page, media: E2EMedia) {
  await page.context().request.delete(`${mediaApiPath(media)}/watch-progress`, {
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
