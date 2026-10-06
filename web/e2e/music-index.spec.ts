import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { VIEWPORTS, expectNoOverflowingElements } from "./e2e-layout";
import {
  ALBUMS_PER_PAGE,
  LIKED_TRACKS_PER_PAGE,
  MOVIES_PER_PAGE,
  MUSICIANS_PER_PAGE,
  TRACKS_INFINITE_PAGE_SIZE,
} from "../src/lib/constants";
import {
  apiResponse,
  expectApiRequest,
  fulfillJSON,
  nullableInt64,
  nullableString,
  pagedList,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import { libraryMovie } from "./fixtures/movies";
import {
  playlistSummary,
  simpleAlbum,
  simpleMusician,
  trackListItem,
} from "./fixtures/music";

type CreatePlaylistRequest = {
  name: string;
  description?: string;
  is_public: boolean;
};

const mockAlbums = [
  simpleAlbum({
    id: 1,
    title: "First Mock Album",
    musician: nullableString("Aurora Pines"),
  }),
];

const pageOneMusicians = [
  simpleMusician({ id: 1, name: "Aurora Pines", album_count: 2, track_count: 18 }),
  simpleMusician({ id: 2, name: "Midnight Static", album_count: 1, track_count: 9 }),
];

const pageTwoMusicians = [
  simpleMusician({ id: 3, name: "Northern Signal", album_count: 3, track_count: 27 }),
];

function blueRecordTrack(id: number, title: string) {
  return trackListItem({
    id,
    title,
    album_id: nullableInt64(10),
    album_title: nullableString("Blue Record"),
    musician_id: nullableInt64(20),
    musician_name: nullableString("The Band"),
  });
}

const mockTracks = [
  blueRecordTrack(1, "Alabaster"),
  blueRecordTrack(2, "Borrowed Light"),
];

const likedTrackPages = {
  1: [
    trackListItem({
      id: 40,
      title: "Heartline",
      duration: 210,
      album_id: nullableInt64(41),
      album_title: nullableString("Warm Static"),
      musician_id: nullableInt64(42),
      musician_name: nullableString("Amber Field"),
    }),
  ],
  2: [
    trackListItem({
      id: 41,
      title: "Second Favorite",
      duration: 195,
      album_id: nullableInt64(43),
      album_title: nullableString("Late Catalog"),
      musician_id: nullableInt64(44),
      musician_name: nullableString("Cedar Room"),
    }),
  ],
};

const mockPlaylists = [
  playlistSummary({
    id: 30,
    name: "Morning Rotation",
    description: nullableString("Daily tracks for the first pass"),
    track_count: 3,
    total_duration: 540000,
  }),
  playlistSummary({
    id: 31,
    user_id: 2,
    name: "Shared Discoveries",
    description: nullableString("Tracks shared by another listener"),
    track_count: 2,
    total_duration: 420000,
    is_owner: false,
    can_edit: false,
  }),
];

type MockMusicIndexOptions = {
  /**
   * Also answer what the Movies sidebar link preloads on hover, and record
   * any in-theaters request instead of failing it.
   */
  preloadTargets?: boolean;
};

async function mockMusicIndexApi(
  page: Page,
  { preloadTargets = false }: MockMusicIndexOptions = {},
) {
  const playlists = [...mockPlaylists];
  const musicianRequests: URL[] = [];
  const movieRequests: URL[] = [];
  const inTheatersRequests: URL[] = [];
  const createdPlaylists: CreatePlaylistRequest[] = [];

  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url, method }) => {
      if (url.pathname === "/api/music/stats") {
        await fulfillJSON(route, apiResponse({
          total_albums: 6,
          total_tracks: 54,
          total_musicians: 3,
        }));
        return true;
      }

      if (url.pathname === "/api/spotify/status" && method === "GET") {
        await fulfillJSON(route, apiResponse({ available: true }));
        return true;
      }

      if (url.pathname === "/api/music/albums") {
        const pageNumber = Number(url.searchParams.get("page") ?? "1");
        const perPage = Number(
          url.searchParams.get("per_page") ?? String(ALBUMS_PER_PAGE),
        );

        await fulfillJSON(route, apiResponse({
          albums: mockAlbums,
          total: mockAlbums.length,
          page: pageNumber,
          per_page: perPage,
          total_pages: 1,
        }));
        return true;
      }

      if (url.pathname === "/api/music/musicians") {
        const musicianPage = Number(url.searchParams.get("page") ?? "1");
        const perPage = Number(
          url.searchParams.get("per_page") ?? String(MUSICIANS_PER_PAGE),
        );
        musicianRequests.push(url);

        await fulfillJSON(route, apiResponse({
          musicians: musicianPage === 2 ? pageTwoMusicians : pageOneMusicians,
          total: 3,
          page: musicianPage,
          per_page: perPage,
          total_pages: 2,
        }));
        return true;
      }

      if (preloadTargets && url.pathname === "/api/movies/stats") {
        movieRequests.push(url);
        await fulfillJSON(route, apiResponse({ total_movies: 1 }));
        return true;
      }

      if (preloadTargets && url.pathname === "/api/movies/library") {
        movieRequests.push(url);
        await fulfillJSON(route, apiResponse(pagedList(
          url,
          "movies",
          [libraryMovie(101, "Signal Fire", 2024)],
          { total: 1, perPage: MOVIES_PER_PAGE },
        )));
        return true;
      }

      if (preloadTargets && url.pathname === "/api/tmdb/movies/in-theaters") {
        inTheatersRequests.push(url);
        await fulfillJSON(route, apiResponse({ movies: [] }));
        return true;
      }

      if (url.pathname === "/api/music/playlists" && method === "GET") {
        await fulfillJSON(route, apiResponse({ playlists }));
        return true;
      }

      if (url.pathname === "/api/music/playlists" && method === "POST") {
        const body = route.request().postDataJSON() as CreatePlaylistRequest;
        createdPlaylists.push(body);

        const playlist = playlistSummary({
          id: 100 + playlists.length,
          name: body.name,
          description: nullableString(body.description ?? ""),
          is_public: body.is_public,
        });

        playlists.push(playlist);
        await fulfillJSON(route, apiResponse({ playlist }));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [2, 40, 41] }));
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked") {
        const likedTracksPage = Number(url.searchParams.get("page") ?? "1");
        const perPage = Number(
          url.searchParams.get("per_page") ?? String(LIKED_TRACKS_PER_PAGE),
        );

        await fulfillJSON(route, apiResponse({
          tracks: likedTrackPages[likedTracksPage as keyof typeof likedTrackPages] ?? [],
          total: 2,
          page: likedTracksPage,
          per_page: perPage,
          total_pages: 2,
          has_more: likedTracksPage < 2,
        }));
        return true;
      }

      if (url.pathname === "/api/music/tracks") {
        await fulfillJSON(route, apiResponse({
          tracks: mockTracks,
          total: mockTracks.length,
          offset: Number(url.searchParams.get("offset") ?? "0"),
          limit: Number(
            url.searchParams.get("limit") ?? String(TRACKS_INFINITE_PAGE_SIZE),
          ),
          has_more: false,
        }));
        return true;
      }

      return false;
    },
  });

  return {
    unexpectedApiRequests,
    musicianRequests,
    movieRequests,
    inTheatersRequests,
    createdPlaylists,
  };
}

