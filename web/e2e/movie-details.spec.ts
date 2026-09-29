import { expect, test, type Locator, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { MOVIES_PER_PAGE } from "../src/lib/constants";
import type {
  LibraryMovieDetailsResponse,
  MovieTechnicalDetailsResponse,
  WatchProgressType,
} from "../src/types";
import {
  apiResponse,
  fulfillJSON,
  nullableFloat64,
  nullableInt64,
  nullableString,
  pagedList,
} from "./e2e-api";
import { VIEWPORTS } from "./e2e-layout";
import { mockApi } from "./e2e-mock-api";
import { expectHref, playButton } from "./media-e2e-helpers";
import { mockYouTubePlayer } from "./mock-youtube-player";
import { libraryMovie } from "./fixtures/movies";

// The details page's sections are unit-tested (src/test/movies/
// movie-details-route.test.tsx); this covers what needs a browser: keyboard
// navigation from the index, the hero's breakpoints, Chromium's direct-play
// verdict in the play links, the playback settings dialog, and the extra
// video player.

const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const movieId = 711;
const moviePath = `/movies/${movieId}`;
const playPath = `${moviePath}/play`;
const chapterStartSeconds = 372;
const extraVideoKey = "signal-fire-trailer";

const signalFire = libraryMovie(
  movieId,
  "Signal Fire",
  2024,
  "/signal-fire-poster.jpg",
);

const movieDetailsPayload = {
  movie: {
    id: movieId,
    title: signalFire.title,
    adult: false,
    tmdb_id: nullableInt64(1711),
    imdb_id: nullableString("tt1711000"),
    poster_path: signalFire.poster_path,
    backdrop_path: nullableString("/signal-fire-backdrop.jpg"),
    language: nullableString("en"),
    year: signalFire.year,
    release_date: nullableString("2024-07-04T12:00:00Z"),
    overview: nullableString(
      "A rescue pilot returns to a coastal town and uncovers the wildfire cover-up that drove her family apart.",
    ),
    tag_line: nullableString("Some fires never fade."),
    certification: signalFire.certification,
    critic_rating: nullableFloat64(8.7),
    audience_rating: nullableFloat64(8.2),
    revenue: nullableFloat64(215000000),
    budget: nullableFloat64(95000000),
    run_time: nullableInt64(126),
    duration: nullableFloat64(7560),
  },
  cast: [
    {
      id: 1,
      character: "Mara Voss",
      cast_order: 0,
      artist_name: "Alex Vega",
      artist_profile: nullableString("/alex-vega.jpg"),
    },
  ],
  crew: [
    { id: 10, job: "Director", department: "Directing", artist_name: "Jordan Lee" },
  ],
  genres: [{ id: 30, tag: "Thriller" }],
  production_companies: [{ id: 20, name: "Northwind Pictures" }],
  extra_videos: [
    {
      id: 30,
      title: "Official Trailer",
      key: extraVideoKey,
      type: "trailer",
      site: "youtube",
    },
  ],
} satisfies LibraryMovieDetailsResponse;

function audioStream(id: number, streamIndex: number, language: string, title: string) {
  return {
    id,
    movie_id: movieId,
    stream_index: streamIndex,
    codec: "aac",
    codec_profile: nullableString("LC"),
    bit_rate: 192000,
    sample_rate: nullableInt64(48000),
    channels: 2,
    channel_layout: nullableString("stereo"),
    language: nullableString(language),
    title: nullableString(title),
    is_default: streamIndex === 1,
  };
}

function subtitle(id: number, streamIndex: number, codec: string, title: string) {
  return {
    id,
    movie_id: movieId,
    stream_index: streamIndex,
    codec,
    language: nullableString("en"),
    title: nullableString(title),
    is_forced: false,
    is_default: false,
  };
}

// 1080p H.264 High and AAC LC in MP4: Chromium plays it as-is, so direct play
// is the default. The Spanish track and the image-based subtitle exercise the
// playback settings dialog.
const technicalDetailsPayload = {
  movie: {
    file_name: "signal-fire.mp4",
    size: 5_100_000_000,
    container: "mp4",
    mime_type: "video/mp4",
    run_time: nullableInt64(126),
    duration: nullableFloat64(7560),
  },
  video_streams: [
    {
      id: 40,
      movie_id: movieId,
      stream_index: 0,
      codec: "h264",
      codec_profile: nullableString("High"),
      codec_level: nullableInt64(41),
      bit_rate: 6000000,
      width: 1920,
      height: 1080,
      coded_width: nullableInt64(1920),
      coded_height: nullableInt64(1080),
      aspect_ratio: nullableString("16:9"),
      frame_rate: 24,
      avg_frame_rate: nullableString("24/1"),
      bit_depth: nullableInt64(8),
      pixel_format: nullableString("yuv420p"),
      color_range: nullableString("tv"),
      color_space: nullableString("bt709"),
      color_primaries: nullableString("bt709"),
      color_transfer: nullableString("bt709"),
      field_order: nullableString("progressive"),
      rotation: nullableInt64(),
      language: nullableString("en"),
      title: nullableString(),
    },
  ],
  audio_streams: [
    audioStream(41, 1, "en", "English Stereo"),
    audioStream(42, 2, "es", "Spanish Stereo"),
  ],
  subtitles: [
    subtitle(60, 3, "subrip", "English"),
    subtitle(61, 4, "hdmv_pgs_subtitle", "English Signs"),
  ],
  chapters: [
    {
      id: 50,
      title: "Opening Credits",
      start_time: chapterStartSeconds,
      thumb: nullableString("/opening-credits.jpg"),
      movie_id: nullableInt64(movieId),
    },
  ],
} satisfies MovieTechnicalDetailsResponse;

const noWatchProgress: WatchProgressType = {
  progress_sec: null,
  duration_sec: null,
  watched: false,
  updated_at: null,
};

async function mockMovieDetailsApi(
  page: Page,
  { admin = false }: { admin?: boolean } = {},
) {
  // Per test: a metadata edit changes it, and the refetch must see the edit.
  const details = structuredClone(movieDetailsPayload);

  const { unexpectedApiRequests } = await mockApi(page, {
    // Only admins get the Edit action on the details page.
    user: admin ? { is_admin: true } : {},
    handle: async ({ route, url, method }) => {
      if (url.pathname === `/api/movies/${movieId}` && method === "PATCH") {
        const body = route.request().postDataJSON() as { title?: string };
        if (body.title !== undefined) details.movie.title = body.title;
        await fulfillJSON(route, apiResponse({}));
        return true;
      }

      if (method !== "GET") {
        return false;
      }

      const bodies: Record<string, unknown> = {
        "/api/movies/stats": { total_movies: 1 },
        "/api/tmdb/status": { available: false },
        "/api/movies/library": pagedList(url, "movies", [signalFire], {
          total: 1,
          perPage: MOVIES_PER_PAGE,
        }),
        [`/api/movies/details/${movieId}`]: details,
        [`/api/movies/${movieId}/technical-details`]: technicalDetailsPayload,
        [`/api/movies/${movieId}/like-status`]: { is_liked: false },
        [`/api/movies/${movieId}/watch-progress`]: noWatchProgress,
        "/api/settings/playback": {
          settings: {
            profiles: [],
            server_upload_mbps: null,
            hardware_acceleration_device: "cpu",
            effective_hardware_acceleration_device: "cpu",
            hardware_fallback_reason: "",
            max_transcode_height: 1080,
          },
        },
      };

      if (!(url.pathname in bodies)) {
        return false;
      }

      await fulfillJSON(route, apiResponse(bodies[url.pathname]));
      return true;
    },
  });

  return unexpectedApiRequests;
}

async function openPlaybackSettings(page: Page) {
  await page.getByRole("button", { name: "More options" }).click();
  await page.getByRole("menuitem", { name: "Playback Settings" }).click();

  const dialog = page.getByRole("dialog", { name: "Playback Settings" });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function pickOption(page: Page, trigger: Locator, option: string) {
  await trigger.click();
  await page.getByRole("listbox").getByRole("option", { name: option, exact: true }).click();
}

test("opens a movie from the index by keyboard and defaults its play links to direct play", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(moviesAllPath);

  await expect(page).toHaveTitle("Movies - Igloo");

  const movieTitleLink = page.getByRole("link", {
    name: "Signal Fire 2024",
    exact: true,
  });
  await expect(movieTitleLink).toBeVisible();
  await movieTitleLink.focus();
  await page.keyboard.press("Enter");

  await expect(page).toHaveTitle("Signal Fire (2024) - Igloo");
  await expect(page).toHaveURL(new RegExp(`${moviePath}/?$`));
  await expect(
    page.getByRole("heading", { name: /Signal Fire/i, level: 1 }),
  ).toBeVisible();

  // The hero drops the poster at lg+ (backdrop-as-hero); it only renders on
  // small viewports.
  const heroPoster = page.getByRole("img", {
    name: "Movie poster for Signal Fire",
  });
  await expect(heroPoster).toBeHidden();
  await page.setViewportSize(VIEWPORTS.phone);
  await expect(heroPoster).toBeVisible();
  await page.setViewportSize(VIEWPORTS.desktop);
  await expect(heroPoster).toBeHidden();

  // The file is direct-play eligible in Chromium, so the play and chapter
  // links start the original file with its first audio track.
  const directPlay = { mode: "direct", audio_track: "0", subtitle_track: "off" };
  await expectHref(page.getByRole("link", { name: "Play", exact: true }), playPath, directPlay);
  await expectHref(page.getByRole("link", { name: /Opening Credits/i }), playPath, {
    ...directPlay,
    start: String(chapterStartSeconds),
  });

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playback settings dialog saves a selection that drives the play links", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page);

  await page.goto(moviePath);
  await expect(
    page.getByRole("heading", { name: /Signal Fire/i, level: 1 }),
  ).toBeVisible();

  const moreOptionsButton = page.getByRole("button", { name: "More options" });
  const playLink = page.getByRole("link", { name: "Play", exact: true });
  const chapterLink = page.getByRole("link", { name: /Opening Credits/i });

  let dialog = await openPlaybackSettings(page);
  await expect(dialog.getByLabel("Playback")).toContainText("Original file — plays as-is");
  await pickOption(page, dialog.getByLabel("Playback"), "720p — lower bandwidth");
  await pickOption(page, dialog.getByLabel("Subtitles"), "English");
  await dialog.getByRole("button", { name: "Done" }).click();

  await expect(dialog).toBeHidden();
  // Closing the dialog must return focus to the menu trigger.
  await expect(moreOptionsButton).toBeFocused();
  await expectHref(playLink, playPath, {
    mode: "720p_3mbps",
    audio_track: "0",
    subtitle_track: "0",
  });

  // Reopening shows the saved selection as the draft. Direct play can only
  // deliver the container's first audio track, so choosing Spanish moves the
  // mode to remux; explicitly choosing no subtitles must stay authoritative in
  // every generated link.
  dialog = await openPlaybackSettings(page);
  await expect(dialog.getByLabel("Playback")).toContainText("720p — lower bandwidth");
  await pickOption(page, dialog.getByLabel("Playback"), "Original file — plays as-is");
  await pickOption(page, dialog.getByLabel("Audio Track"), "Spanish · Stereo");
  await expect(dialog.getByLabel("Playback")).toContainText("Original video, adjusted audio");
  await pickOption(page, dialog.getByLabel("Subtitles"), "None");
  await dialog.getByRole("button", { name: "Done" }).click();
  await expect(dialog).toBeHidden();

  const remuxSpanish = { mode: "remux", audio_track: "1", subtitle_track: "off" };
  await expectHref(playLink, playPath, remuxSpanish);
  await expectHref(chapterLink, playPath, {
    ...remuxSpanish,
    start: String(chapterStartSeconds),
  });

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("plays an extra video in the YouTube player and returns to the details page", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page);
  await mockYouTubePlayer(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(moviePath);

  await expect(page).toHaveTitle("Signal Fire (2024) - Igloo");
  await page.getByRole("link", { name: /Official Trailer/i }).click();

  // The clip opens the shared trailer dialog with the YouTube player.
  await expect(page).toHaveURL(
    `/trailer?videoKey=${extraVideoKey}&returnTo=${encodeURIComponent(moviePath)}`,
  );
  await expect(page.getByRole("dialog", { name: "Trailer" })).toBeVisible();

  // Starting playback flips the control to Pause, proving the player is playing.
  await playButton(page).click();
  await expect(
    page.getByRole("button", { name: "Pause (Space or K)" }),
  ).toBeVisible();

  // Closing the player returns to the originating movie details page.
  await page.keyboard.press("Escape");
  await expect(page).toHaveURL(moviePath);
  await expect(
    page.getByRole("heading", { name: /Signal Fire/i, level: 1 }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("editing the title retitles the tab", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page, { admin: true });

  await page.goto(moviePath);
  await expect(page).toHaveTitle("Signal Fire (2024) - Igloo");

  await page.getByRole("button", { name: "More options" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const dialog = page.getByRole("dialog", { name: "Edit Movie" });
  await dialog.getByRole("tab", { name: "Manual" }).click();
  await dialog.locator("#manual-title").fill("Signal Fire: Director's Cut");
  await dialog.getByRole("button", { name: "Save Changes" }).click();

  await expect(
    page.getByRole("heading", { level: 1, name: /Director's Cut/ }),
  ).toBeVisible();
  // The head only reruns on a router load: the dialog has to refetch the
  // details and then reload the route, or the tab keeps the old title.
  await expect(page).toHaveTitle("Signal Fire: Director's Cut (2024) - Igloo");

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
