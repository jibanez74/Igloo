import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { readE2EEnv } from "./e2e-env";
import { loginWithCredentials } from "./media-e2e-helpers";

// Drives the movie play route against the mock API server. The stream request
// is intentionally left pending: the player chrome reaches its ready state
// from the metadata queries alone, and with the media stuck at HAVE_NOTHING a
// seek sets the element's default playback start position, which currentTime
// reads back — enough to prove the chapter-seek flow without real media.
async function openMoviePlayer(page: Page) {
  await loginWithCredentials(page, readE2EEnv());

  await page.route("**/api/movies/*/stream*", () => {
    // Never fulfilled: keeps the player ready without firing a media error.
  });

  await page.goto(
    "/movies/101/play?mode=direct&audio_track=0&subtitle_track=off&start=0",
  );

  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
}

async function recordHlsManifestRequests(
  page: Page,
  manifestRequests: string[],
) {
  const usesNativeHls = await page.evaluate(() => {
    const video = document.createElement("video");
    return (
      video.canPlayType("application/vnd.apple.mpegurl") !== "" ||
      video.canPlayType("application/x-mpegURL") !== ""
    );
  });

  await page.route(
    "**/api/movies/101/hls/*/playlist.m3u8*",
    async route => {
      manifestRequests.push(route.request().url());
      if (usesNativeHls && route.request().resourceType() === "fetch") {
        await route.fulfill({
          status: 200,
          contentType: "application/vnd.apple.mpegurl",
          body: "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXT-X-ENDLIST\n",
        });
      }
    },
  );

  // Native HLS performs one metadata preflight, then one media load of the
  // same URL. hls.js reads metadata from its single manifest request.
  return usesNativeHls ? 2 : 1;
}

for (const {
  label,
  mode,
  expectedRequestPath,
} of [
  {
    label: "direct",
    mode: "direct",
    expectedRequestPath: "/api/movies/101/stream",
  },
  {
    label: "HLS",
    mode: "720p_3mbps",
    expectedRequestPath:
      "/api/movies/101/hls/720p_3mbps/playlist.m3u8",
  },
]) {
  test(`${label} playback waits for preferences before requesting media`, async ({
    page,
  }) => {
    await loginWithCredentials(page, readE2EEnv());

    let releasePlaybackSettings!: () => void;
    const playbackSettingsGate = new Promise<void>(resolve => {
      releasePlaybackSettings = resolve;
    });
    let playbackSettingsRequested = false;
    await page.route("**/api/settings/playback", async route => {
      playbackSettingsRequested = true;
      await playbackSettingsGate;
      await route.continue();
    });

    const mediaRequests: string[] = [];
    await page.route("**/api/movies/101/stream*", route => {
      mediaRequests.push(route.request().url());
    });
    const expectedHlsRequestCount = await recordHlsManifestRequests(
      page,
      mediaRequests,
    );

    await page.goto(
      `/movies/101/play?mode=${mode}&audio_track=0&start=0`,
    );

    await expect.poll(() => playbackSettingsRequested).toBe(true);
    await expect(page.getByText("Preparing playback...")).toBeVisible();
    await expect(page.locator("video")).toHaveCount(0);
    expect(mediaRequests).toEqual([]);

    releasePlaybackSettings();

    const expectedRequestCount =
      mode === "direct" ? 1 : expectedHlsRequestCount;
    await expect.poll(() => mediaRequests.length).toBe(expectedRequestCount);
    expect(mediaRequests).toHaveLength(expectedRequestCount);
    for (const request of mediaRequests) {
      expect(request).toContain(expectedRequestPath);
    }
  });
}

test("omitted subtitle state synchronizes to the canonical off URL", async ({
  page,
}) => {
  await loginWithCredentials(page, readE2EEnv());

  await page.route("**/api/movies/*/stream*", () => {
    // Keep direct media pending while the route synchronizes its search state.
  });

  await page.goto("/movies/101/play?mode=direct&audio_track=0&start=0");
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
  await expect
    .poll(() => new URL(page.url()).searchParams.get("subtitle_track"))
    .toBe("off");
});

test("loader redirect serializes subtitle-off canonically", async ({ page }) => {
  await loginWithCredentials(page, readE2EEnv());

  await page.route("**/api/movies/*/stream*", () => {
    // Keep direct media pending after the loader's default-settings redirect.
  });

  await page.goto("/movies/101/play?start=0");
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();

  const url = new URL(page.url());
  expect(url.searchParams.get("mode")).toBe("direct");
  expect(url.searchParams.get("audio_track")).toBe("0");
  expect(url.searchParams.get("subtitle_track")).toBe("off");
});

