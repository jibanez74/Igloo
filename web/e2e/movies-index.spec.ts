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
  MoviePlaylistRowType,
  MoviePlaylistSummaryType,
  MoviesLibraryListItemType,
  UpdateMoviePlaylistRequest,
} from "../src/types";
import {
  apiResponse,
  fulfillJSON,
  gateRoute,
  nullableString,
  pagedList,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  fillLibraryPage,
  libraryMovie,
  movieCardPreload,
  moviePlaylist,
} from "./fixtures/movies";

// The movie-only half of the library page: playlists and the liked view. The
// tabs, grid, genres, pagination and sort both libraries share are covered by
// library-index.spec.ts.

const moviesPlaylistsPath =
  "/movies?tab=playlists&allPage=1&sort=asc&genresPage=1&playlistsPage=1";

const signalFire = libraryMovie(101, "Signal Fire", 2024, "/signal-fire.jpg");
const quietHarbor = libraryMovie(102, "Quiet Harbor", 2022);

// The route loader also warms the All Movies tab, so the library list serves
// the same pages as the liked view.
const moviePages = [
  fillLibraryPage(
    [signalFire, quietHarbor],
    { prefix: "Liked Mock", startId: 3000, perPage: MOVIES_PER_PAGE },
    libraryMovie,
  ),
  [libraryMovie(201, "Verdant Run", 2025, "/verdant-run.jpg")],
];

const moviesById = new Map(moviePages.flat().map(movie => [movie.id, movie]));

const initialPlaylists = [
  moviePlaylist(
    501,
    "Friday Feature",
    7,
    true,
    "Movies queued for the end of the week",
    "/api/static/playlists/friday-feature.jpg",
  ),
  moviePlaylist(502, "Guest Picks", 3, false, "Shared picks from another account"),
];

/** The playlist row the detail and mutation envelopes carry: the summary without its counts. */
function moviePlaylistRow(playlist: MoviePlaylistSummaryType): MoviePlaylistRowType {
  return {
    id: playlist.id,
    user_id: playlist.user_id,
    name: playlist.name,
    description: playlist.description,
    cover_image: playlist.cover_image,
    is_public: playlist.is_public,
    movie_id: playlist.movie_id,
    content_type: playlist.content_type,
    created_at: playlist.created_at,
    updated_at: playlist.updated_at,
  };
}

function playlistNotFound(playlistId: number) {
  return { error: true, message: `Movie playlist ${playlistId} not found` };
}

async function mockMoviesApi(page: Page) {
  const playlists = [...initialPlaylists];
  // The owned playlist's first page; its count stays the summary's own number.
  const playlistMovies = new Map<number, MoviesLibraryListItemType[]>([
    [501, [signalFire, quietHarbor]],
  ]);
  const createdPlaylistRequests: CreateMoviePlaylistRequest[] = [];
  const updatedPlaylistRequests: UpdateMoviePlaylistRequest[] = [];
  const deletedPlaylistIds: number[] = [];
  const removedMovieRequests: { playlistId: number; movieId: number }[] = [];

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

      const ownedMatch = url.pathname.match(/^\/api\/movies\/playlists\/(\d+)$/);
      if (ownedMatch && method === "PUT") {
        const body = route.request().postDataJSON() as UpdateMoviePlaylistRequest;
        updatedPlaylistRequests.push(body);

        // A full replace, like the server: a field the request leaves out is cleared.
        const index = playlists.findIndex(candidate => candidate.id === Number(ownedMatch[1]));
        playlists[index] = {
          ...playlists[index],
          name: body.name,
          description: nullableString(body.description ?? ""),
          cover_image: nullableString(body.cover_image ?? ""),
          is_public: body.is_public ?? false,
        };
        await fulfillJSON(route, apiResponse({ playlist: moviePlaylistRow(playlists[index]) }));
        return true;
      }

      if (ownedMatch && method === "DELETE") {
        const playlistId = Number(ownedMatch[1]);
        deletedPlaylistIds.push(playlistId);
        playlists.splice(playlists.findIndex(candidate => candidate.id === playlistId), 1);
        await fulfillJSON(route, { error: false, message: "Playlist deleted successfully" });
        return true;
      }

      const removal = url.pathname.match(/^\/api\/movies\/playlists\/(\d+)\/movies\/(\d+)$/);
      if (removal && method === "DELETE") {
        const playlistId = Number(removal[1]);
        const movieId = Number(removal[2]);
        removedMovieRequests.push({ playlistId, movieId });
        playlistMovies.set(
          playlistId,
          (playlistMovies.get(playlistId) ?? []).filter(candidate => candidate.id !== movieId),
        );
        const index = playlists.findIndex(candidate => candidate.id === playlistId);
        playlists[index] = { ...playlists[index], movie_count: playlists[index].movie_count - 1 };
        await fulfillJSON(route, { error: false, message: "Movie removed from playlist" });
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
          await fulfillJSON(route, apiResponse(pagedList(url, "movies", playlistMovies.get(playlistId) ?? [], {
            total: playlist.movie_count,
            perPage: MOVIES_PER_PAGE,
          })));
          return true;
        }

        await fulfillJSON(route, apiResponse({
          playlist: moviePlaylistRow(playlist),
          movie_count: playlist.movie_count,
          is_owner: playlist.is_owner,
          can_edit: playlist.can_edit,
          owner: { id: playlist.user_id, name: "Playlist Owner", avatar: null },
          collaborators: null,
        } satisfies MoviePlaylistDetailResponseType));
        return true;
      }

      return false;
    },
  });

  return {
    createdPlaylistRequests,
    updatedPlaylistRequests,
    deletedPlaylistIds,
    removedMovieRequests,
    unexpectedApiRequests,
  };
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

  const dialog = page.getByRole("dialog", { name: "Create New Playlist" });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name").fill("Roadshow Queue");
  await dialog.getByLabel("Description (optional)").fill("Titles waiting for a group watch");
  await dialog.getByRole("button", { name: "Create Playlist" }).click();

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