test("musicians tab renders accessible count text and URL-backed pagination", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, musicianRequests } = await mockMusicIndexApi(page);
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=musicians");

  const musiciansTab = page.getByRole("tab", { name: "Musicians" });
  await expect(musiciansTab).toHaveAttribute("aria-selected", "true");

  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Midnight Static, 1 album, 9 tracks" })).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/musiciansPage=2/);
  await expectApiRequest(musicianRequests, "/api/music/musicians", {
    page: "2",
    per_page: String(MUSICIANS_PER_PAGE),
  });
  await expect(page.getByRole("link", { name: "Northern Signal, 3 albums, 27 tracks" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("musicians tab shows an inline error with a working retry", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMusicIndexApi(page);

  let failNextMusiciansRequest = true;
  await page.route(/\/api\/music\/musicians\?/, async route => {
    if (!failNextMusiciansRequest) {
      await route.fallback();
      return;
    }

    failNextMusiciansRequest = false;
    await fulfillJSON(route, {
      error: true,
      message: "Music library is unavailable",
    });
  });

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=musicians");

  const alert = page.getByRole("alert");
  await expect(alert).toContainText("Music library is unavailable");

  await alert.getByRole("button", { name: "Try again" }).click();

  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();
  await expect(alert).toBeHidden();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("Home sidebar links do not preload in-theaters data from music", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, movieRequests, inTheatersRequests } =
    await mockMusicIndexApi(page, { preloadTargets: true });
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=musicians");
  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();

  const mainNavigation = page.getByRole("navigation", { name: "Main navigation" });
  await page.getByRole("link", { name: /Igloo.*Home/ }).hover();
  await mainNavigation.getByRole("link", { name: "Home", exact: true }).hover();

  // Positive control: the other links preload on intent, so hovering Movies
  // must reach its loader. Only then does an empty in-theaters log prove the
  // Home links opted out rather than that preloading never ran.
  await mainNavigation.getByRole("link", { name: "Movies", exact: true }).hover();
  await expectApiRequest(movieRequests, "/api/movies/stats", {});
  await expectApiRequest(movieRequests, "/api/movies/library", { page: "1" });

  expect(inTheatersRequests).toEqual([]);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("music tabs avoid horizontal overflow on a phone", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMusicIndexApi(page);
  await page.setViewportSize(VIEWPORTS.phone);
  await page.goto("/music?tab=albums");

  await expect(page.getByRole("link", { name: "First Mock Album by Aurora Pines" })).toBeVisible();
  await expectNoOverflowingElements(page);

  await page.getByRole("tab", { name: "Musicians" }).click();
  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();
  await expectNoOverflowingElements(page);

  await page.getByRole("tab", { name: "Tracks" }).click();
  await expect(page.getByRole("button", { name: "Add Alabaster to liked" })).toBeVisible();
  await expectNoOverflowingElements(page);

  await page.getByRole("tab", { name: "Playlists" }).click();
  await expect(page.getByRole("link", { name: "Morning Rotation, 3 tracks, 9m 0s" })).toBeVisible();
  await expectNoOverflowingElements(page);

  await page.getByRole("button", { name: "View liked tracks" }).click();
  await expect(page.getByRole("button", { name: "Remove Heartline from liked" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();
  await expectNoOverflowingElements(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("tracks tab opens a track's action menu with playlist, album and artist targets", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMusicIndexApi(page);
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=tracks");

  await expect(page.getByRole("tab", { name: "Tracks" })).toHaveAttribute("aria-selected", "true");

  const moreActions = page.getByRole("button", { name: "More actions for Alabaster" });
  await moreActions.click();

  await expect(page.getByRole("menuitem", { name: "Add to Playlist" })).toBeVisible();
  const goToAlbum = page.getByRole("menuitem", { name: "Go to Album" });
  const goToArtist = page.getByRole("menuitem", { name: "Go to Artist" });
  await expect(goToAlbum).toHaveAttribute("href", "/music/album/10");
  await expect(goToArtist).toHaveAttribute("href", "/music/musician/20");

  // Escape closes the menu and hands focus back to the trigger.
  await page.keyboard.press("Escape");
  await expect(goToAlbum).toBeHidden();
  await expect(moreActions).toBeFocused();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab creates a playlist from the toolbar dialog", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests, createdPlaylists } = await mockMusicIndexApi(page);
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=playlists");

  await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("link", { name: "Morning Rotation, 3 tracks, 9m 0s" })).toBeVisible();

  const createPlaylistButton = page.getByRole("button", { name: "Create new playlist" });
  await createPlaylistButton.click();

  const dialog = page.getByRole("dialog", { name: "Create New Playlist" });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name").fill("Fresh Queue");
  await dialog.getByLabel("Description").fill("Songs to sort later");
  await dialog.getByRole("button", { name: "Create Playlist" }).click();

  await expect.poll(() => createdPlaylists).toEqual([
    {
      name: "Fresh Queue",
      description: "Songs to sort later",
      is_public: false,
    },
  ]);
  await expect(dialog).toBeHidden();
  await expect(createPlaylistButton).toBeFocused();

  // The list refetches and shows what the server now holds.
  await expect(page.getByRole("link", { name: /^Fresh Queue, / })).toBeVisible();
  await expect(page.getByText("3 playlists", { exact: true })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab opens liked tracks subview with URL-backed pagination", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockMusicIndexApi(page);
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=playlists");

  await page.getByRole("button", { name: "View liked tracks" }).click();

  await expect(page).toHaveURL(/tab=playlists/);
  await expect(page).toHaveURL(/playlistsView=liked/);
  await expect(page.getByRole("button", { name: "Back to playlists" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Liked Tracks" })).toBeVisible();
  await expect(page.getByText("2 tracks")).toBeVisible();
  await expect(page.getByText("Heartline")).toBeVisible();
  await expect(page.getByRole("button", { name: "Remove Heartline from liked" })).toBeVisible();
  await expect(page.getByRole("button", { name: "More actions for Heartline" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Heartline" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/likedTracksPage=2/);
  await expect(page.getByText("Second Favorite")).toBeVisible();
  await expect(page.getByRole("button", { name: "Remove Second Favorite from liked" })).toBeVisible();

  await page.getByRole("button", { name: "Back to playlists" }).click();

  await expect(page).not.toHaveURL(/playlistsView=liked/);
  await expect(page).not.toHaveURL(/likedTracksPage=2/);
  await expect(page).toHaveURL(/tab=playlists/);
  await expect(page.getByRole("button", { name: "View liked tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Create new playlist" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
