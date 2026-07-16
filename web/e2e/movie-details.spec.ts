import { expect, test, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { MOVIES_PER_PAGE } from "../src/lib/constants";
import type { MovieWatchProgressType } from "../src/types";

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

const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const movieId = 711;
const chapterStartSeconds = 372;
const extraVideoKey = "signal-fire-trailer";
const noWatchProgress: MovieWatchProgressType = {
  progress_sec: null,
  duration_sec: null,
  watched: false,
  updated_at: null,
};

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
    {
      id: 11,
      movie_id: movieId,
      artist_id: 2002,
      job: "Writer",
      department: "Writing",
      artist_name: "Casey North",
      artist_profile: nullableString("/casey-north.jpg"),
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
      title: "Official Trailer",
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
  chapters: [
    {
      id: 50,
      title: "Opening Credits",
      start_time: chapterStartSeconds,
      thumb: nullableString("/opening-credits.jpg"),
      movie_id: nullableInt64(movieId),
    },
  ],
};

async function mockMovieDetailsApi(
  page: Page,
  watchProgress: MovieWatchProgressType = noWatchProgress,
) {
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
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="240" viewBox="0 0 160 240"><rect width="160" height="240" fill="#f59e0b"/><rect x="14" y="14" width="132" height="212" rx="12" fill="#0f172a"/><circle cx="80" cy="78" r="26" fill="#f8fafc"/><rect x="34" y="146" width="92" height="14" rx="7" fill="#f8fafc"/><rect x="42" y="170" width="76" height="10" rx="5" fill="#fbbf24"/></svg>`,
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
      await fulfillJSON(route, apiResponse(watchProgress));
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

test("movie details page renders the mocked success path from the movies index", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page);

  await page.setViewportSize({ width: 1440, height: 1200 });
  await page.goto(moviesAllPath);

  await expect(page).toHaveTitle("Movies - Igloo");
  await expect(
    page.getByRole("heading", { name: "Movie Library", level: 1 }),
  ).toBeVisible();

  const movieTitleLink = page.getByRole("link", {
    name: "Signal Fire 2024",
    exact: true,
  });
  await expect(movieTitleLink).toBeVisible();
  await movieTitleLink.focus();
  await page.keyboard.press("Enter");

  await expect(page).toHaveTitle("Signal Fire (2024) - Igloo");
  expect(new URL(page.url()).pathname).toMatch(/^\/movies\/711\/?$/);

  await expect(
    page.getByRole("heading", { name: /Signal Fire/i, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "A rescue pilot returns to a coastal town and uncovers the wildfire cover-up that drove her family apart.",
    ),
  ).toBeVisible();
  // The hero drops the poster at lg+ (backdrop-as-hero); it only renders on
  // small viewports.
  const heroPoster = page.getByRole("img", {
    name: "Movie poster for Signal Fire",
  });
  await expect(heroPoster).toBeHidden();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(heroPoster).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1200 });
  await expect(heroPoster).toBeHidden();

  const metadataRow = page.getByRole("list", { name: "Movie details" });
  await expect(metadataRow).toBeVisible();
  await expect(metadataRow).toContainText("PG-13");
  await expect(metadataRow).toContainText("2 hr 6 min");
  await expect(metadataRow).toContainText("July 4, 2024");
  const runtime = metadataRow.getByLabel("Runtime: 2 hours 6 minutes");
  await expect(runtime).toHaveAttribute("datetime", "PT126M");

  const playLink = page.getByRole("link", { name: "Play" });
  const watchButton = page.getByRole("button", {
    name: "Mark movie as watched",
  });
  const likeButton = page.getByRole("button", { name: "Like this movie" });
  const moreOptionsButton = page.getByRole("button", { name: "More options" });

  await expect(playLink).toBeVisible();
  await expect(watchButton).toBeVisible();
  await expect(likeButton).toBeVisible();
  await expect(moreOptionsButton).toBeVisible();

  const skipLinksNav = page.getByRole("navigation", { name: "Skip to section" });
  await expect(skipLinksNav).toHaveCount(1);

  for (const [label, href] of [
    ["Skip to movie info", "#movie-title"],
    ["Skip to overview", "#overview-heading"],
    ["Skip to key crew", "#crew-heading"],
    ["Skip to cast", "#cast-heading"],
    ["Skip to chapters", "#chapters-heading"],
    ["Skip to extra videos", "#extra-videos-heading"],
    ["Skip to about", "#details-heading"],
  ] as const) {
    await expect(skipLinksNav.getByRole("link", { name: label })).toHaveAttribute(
      "href",
      href,
    );
  }

  await expect(page.getByRole("heading", { name: "Key Crew" })).toBeVisible();
  await expect(page.getByText("Jordan Lee")).toBeVisible();
  await expect(page.getByText("Casey North")).toBeVisible();

  await expect(page.getByRole("heading", { name: "Cast" })).toBeVisible();
  await expect(
    page.getByRole("article", { name: "Alex Vega as Mara Voss" }),
  ).toBeVisible();

  await expect(page.getByRole("heading", { name: "Chapters" })).toBeVisible();
  await expect(
    page.getByRole("list", { name: "Chapters, 1 total" }),
  ).toBeVisible();

  await expect(
    page.getByRole("heading", { name: "Extra Videos" }),
  ).toBeVisible();
  await expect(
    page.getByRole("list", { name: "Extra videos, 1 clips" }),
  ).toBeVisible();

  await expect(
    page.getByRole("heading", { name: "About Signal Fire" }),
  ).toBeVisible();
  const aboutSection = page.locator("section", {
    has: page.getByRole("heading", { name: "About Signal Fire" }),
  });
  await expect(aboutSection.getByText("Original language")).toBeVisible();
  await expect(aboutSection.getByText("EN", { exact: true })).toBeVisible();
  await expect(aboutSection.getByText("$95,000,000")).toBeVisible();
  await expect(aboutSection.getByText("$215,000,000")).toBeVisible();
  await expect(aboutSection.getByText("Northwind Pictures")).toBeVisible();

  const playHref = await playLink.getAttribute("href");
  expect(playHref).not.toBeNull();
  const playUrl = new URL(playHref ?? "", "http://localhost");
  expect(playUrl.pathname).toBe(`/movies/${movieId}/play`);
  expect(playUrl.searchParams.get("mode")).toBe("direct");
  expect(playUrl.searchParams.get("audio_track")).toBe("0");
  expect(playUrl.searchParams.get("subtitle_track")).toBeNull();

  const chapterLink = page.getByRole("link", { name: /Opening Credits/i });
  await expect(chapterLink).toBeVisible();
  const chapterHref = await chapterLink.getAttribute("href");
  expect(chapterHref).not.toBeNull();
  const chapterUrl = new URL(chapterHref ?? "", "http://localhost");
  expect(chapterUrl.pathname).toBe(`/movies/${movieId}/play`);
  expect(chapterUrl.searchParams.get("mode")).toBe("direct");
  expect(chapterUrl.searchParams.get("audio_track")).toBe("0");
  expect(chapterUrl.searchParams.get("start")).toBe(
    String(chapterStartSeconds),
  );

  const extraVideoLink = page.getByRole("link", { name: /Official Trailer/i });
  await expect(extraVideoLink).toBeVisible();
  const extraVideoHref = await extraVideoLink.getAttribute("href");
  expect(extraVideoHref).not.toBeNull();
  const extraVideoUrl = new URL(extraVideoHref ?? "", "http://localhost");
  expect(extraVideoUrl.pathname).toBe("/trailer");
  expect(extraVideoUrl.searchParams.get("videoKey")).toBe(extraVideoKey);
  expect(extraVideoUrl.searchParams.get("returnTo")).toBe(`/movies/${movieId}`);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("movie details page renders eligible resume progress", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockMovieDetailsApi(page, {
    progress_sec: 1890,
    duration_sec: 7560,
    watched: false,
    updated_at: "2026-07-16T12:00:00Z",
  });

  await page.goto(`/movies/${movieId}`);

  const minutesLeft = page.getByText("95 min left", { exact: true });
  await expect(minutesLeft).toBeVisible();

  const progressFill = minutesLeft
    .locator("..")
    .locator(":scope > div[aria-hidden='true'] > div");
  await expect(progressFill).toHaveAttribute("style", /width:\s*25%/);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

for (const { state, watchProgress, watchedButtonName } of [
  {
    state: "watched",
    watchProgress: {
      progress_sec: 1890,
      duration_sec: 7560,
      watched: true,
      updated_at: "2026-07-16T12:00:00Z",
    },
    watchedButtonName: "Mark movie as unwatched",
  },
  {
    state: "completed",
    watchProgress: {
      progress_sec: 980,
      duration_sec: 1000,
      watched: false,
      updated_at: "2026-07-16T12:00:00Z",
    },
    watchedButtonName: "Mark movie as watched",
  },
] satisfies {
  state: string;
  watchProgress: MovieWatchProgressType;
  watchedButtonName: string;
}[]) {
  test(`movie details page suppresses resume progress when ${state}`, async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);
    const unexpectedApiRequests = await mockMovieDetailsApi(page, watchProgress);

    await page.goto(`/movies/${movieId}`);

    await expect(
      page.getByRole("button", { name: watchedButtonName }),
    ).toBeEnabled();
    await expect(page.getByText(/^\d+ min left$/)).toHaveCount(0);

    assertMockSuiteClean(browserIssues, unexpectedApiRequests);
  });
}
