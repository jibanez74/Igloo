import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { cardFor, expectNoOverflowingElements, VIEWPORTS } from "./e2e-layout";
import { MOVIES_PER_PAGE } from "../src/lib/constants";
import type {
  CreateMoviePlaylistRequest,
  MoviePlaylistDetailResponseType,
  MoviePlaylistSummaryType,
} from "../src/types";
import {
  apiResponse,
  fulfillJSON,
  nullableInt64,
  nullableString,
  pagedList,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  fillLibraryPage,
  libraryMovie,
  movieCardPreload,
} from "./fixtures/movies";

// The movie-only half of the library page: playlists and the liked view. The
// tabs, grid, genres, pagination and sort both libraries share are covered by
// library-index.spec.ts.

function moviePlaylist(
  id: number,
  name: string,
  movieCount: number,
  isOwner: boolean,
  description: string,
): MoviePlaylistSummaryType {
  return {
    id,
    user_id: isOwner ? 1 : 2,
    name,
    description: nullableString(description),
    cover_image: nullableString(),
    is_public: false,
    movie_id: nullableInt64(),
    content_type: "movie",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    movie_count: movieCount,
    is_owner: isOwner,
    can_edit: isOwner,
  };
}

const moviesPlaylistsPath =
  "/movies?tab=playlists&allPage=1&sort=asc&genresPage=1&playlistsPage=1";

// The route loader also warms the All Movies tab, so the library list serves
// the same pages as the liked view.
const moviePages = [
  fillLibraryPage(
    [
      libraryMovie(101, "Signal Fire", 2024, "/signal-fire.jpg"),
      libraryMovie(102, "Quiet Harbor", 2022),
    ],
    { prefix: "Liked Mock", startId: 3000, perPage: MOVIES_PER_PAGE },
    libraryMovie,
  ),
  [libraryMovie(201, "Verdant Run", 2025, "/verdant-run.jpg")],
];

const moviesById = new Map(moviePages.flat().map(movie => [movie.id, movie]));

const initialPlaylists = [
  moviePlaylist(501, "Friday Feature", 7, true, "Movies queued for the end of the week"),
  moviePlaylist(502, "Guest Picks", 3, false, "Shared picks from another account"),
];

function playlistNotFound(playlistId: number) {
  return { error: true, message: `Movie playlist ${playlistId} not found` };
}

async function mockMoviesApi(page: Page) {
  const playlists = [...initialPlaylists];
  const createdPlaylistRequests: CreateMoviePlaylistRequest[] = [];

  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url, method }) => {
      if (url.pathname === "/api/movies/playlists" && method === "POST") {
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
        return true;
      }

      if (method !== "GET") {
        return false;
      }

      if (url.pathname === "/api/movies/stats") {
        await fulfillJSON(route, apiResponse({ total_movies: 25 }));
        return true;
      }

      if (url.pathname === "/api/tmdb/status") {
        await fulfillJSON(route, apiResponse({ available: true }));
        return true;
      }

      if (url.pathname === "/api/movies/playlists") {
        await fulfillJSON(route, apiResponse({ playlists }));
        return true;
      }

      if (url.pathname === "/api/movies/library" || url.pathname === "/api/movies/liked") {
        const movies = moviePages[url.searchParams.get("page") === "2" ? 1 : 0];
        await fulfillJSON(route, apiResponse(pagedList(url, "movies", movies, {
          total: 25,
          perPage: MOVIES_PER_PAGE,
        })));
        return true;
      }

      const preload = movieCardPreload(url.pathname, moviesById);
      if (preload !== undefined) {
        await fulfillJSON(route, apiResponse(preload));
        return true;
      }

      // Pointing at a playlist card preloads the playlist route's two queries.
      const playlistMatch = url.pathname.match(/^\/api\/movies\/playlists\/(\d+)(\/movies)?$/);
      if (playlistMatch) {
        const playlistId = Number(playlistMatch[1]);
        const playlist = playlists.find(candidate => candidate.id === playlistId);

        if (!playlist) {
          await fulfillJSON(route, playlistNotFound(playlistId), 404);
          return true;
        }

        if (playlistMatch[2]) {
          await fulfillJSON(route, apiResponse(pagedList(url, "movies", [], {
            total: playlist.movie_count,
            perPage: MOVIES_PER_PAGE,
          })));
          return true;
        }

        const { movie_count, is_owner, can_edit, ...playlistRow } = playlist;
        await fulfillJSON(route, apiResponse({
          playlist: playlistRow,
          movie_count,
          is_owner,
          can_edit,
          collaborators: null,
        } satisfies MoviePlaylistDetailResponseType));
        return true;
      }

      return false;
    },
  });

  return { createdPlaylistRequests, unexpectedApiRequests };
}

test("playlists tab lists playlists and creates a playlist from the toolbar dialog", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { createdPlaylistRequests, unexpectedApiRequests } = await mockMoviesApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(moviesPlaylistsPath);

  const playlistsTab = page.getByRole("tab", { name: "Playlists" });
  const createPlaylistButton = page.getByRole("button", { name: "New playlist" });

  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("2 playlists")).toBeVisible();

  const ownedPlaylist = "Friday Feature, 7 movies";
  const sharedPlaylist = "Guest Picks, 3 movies";
  await expect(page.getByRole("link", { name: ownedPlaylist })).toBeVisible();
  await expect(page.getByRole("link", { name: sharedPlaylist })).toBeVisible();
  await expect(cardFor(page, ownedPlaylist).getByText("Owner")).toBeVisible();
  await expect(cardFor(page, sharedPlaylist).getByText("Owner")).toHaveCount(0);
  await expect(page.getByText("Owner")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();

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
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMoviesApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
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

test("playlists and liked views avoid horizontal overflow on a phone", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMoviesApi(page);

  await page.setViewportSize(VIEWPORTS.phone);
  await page.goto(moviesPlaylistsPath);

  await expect(page.getByRole("link", { name: "Friday Feature, 7 movies" })).toBeVisible();
  await expectNoOverflowingElements(page);

  await page.getByRole("button", { name: "Liked movies" }).click();

  await expect(page.getByRole("link", { name: "Signal Fire 2024", exact: true })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();
  await expectNoOverflowingElements(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
