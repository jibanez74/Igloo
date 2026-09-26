import { expect, type Page } from "@playwright/test";
import { readJSON } from "./e2e-api";

// Shared player helpers. The locator and request helpers serve the mocked
// player specs as well; the fetch/progress helpers below serve only the
// opt-in real-media suites (hls-transcode, direct-play-media), which run
// against a live instance via E2E_BASE_URL.

/** The player's play toggle, which appears once the chrome is ready. */
export function playButton(page: Page) {
  return page.getByRole("button", { name: "Play (Space or K)" });
}

/**
 * Every request for `pathname`, whether or not a route handler answers it, so
 * a test can prove the stream was (or was not) asked for.
 */
export function trackStreamRequests(page: Page, pathname: string) {
  const streamRequests: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === pathname) {
      streamRequests.push(request.url());
    }
  });
  return streamRequests;
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

  const body = await readJSON<T>(response);
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