test("cold non-first audio waits for metadata and never requests the raw stream", async ({
  page,
}) => {
  await loginWithCredentials(page, readE2EEnv());

  let releaseTechnicalDetails!: () => void;
  const technicalDetailsGate = new Promise<void>(resolve => {
    releaseTechnicalDetails = resolve;
  });
  let technicalDetailsRequested = false;
  await page.route(
    "**/api/movies/101/technical-details",
    async route => {
      technicalDetailsRequested = true;
      await technicalDetailsGate;

      const response = await route.fetch();
      const body = (await response.json()) as {
        data: {
          audio_streams: Array<Record<string, unknown>>;
        };
      };
      body.data.audio_streams.push({
        ...body.data.audio_streams[0],
        id: 2,
        stream_index: 2,
        language: { String: "spa", Valid: true },
        title: { String: "Spanish", Valid: true },
      });
      await route.fulfill({ response, json: body });
    },
  );

  const mediaRequests: string[] = [];
  await page.route("**/api/movies/101/stream*", route => {
    mediaRequests.push(route.request().url());
  });
  const expectedRequestCount = await recordHlsManifestRequests(
    page,
    mediaRequests,
  );

  await page.goto(
    "/movies/101/play?mode=direct&audio_track=1&subtitle_track=off&start=0",
  );

  await expect.poll(() => technicalDetailsRequested).toBe(true);
  await expect(page.getByText("Preparing playback...")).toBeVisible();
  await expect(page.locator("video")).toHaveCount(0);
  expect(mediaRequests).toEqual([]);

  releaseTechnicalDetails();

  await expect.poll(() => mediaRequests.length).toBe(expectedRequestCount);
  expect(mediaRequests).toHaveLength(expectedRequestCount);
  const requestUrl = new URL(mediaRequests[0]);
  expect(requestUrl.pathname).toBe(
    "/api/movies/101/hls/remux/playlist.m3u8",
  );
  expect(requestUrl.searchParams.get("audio_track")).toBe("1");
  expect(mediaRequests.every(url => url === mediaRequests[0])).toBe(true);
  expect(
    mediaRequests.some(url => new URL(url).pathname.endsWith("/stream")),
  ).toBe(false);
});

test("cold direct deep link to an ineligible file never requests the raw stream", async ({
  page,
}) => {
  await loginWithCredentials(page, readE2EEnv());

  // Serve the movie as MKV with delayed technical details: a bookmarked
  // ?mode=direct&audio_track=0 link must wait for eligibility instead of
  // optimistically firing /stream (audit D16, matrix row 18c).
  let releaseTechnicalDetails!: () => void;
  const technicalDetailsGate = new Promise<void>(resolve => {
    releaseTechnicalDetails = resolve;
  });
  let technicalDetailsRequested = false;
  await page.route(
    "**/api/movies/101/technical-details",
    async route => {
      technicalDetailsRequested = true;
      await technicalDetailsGate;

      const response = await route.fetch();
      const body = (await response.json()) as {
        data: {
          movie: Record<string, unknown>;
        };
      };
      body.data.movie.container = "mkv";
      body.data.movie.mime_type = "video/x-matroska";
      await route.fulfill({ response, json: body });
    },
  );

  const mediaRequests: string[] = [];
  await page.route("**/api/movies/101/stream*", route => {
    mediaRequests.push(route.request().url());
  });
  const expectedRequestCount = await recordHlsManifestRequests(
    page,
    mediaRequests,
  );

  await page.goto(
    "/movies/101/play?mode=direct&audio_track=0&subtitle_track=off&start=0",
  );

  await expect.poll(() => technicalDetailsRequested).toBe(true);
  await expect(page.getByText("Preparing playback...")).toBeVisible();
  await expect(page.locator("video")).toHaveCount(0);
  expect(mediaRequests).toEqual([]);

  releaseTechnicalDetails();

  await expect
    .poll(() => new URL(page.url()).searchParams.get("mode"))
    .toBe("remux");
  await expect.poll(() => mediaRequests.length).toBe(expectedRequestCount);
  expect(mediaRequests).toHaveLength(expectedRequestCount);
  expect(new URL(mediaRequests[0]).pathname).toBe(
    "/api/movies/101/hls/remux/playlist.m3u8",
  );
  expect(mediaRequests.every(url => url === mediaRequests[0])).toBe(true);
  expect(
    mediaRequests.some(url => new URL(url).pathname.endsWith("/stream")),
  ).toBe(false);
  // The pre-emptive mode resolution is silent: it must not read as an
  // error-driven fallback announcement.
  await expect(page.getByText(/can't be played directly/)).toHaveCount(0);
});

test("HLS seek navigation preserves canonical subtitle-off", async ({ page }) => {
  await loginWithCredentials(page, readE2EEnv());

  await page.route(
    "**/api/movies/101/hls/*/playlist.m3u8*",
    () => {
      // Keep HLS pending; player controls and route navigation remain testable.
    },
  );

  await page.goto(
    "/movies/101/play?mode=720p_3mbps&audio_track=0&subtitle_track=off&start=0",
  );
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await page.getByRole("menuitem", { name: /The Journey/ }).click();

  await expect
    .poll(() => new URL(page.url()).searchParams.get("start"))
    .toBe("372");
  expect(new URL(page.url()).searchParams.get("subtitle_track")).toBe("off");
});

test("chapter menu lists chapters with spoken labels and marks the current one", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page);

  const chapterTrigger = page.getByRole("button", {
    name: "Chapters, 2 chapters",
  });
  await expect(chapterTrigger).toBeVisible();
  await chapterTrigger.click();

  const firstChapter = page.getByRole("menuitem", {
    name: "Chapter 1 of 2, Opening Credits, starts at 0 seconds, current chapter",
  });
  await expect(firstChapter).toBeVisible();
  await expect(firstChapter).toHaveAttribute("aria-current", "true");

  const secondChapter = page.getByRole("menuitem", {
    name: "Chapter 2 of 2, The Journey, starts at 6 minutes 12 seconds",
  });
  await expect(secondChapter).toBeVisible();
  await expect(secondChapter).not.toHaveAttribute("aria-current", "true");

  await page.keyboard.press("Escape");
  browserIssues.assertClean();
});

test("selecting a chapter seeks to its start and announces the jump", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);

  await openMoviePlayer(page);

  await page.getByRole("button", { name: "Chapters, 2 chapters" }).click();
  await page
    .getByRole("menuitem", { name: /The Journey/ })
    .click();

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