test("renaming a movie playlist from its page retitles the tab", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { updatedPlaylistRequests, unexpectedApiRequests } = await mockMoviesApi(page);

  await page.goto("/movies/playlist/501");

  await expect(page.getByRole("heading", { level: 1, name: "Friday Feature" })).toBeVisible();
  await expect(page).toHaveTitle("Friday Feature - Igloo");

  const editButton = page.getByRole("button", { name: "Edit playlist" });
  await editButton.click();
  const dialog = page.getByRole("dialog", { name: "Edit Playlist" });
  await expect(dialog.getByRole("textbox", { name: /^Name/ })).toHaveValue("Friday Feature");
  await expect(dialog.getByLabel("Description (optional)")).toHaveValue(
    "Movies queued for the end of the week",
  );
  await dialog.getByRole("textbox", { name: /^Name/ }).fill("Friday Night Feature");
  await dialog.getByRole("button", { name: "Save Changes" }).click();

  // PUT replaces every field, so the page must resend what it did not edit.
  await expect.poll(() => updatedPlaylistRequests).toEqual([
    {
      name: "Friday Night Feature",
      description: "Movies queued for the end of the week",
      cover_image: "/api/static/playlists/friday-feature.jpg",
      is_public: false,
    },
  ]);
  await expect(dialog).toBeHidden();
  await expect(editButton).toBeFocused();
  await expect(page.getByRole("heading", { level: 1, name: "Friday Night Feature" })).toBeVisible();
  // The head only reruns on a router load: the dialog refetches the details
  // and then reloads the route, or the tab keeps the old name.
  await expect(page).toHaveTitle("Friday Night Feature - Igloo");

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("deleting a movie playlist from its page returns to the playlists tab", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { deletedPlaylistIds, unexpectedApiRequests } = await mockMoviesApi(page);

  await page.goto("/movies/playlist/501");

  await page.getByRole("button", { name: "Delete playlist" }).click();
  const confirm = page.getByRole("alertdialog", { name: "Delete playlist" });
  await expect(confirm).toContainText("Are you sure you want to delete “Friday Feature”?");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect.poll(() => deletedPlaylistIds).toEqual([501]);
  await expect(page).toHaveURL(/\/movies\?.*tab=playlists/);
  await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByText("Playlist deleted")).toBeVisible();
  await expect(page.getByRole("link", { name: "Guest Picks, 3 movies" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Friday Feature, 7 movies" })).toHaveCount(0);
  await expect(page.getByText("1 playlist", { exact: true })).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("a viewer who does not own a movie playlist sees no Edit or Delete", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMoviesApi(page);

  await page.goto("/movies/playlist/502");

  await expect(page.getByRole("heading", { level: 1, name: "Guest Picks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit playlist" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Delete playlist" })).toHaveCount(0);

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("removing a movie from a playlist page drops its card and updates the count", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { removedMovieRequests, unexpectedApiRequests } = await mockMoviesApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/movies/playlist/501");

  await expect(page.getByText("7 movies")).toBeVisible();
  const card = cardFor(page, "Quiet Harbor 2022");
  const menuButton = card.getByRole("button", { name: "More actions for Quiet Harbor" });
  // The card's menu reveals with the card, like its play control.
  await expect(menuButton.locator("..")).toHaveCSS("opacity", "0");
  await card.hover();
  await expect(menuButton.locator("..")).toHaveCSS("opacity", "1");

  await menuButton.click();
  await page.getByRole("menuitem", { name: "Remove from Playlist" }).click();

  await expect.poll(() => removedMovieRequests).toEqual([{ playlistId: 501, movieId: 102 }]);
  await expect(page.getByText("Movie removed from playlist")).toBeVisible();
  await expect(card).toHaveCount(0);
  await expect(cardFor(page, "Signal Fire 2024")).toBeVisible();
  await expect(page.getByText("6 movies")).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test.describe("on a touch phone", () => {
  test.use({ viewport: VIEWPORTS.phone, hasTouch: true, isMobile: true });

  // Nothing hovers on a touch screen, so the card menu is always shown there.
  test("the playlist card menu is shown without hover", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);
    const { unexpectedApiRequests } = await mockMoviesApi(page);

    await page.goto("/movies/playlist/501");

    const menuButton = page.getByRole("button", { name: "More actions for Signal Fire" });
    await expect(menuButton).toBeVisible();
    await expect(menuButton.locator("..")).toHaveCSS("opacity", "1");

    assertMockSuiteClean(browserIssues, unexpectedApiRequests);
  });
});

test("switching to the playlists tab commits at once and shows its skeleton", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMoviesApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/movies");

  const playlistsTab = page.getByRole("tab", { name: "Playlists" });
  await expect(playlistsTab).toBeVisible();

  const playlistsList = await gateRoute(page, /\/api\/movies\/playlists$/);
  await playlistsTab.click();

  // The loader only starts an in-page tab's query, so the tab is selected
  // while the playlists load and the grid's skeleton covers the wait.
  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("status", { name: "Loading playlists" })).toBeVisible();

  playlistsList.release();
  await expect(page.getByText("2 playlists")).toBeVisible();

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
  await expect(page.getByText("25 liked movies", { exact: true })).toBeVisible();
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
