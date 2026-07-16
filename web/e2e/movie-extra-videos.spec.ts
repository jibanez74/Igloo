import { expect, test, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { mockYouTubePlayer } from "./mock-youtube-player";
import { MOVIES_PER_PAGE } from "../src/lib/constants";

type NullableString = {
  String: string;
  Valid: boolean;
};

type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

type NullableFloat64 = {
  Float64: number;
  Valid: boolean;
};

function nullableString(value = ""): NullableString {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function nullableFloat64(value: number | null = null): NullableFloat64 {
  return {
    Float64: value ?? 0,
    Valid: value != null,
  };
}

function apiResponse(data: unknown) {
  return {
    error: false,
    data,
  };
}

async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function assertMockSuiteClean(
  browserIssues: ReturnType<typeof trackBrowserIssues>,
  unexpectedApiRequests: string[],
) {
  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
}

const movieId = 711;
const moviePath = `/movies/${movieId}`;
const extraVideoKey = "signal-fire-trailer";
const extraVideoTitle = "Official Trailer";

const libraryMovie = {
  id: movieId,
  title: "Signal Fire",
  poster_path: nullableString("/signal-fire-poster.jpg"),
  year: nullableInt64(2024),
  certification: nullableString("PG-13"),
};

const movieDetailsPayload = {
  movie: {
    id: movieId,
    title: libraryMovie.title,
    file_path: "/library/movies/signal-fire.mp4",
    file_name: "signal-fire.mp4",
    size: 5_100_000_000,
    container: "mp4",
    mime_type: "video/mp4",
    adult: false,
    tmdb_id: nullableInt64(1711),
    imdb_id: nullableString("tt1711000"),
    poster_path: libraryMovie.poster_path,
    backdrop_path: nullableString("/signal-fire-backdrop.jpg"),
    language: nullableString("en"),
    year: libraryMovie.year,
    release_date: nullableString("2024-07-04T12:00:00Z"),
    overview: nullableString(
      "A rescue pilot returns to a coastal town and uncovers the wildfire cover-up that drove her family apart.",
    ),
    tag_line: nullableString("Some fires never fade."),
    certification: libraryMovie.certification,
    critic_rating: nullableFloat64(8.7),
    audience_rating: nullableFloat64(8.2),
    revenue: nullableFloat64(215000000),
    budget: nullableFloat64(95000000),
    run_time: nullableInt64(126),
    duration: nullableFloat64(7560),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  cast: [
    {
      id: 1,
      movie_id: movieId,
      artist_id: 1001,
      character: "Mara Voss",
      cast_order: 0,
      artist_name: "Alex Vega",
      artist_profile: nullableString("/alex-vega.jpg"),
    },
  ],
  crew: [
    {
      id: 10,
      movie_id: movieId,
      artist_id: 2001,
      job: "Director",
      department: "Directing",
      artist_name: "Jordan Lee",
      artist_profile: nullableString("/jordan-lee.jpg"),
    },
  ],
  genres: [
    {
      id: 30,
      tag: "Thriller",
    },
  ],
  production_companies: [
    {
      id: 20,
      name: "Northwind Pictures",
      tmdb_id: 2020,
      logo: nullableString("/northwind-pictures.png"),
      country: nullableString("US"),
    },
  ],
  extra_videos: [
    {
      id: 30,
      title: extraVideoTitle,
      external_id: nullableString("yt-signal-fire-trailer"),
      key: extraVideoKey,
      type: "trailer",
      site: "youtube",
      official: true,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  ],
};

const technicalDetailsPayload = {
  movie: {
    file_name: "signal-fire.mp4",
    file_path: "/library/movies/signal-fire.mp4",
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
      bit_rate: 6000000,
      width: 1920,
      height: 1080,
    },
  ],
  audio_streams: [
    {
      id: 41,
      movie_id: movieId,
      stream_index: 1,
      codec: "aac",
      bit_rate: 192000,
      channels: 2,
      channel_layout: nullableString("stereo"),
      language: nullableString("en"),
      title: nullableString("English Stereo"),
    },
  ],
  subtitles: [],
  chapters: [],
};

async function mockMovieDetailsApi(page: Page) {
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (
      url.pathname.startsWith("/api/tmdb/images/") ||
      url.pathname.startsWith("/api/youtube/thumbnails/")
    ) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="240" viewBox="0 0 160 240"><rect width="160" height="240" fill="#f59e0b"/><rect x="14" y="14" width="132" height="212" rx="12" fill="#0f172a"/></svg>`,
      });
      return;
    }

    if (method !== "GET") {
      const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
      unexpectedApiRequests.push(message);
      await fulfillJSON(route, { error: true, message }, 405);
      return;
    }

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Movie User",
          email: "movies@example.com",
          is_admin: false,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/notifications/unread-count") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

    if (url.pathname === "/api/movies/stats") {
      await fulfillJSON(route, apiResponse({ total_movies: 1 }));
      return;
    }

    if (url.pathname === "/api/tmdb/status") {
      await fulfillJSON(route, apiResponse({ available: false }));
      return;
    }

    if (url.pathname === "/api/movies/library") {
      const pageNumber = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(
        url.searchParams.get("per_page") ?? String(MOVIES_PER_PAGE),
      );
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";

      await fulfillJSON(route, apiResponse({
        movies: [libraryMovie],
        total: 1,
        page: pageNumber,
        per_page: perPage,
        total_pages: 1,
        sort,
      }));
      return;
    }

    if (url.pathname === `/api/movies/details/${movieId}`) {
      await fulfillJSON(route, apiResponse(movieDetailsPayload));
      return;
    }

    if (url.pathname === `/api/movies/${movieId}/technical-details`) {
      await fulfillJSON(route, apiResponse(technicalDetailsPayload));
      return;
    }

    if (url.pathname === `/api/movies/${movieId}/like-status`) {
      await fulfillJSON(route, apiResponse({ is_liked: false }));
      return;
    }

    if (url.pathname === `/api/movies/${movieId}/watch-progress`) {
      await fulfillJSON(route, apiResponse({
        progress_sec: null,
        duration_sec: null,
        watched: false,
        updated_at: null,
      }));
      return;
    }

    if (url.pathname === "/api/settings/playback") {
      await fulfillJSON(route, apiResponse({
        settings: {
          profiles: [],
          preferred_profile: null,
          download_mbps: null,
          server_upload_mbps: null,
          is_admin: false,
          preferred_audio_language: null,
          preferred_subtitle_language: null,
        },
      }));
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
  });

  return unexpectedApiRequests;
}

test("plays a movie extra video in the YouTube player and returns to the details page", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page);
  await mockYouTubePlayer(page);

  await page.setViewportSize({ width: 1440, height: 1200 });
  await page.goto(moviePath);

  await expect(page).toHaveTitle("Signal Fire (2024) - Igloo");

  // The Extra Videos section renders the movie's YouTube clips.
  await expect(
    page.getByRole("heading", { name: "Extra Videos" }),
  ).toBeVisible();
  await expect(
    page.getByRole("list", { name: "Extra videos, 1 clips" }),
  ).toBeVisible();

  const extraVideoLink = page.getByRole("link", { name: /Official Trailer/i });
  await expect(extraVideoLink).toBeVisible();
  await extraVideoLink.click();

  // Clicking the clip opens the shared trailer dialog with the YouTube player.
  await expect(page).toHaveURL(
    `/trailer?videoKey=${extraVideoKey}&returnTo=${encodeURIComponent(moviePath)}`,
  );
  await expect(page.getByRole("dialog", { name: "Trailer" })).toBeVisible();

  const playButton = page.getByRole("button", { name: "Play (Space or K)" });
  await expect(playButton).toBeVisible();

  // Starting playback flips the control to Pause, proving the player is playing.
  await playButton.click();
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
