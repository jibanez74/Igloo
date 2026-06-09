import { expect, test, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";

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

type CreateMoviePlaylistRequest = {
  name: string;
  description?: string;
  is_public?: boolean;
  movie_id?: number;
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

function movie(
  id: number,
  title: string,
  year: number,
  posterPath = "",
) {
  return {
    id,
    title,
    poster_path: nullableString(posterPath),
    year: nullableInt64(year),
    certification: nullableString("PG-13"),
  };
}

function buildMoviePage(
  featuredMovies: ReturnType<typeof movie>[],
  fillerPrefix: string,
  fillerStartId: number,
) {
  return [
    ...featuredMovies,
    ...Array.from({ length: 24 - featuredMovies.length }, (_, index) =>
      movie(
        fillerStartId + index,
        `${fillerPrefix} ${index + 1}`,
        2000 + ((index + 1) % 20),
        index % 2 === 0 ? `/${fillerPrefix.toLowerCase().replaceAll(" ", "-")}-${index + 1}.jpg` : "",
      ),
    ),
  ];
}

function moviePlaylist(
  id: number,
  name: string,
  movieCount: number,
  isOwner: boolean,
  description: string,
) {
  return {
    id,
    user_id: isOwner ? 1 : 2,
    name,
    description: nullableString(description),
    cover_image: nullableString(),
    is_public: false,
    folder_id: nullableInt64(),
    movie_id: nullableInt64(),
    content_type: "movie",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    movie_count: movieCount,
    is_owner: isOwner,
    can_edit: isOwner,
  };
}

const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const moviesGenresPath =
  "/movies?tab=genres&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const moviesPlaylistsPath =
  "/movies?tab=playlists&allPage=1&sort=asc&genresPage=1&playlistsPage=1";

const libraryPageOneMovies = buildMoviePage(
  [
    movie(101, "Signal Fire", 2024, "/signal-fire.jpg"),
    movie(102, "Quiet Harbor", 2022),
  ],
  "Library Mock",
  1000,
);

const libraryPageTwoMovies = [
  movie(201, "Verdant Run", 2025, "/verdant-run.jpg"),
];

const actionPageOneMovies = buildMoviePage(
  [
    movie(301, "Sky Relay", 2025, "/sky-relay.jpg"),
    movie(302, "Cinder Avenue", 2021),
  ],
  "Action Mock",
  2000,
);

const actionPageTwoMovies = [
  movie(325, "Afterburn", 2020, "/afterburn.jpg"),
];

const dramaPageOneMovies = [
  movie(401, "Quiet Harbor", 2022),
];

const likedPageOneMovies = buildMoviePage(
  [
    movie(101, "Signal Fire", 2024, "/signal-fire.jpg"),
    movie(102, "Quiet Harbor", 2022),
  ],
  "Liked Mock",
  3000,
);

const likedPageTwoMovies = [
  movie(201, "Verdant Run", 2025, "/verdant-run.jpg"),
];

const movieGenres = [
  {
    genre_id: 10,
    genre_tag: "Action",
    movie_count: 25,
  },
  {
    genre_id: 20,
    genre_tag: "Drama",
    movie_count: 1,
  },
];

const initialPlaylists = [
  moviePlaylist(
    501,
    "Friday Feature",
    7,
    true,
    "Movies queued for the end of the week",
  ),
  moviePlaylist(
    502,
    "Guest Picks",
    3,
    false,
    "Shared picks from another account",
  ),
];

const mockMoviesById = new Map(
  [
    ...libraryPageOneMovies,
    ...libraryPageTwoMovies,
    ...actionPageOneMovies,
    ...actionPageTwoMovies,
    ...dramaPageOneMovies,
    ...likedPageOneMovies,
    ...likedPageTwoMovies,
  ].map(movieEntry => [movieEntry.id, movieEntry]),
);

function movieDetails(movieSummary: ReturnType<typeof movie>) {
  return {
    movie: {
      id: movieSummary.id,
      title: movieSummary.title,
      file_path: `/movies/${movieSummary.id}.mkv`,
      file_name: `${movieSummary.title.toLowerCase().replaceAll(" ", "-")}.mkv`,
      size: 0,
      container: "matroska",
      mime_type: "video/x-matroska",
      adult: false,
      tmdb_id: nullableInt64(movieSummary.id),
      imdb_id: nullableString(`tt${movieSummary.id.toString().padStart(7, "0")}`),
      poster_path: movieSummary.poster_path,
      backdrop_path: nullableString(),
      language: nullableString("en"),
      year: movieSummary.year,
      release_date: nullableString(`${movieSummary.year.Int64}-01-01`),
      overview: nullableString(`${movieSummary.title} overview`),
      tag_line: nullableString(),
      certification: movieSummary.certification,
      critic_rating: nullableFloat64(),
      audience_rating: nullableFloat64(),
      revenue: nullableFloat64(),
      budget: nullableFloat64(),
      run_time: nullableInt64(120),
      duration: nullableFloat64(7200),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    cast: [],
    crew: [],
    genres: [],
    production_companies: [],
    extra_videos: [],
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

async function mockMoviesApi(
  page: Page,
  requestedLibraryRequests: string[],
  requestedGenreRequests: string[],
  createdPlaylistRequests: CreateMoviePlaylistRequest[] = [],
) {
  const playlists = [...initialPlaylists];
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname.startsWith("/api/tmdb/images/")) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="150" viewBox="0 0 100 150"><rect width="100" height="150" fill="#f59e0b"/><rect x="12" y="12" width="76" height="126" rx="10" fill="#0f172a"/><circle cx="50" cy="50" r="18" fill="#f8fafc"/><rect x="24" y="92" width="52" height="10" rx="5" fill="#f8fafc"/><rect x="30" y="110" width="40" height="8" rx="4" fill="#fbbf24"/></svg>`,
      });
      return;
    }

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Movies User",
          email: "movies@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/movies/stats") {
      await fulfillJSON(route, apiResponse({ total_movies: 25 }));
      return;
    }

    const movieDetailsMatch = url.pathname.match(/^\/api\/movies\/details\/(\d+)$/);
    if (movieDetailsMatch) {
      const movieId = Number(movieDetailsMatch[1]);
      const movieSummary = mockMoviesById.get(movieId);

      if (!movieSummary) {
        await fulfillJSON(
          route,
          {
            error: true,
            message: `Movie ${movieId} not found`,
          },
          404,
        );
        return;
      }

      await fulfillJSON(route, apiResponse(movieDetails(movieSummary)));
      return;
    }

    if (url.pathname === "/api/movies/library") {
      const libraryPage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedLibraryRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        movies: libraryPage === 2 ? libraryPageTwoMovies : libraryPageOneMovies,
        total: 25,
        page: libraryPage,
        per_page: perPage,
        total_pages: 2,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/movies/genres") {
      await fulfillJSON(route, apiResponse({ genres: movieGenres }));
      return;
    }

    if (url.pathname === "/api/movies/genres/10/movies") {
      const genrePage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedGenreRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        movies: genrePage === 2 ? actionPageTwoMovies : actionPageOneMovies,
        total: 25,
        page: genrePage,
        per_page: perPage,
        total_pages: 2,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/movies/genres/20/movies") {
      const genrePage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedGenreRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        movies: dramaPageOneMovies,
        total: 1,
        page: genrePage,
        per_page: perPage,
        total_pages: 1,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/movies/playlists") {
      if (method === "GET") {
        await fulfillJSON(route, apiResponse({ playlists }));
        return;
      }

      if (method === "POST") {
        const body = route.request().postDataJSON() as CreateMoviePlaylistRequest;
        createdPlaylistRequests.push(body);

        const playlist = moviePlaylist(
          600 + playlists.length,
          body.name,
          0,
          true,
          body.description ?? "",
        );

        playlists.push(playlist);
        await fulfillJSON(route, apiResponse({ playlist }));
        return;
      }

      const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
      unexpectedApiRequests.push(message);
      await fulfillJSON(route, { error: true, message }, 405);
      return;
    }

    const playlistDetailsMatch = url.pathname.match(/^\/api\/movies\/playlists\/(\d+)$/);
    if (playlistDetailsMatch) {
      const playlistId = Number(playlistDetailsMatch[1]);
      const playlist = playlists.find(candidate => candidate.id === playlistId);

      if (!playlist) {
        await fulfillJSON(
          route,
          {
            error: true,
            message: `Movie playlist ${playlistId} not found`,
          },
          404,
        );
        return;
      }

      const { movie_count, is_owner, can_edit, ...playlistRow } = playlist;
      await fulfillJSON(route, apiResponse({
        playlist: playlistRow,
        movie_count,
        is_owner,
        can_edit,
        collaborators: null,
      }));
      return;
    }

    const playlistMoviesMatch = url.pathname.match(
      /^\/api\/movies\/playlists\/(\d+)\/movies$/,
    );
    if (playlistMoviesMatch) {
      const playlistId = Number(playlistMoviesMatch[1]);
      const playlist = playlists.find(candidate => candidate.id === playlistId);
      const page = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";

      if (!playlist) {
        await fulfillJSON(
          route,
          {
            error: true,
            message: `Movie playlist ${playlistId} not found`,
          },
          404,
        );
        return;
      }

      await fulfillJSON(route, apiResponse({
        movies: [],
        total: playlist.movie_count,
        page,
        per_page: perPage,
        total_pages: Math.max(1, Math.ceil(playlist.movie_count / perPage)),
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/movies/liked") {
      const likedPage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";

      await fulfillJSON(route, apiResponse({
        movies: likedPage === 2 ? likedPageTwoMovies : likedPageOneMovies,
        total: 25,
        page: likedPage,
        per_page: perPage,
        total_pages: 2,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/tmdb/status") {
      await fulfillJSON(route, apiResponse({ available: true }));
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
  });

  return unexpectedApiRequests;
}

test("movies library shell and URL-backed tabs render accessibly", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(moviesAllPath);

  await expect(page).toHaveTitle("Movies - Igloo");
  await expect(page.getByRole("heading", { name: "Movie Library", level: 1 })).toBeVisible();
  await expect(page.getByRole("region", { name: "Library statistics: 25 movies" })).toBeVisible();

  const tablist = page.getByRole("tablist");
  await expect(tablist).toBeVisible();
  await expect(page.getByRole("tab")).toHaveCount(3);

  const allMoviesTab = page.getByRole("tab", { name: "All Movies" });
  const genresTab = page.getByRole("tab", { name: "Genres" });
  const playlistsTab = page.getByRole("tab", { name: "Playlists" });

  await expect(allMoviesTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "All Movies" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Signal Fire 2024", exact: true })).toBeVisible();

  await genresTab.click();

  await expect(page).toHaveURL(/tab=genres/);
  await expect(genresTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
  await expect(page.getByRole("list", { name: "Movie genres" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Action 25 movies" })).toBeVisible();

  await playlistsTab.click();

  await expect(page).toHaveURL(/tab=playlists/);
  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "Playlists" })).toBeVisible();
  await expect(page.getByText("2 playlists")).toBeVisible();
  await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
  await expect(page.getByRole("button", { name: "New playlist" })).toBeVisible();

  await allMoviesTab.click();

  await expect(page).toHaveURL(/tab=all/);
  await expect(allMoviesTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "All Movies" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("all movies tab renders accessible movie cards and URL-backed pagination", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(moviesAllPath);

  const signalFireLink = page.getByRole("link", {
    name: "Signal Fire 2024",
    exact: true,
  });
  const quietHarborLink = page.getByRole("link", {
    name: "Quiet Harbor 2022",
    exact: true,
  });

  await expect(signalFireLink).toBeVisible();
  await expect(quietHarborLink).toBeVisible();
  await expect(page.getByRole("link", { name: "Play Signal Fire 2024" })).toHaveAttribute("href", "/movies/101/play");
  await expect(page.getByRole("link", { name: "Play Quiet Harbor 2022" })).toHaveAttribute("href", "/movies/102/play");

  const posterBackedCard = signalFireLink.locator("xpath=ancestor::article");
  const posterlessCard = quietHarborLink.locator("xpath=ancestor::article");

  await expect(posterBackedCard.locator("img")).toHaveAttribute(
    "src",
    /\/api\/tmdb\/images\/w500\/signal-fire\.jpg$/,
  );
  await expect(posterlessCard.locator("img")).toHaveCount(0);

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/allPage=2/);
  await expect
    .poll(() =>
      requestedLibraryRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/movies/library" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === "24" &&
          parsed.searchParams.get("sort") === "asc"
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Verdant Run 2025", exact: true })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("genres tab renders accessible counts, filtering, and URL-backed pagination", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(moviesGenresPath);

  const genresTab = page.getByRole("tab", { name: "Genres" });
  const genresList = page.getByRole("list", { name: "Movie genres" });
  const actionButton = page.getByRole("button", { name: "Action 25 movies" });

  await expect(genresTab).toHaveAttribute("aria-selected", "true");
  await expect(genresList).toBeVisible();
  await expect(actionButton).toBeVisible();
  await expect(page.getByRole("button", { name: "Drama 1 movie" })).toBeVisible();

  await actionButton.click();

  await expect(page).toHaveURL(/genreId=10/);
  await expect(page.getByText("Sky Relay")).toBeVisible();
  const clearGenreFilterButton = page.getByRole("button", {
    name: "Clear genre filter",
  });
  await expect(
    clearGenreFilterButton.locator(
      "xpath=preceding-sibling::span[contains(., '25 movies')]",
    ),
  ).toBeVisible();
  await expect(clearGenreFilterButton).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/genresPage=2/);
  await expect
    .poll(() =>
      requestedGenreRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/movies/genres/10/movies" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === "24" &&
          parsed.searchParams.get("sort") === "asc"
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Afterburn 2020", exact: true })).toBeVisible();

  await clearGenreFilterButton.click();

  await expect(page).not.toHaveURL(/genreId=/);
  await expect(page).not.toHaveURL(/genresPage=2/);
  await expect(genresList).toBeVisible();
  await expect(page.getByRole("link", { name: "Afterburn 2020", exact: true })).toHaveCount(0);
  await expect(actionButton).toBeFocused();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("movies tabs avoid horizontal overflow on mobile", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(moviesAllPath);

  const tablist = page.getByRole("tablist");
  const stats = page.getByRole("region", { name: "Library statistics: 25 movies" });
  const allMoviesLink = page.getByRole("link", {
    name: "Signal Fire 2024",
    exact: true,
  });
  await expect(allMoviesLink).toBeVisible();
  const allMoviesCard = allMoviesLink.locator("xpath=ancestor::article");
  const allMoviesGrid = allMoviesCard.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "movies tablist");
  await expectNoHorizontalOverflow(stats, "movies stats");
  await expectNoHorizontalOverflow(allMoviesGrid, "all movies grid");
  await expectNoHorizontalOverflow(allMoviesCard, "all movies card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "all movies pagination");

  await page.getByRole("tab", { name: "Genres" }).click();

  const genresList = page.getByRole("list", { name: "Movie genres" });
  const actionButton = page.getByRole("button", { name: "Action 25 movies" });
  await expect(actionButton).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "movies tablist");
  await expectNoHorizontalOverflow(stats, "movies stats");
  await expectNoHorizontalOverflow(genresList, "genres grid");
  await expectNoHorizontalOverflow(actionButton, "genre button");

  await actionButton.click();

  const selectedGenreCard = page
    .getByRole("link", { name: "Sky Relay 2025", exact: true })
    .locator("xpath=ancestor::article");
  const selectedGenreGrid = selectedGenreCard.locator("xpath=parent::*");
  const selectedGenreHeader = page
    .getByRole("button", { name: "Clear genre filter" })
    .locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' mb-5 ')][1]");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(selectedGenreHeader, "selected genre header");
  await expectNoHorizontalOverflow(selectedGenreGrid, "selected genre results");
  await expectNoHorizontalOverflow(selectedGenreCard, "selected genre card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "selected genre pagination");

  await page.getByRole("tab", { name: "Playlists" }).click();

  const likedMoviesButton = page.getByRole("button", { name: "Liked movies" });
  const playlistLink = page.getByRole("link", { name: "Friday Feature, 7 movies" });
  await expect(playlistLink).toBeVisible();
  const playlistCard = playlistLink.locator("xpath=ancestor::article");
  const playlistGrid = playlistCard.locator("xpath=parent::*");
  const playlistsToolbar = likedMoviesButton.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "movies tablist");
  await expectNoHorizontalOverflow(stats, "movies stats");
  await expectNoHorizontalOverflow(playlistsToolbar, "playlists toolbar");
  await expectNoHorizontalOverflow(playlistGrid, "playlist grid");
  await expectNoHorizontalOverflow(playlistCard, "playlist card");

  await likedMoviesButton.click();

  const likedHeader = page
    .getByRole("button", { name: "Back to playlists" })
    .locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' mb-6 ')][1]");
  const likedMovieCard = page
    .getByRole("link", { name: "Signal Fire 2024", exact: true })
    .locator("xpath=ancestor::article");
  const likedMoviesGrid = likedMovieCard.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "movies tablist");
  await expectNoHorizontalOverflow(stats, "movies stats");
  await expectNoHorizontalOverflow(likedHeader, "liked movies header");
  await expectNoHorizontalOverflow(likedMoviesGrid, "liked movies grid");
  await expectNoHorizontalOverflow(likedMovieCard, "liked movies card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "liked movies pagination");
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab lists playlists and creates a playlist from the toolbar dialog", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const createdPlaylistRequests: CreateMoviePlaylistRequest[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
    createdPlaylistRequests,
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(moviesPlaylistsPath);

  const playlistsTab = page.getByRole("tab", { name: "Playlists" });
  const createPlaylistButton = page.getByRole("button", { name: "New playlist" });

  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("2 playlists")).toBeVisible();

  const ownedPlaylist = page.getByRole("link", { name: "Friday Feature, 7 movies" });
  const sharedPlaylist = page.getByRole("link", { name: "Guest Picks, 3 movies" });
  await expect(ownedPlaylist).toBeVisible();
  await expect(sharedPlaylist).toBeVisible();
  await expect(ownedPlaylist.locator("xpath=ancestor::article").getByText("Owner")).toBeVisible();
  await expect(sharedPlaylist.locator("xpath=ancestor::article").getByText("Owner")).toHaveCount(0);
  await expect(page.getByText("Owner")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
  await expect(createPlaylistButton).toBeVisible();

  await createPlaylistButton.click();

  const dialog = page.getByRole("dialog", { name: "New movie playlist" });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name").fill("Roadshow Queue");
  await dialog.getByLabel("Description (optional)").fill("Titles waiting for a group watch");
  await dialog.getByRole("button", { name: "Create" }).click();

  await expect.poll(() => createdPlaylistRequests).toEqual([
    {
      name: "Roadshow Queue",
      description: "Titles waiting for a group watch",
      is_public: false,
    },
  ]);
  await expect(dialog).toBeHidden();
  await expect(createPlaylistButton).toBeFocused();
  await expect(page.getByRole("link", { name: "Roadshow Queue, 0 movies" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab opens liked movies subview with URL-backed pagination", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMoviesApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(moviesPlaylistsPath);

  await page.getByRole("button", { name: "Liked movies" }).click();

  await expect(page).toHaveURL(/tab=playlists/);
  await expect(page).toHaveURL(/view=liked/);
  await expect(page.getByRole("button", { name: "Back to playlists" })).toBeVisible();
  await expect(page.getByText("25 liked")).toBeVisible();
  await expect(page.getByRole("link", { name: "Signal Fire 2024", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Play Signal Fire 2024" })).toHaveAttribute("href", "/movies/101/play");
  await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/playlistsPage=2/);
  await expect(page.getByRole("link", { name: "Verdant Run 2025", exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Play Verdant Run 2025" })).toHaveAttribute("href", "/movies/201/play");

  await page.getByRole("button", { name: "Back to playlists" }).click();

  await expect(page).not.toHaveURL(/view=liked/);
  await expect(page).not.toHaveURL(/playlistsPage=2/);
  await expect(page).toHaveURL(/tab=playlists/);
  await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
  await expect(page.getByRole("button", { name: "New playlist" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
