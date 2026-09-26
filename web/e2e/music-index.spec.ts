import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  VIEWPORTS,
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";
import {
  ALBUMS_PER_PAGE,
  LIKED_TRACKS_PER_PAGE,
  MUSICIANS_PER_PAGE,
  TRACKS_INFINITE_PAGE_SIZE,
} from "../src/lib/constants";
import {
  apiResponse,
  fulfillJSON,
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import {
  playlistSummary,
  simpleAlbum,
  simpleMusician,
  trackListItem,
  type MusicianDetails,
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

const mockMusicianDetails = {
  musician: {
    id: 1,
    name: "Aurora Pines",
    sort_name: "Aurora Pines",
    summary: nullableString("Layered ambient pop with long descriptive copy for the tablet hero layout."),
    spotify_popularity: nullableFloat64(82),
    spotify_followers: nullableInt64(42000),
    spotify_id: nullableString("spotify-aurora-pines"),
    thumb: nullableString(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  albums: [
    {
      id: 10,
      title: "Blue Record",
      cover: nullableString(),
      year: nullableInt64(2026),
      release_date: nullableString("2026-01-01"),
      track_count: 2,
    },
  ],
  tracks: mockTracks.map(track => ({
    id: track.id,
    title: track.title,
    duration: track.duration,
    codec: track.codec,
    bit_rate: track.bit_rate,
    album_id: track.album_id,
    album_title: track.album_title,
    album_cover: track.album_cover,
  })),
  genres: ["Ambient", "Pop", "Electronic"],
  total_duration: 360000,
} satisfies MusicianDetails;

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

async function mockMusicIndexApi(
  page: Page,
  requestedMusicianRequests: string[],
  createdPlaylistRequests: CreatePlaylistRequest[] = [],
  requestedInTheatersRequests?: string[],
) {
  const playlists = [...mockPlaylists];

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
        requestedMusicianRequests.push(`${url.pathname}${url.search}`);

        await fulfillJSON(route, apiResponse({
          musicians: musicianPage === 2 ? pageTwoMusicians : pageOneMusicians,
          total: 3,
          page: musicianPage,
          per_page: perPage,
          total_pages: 2,
        }));
        return true;
      }

      if (url.pathname === "/api/music/musicians/1") {
        await fulfillJSON(route, apiResponse(mockMusicianDetails));
        return true;
      }

      if (url.pathname === "/api/tmdb/movies/in-theaters" && requestedInTheatersRequests) {
        requestedInTheatersRequests.push(`${url.pathname}${url.search}`);
        await fulfillJSON(route, apiResponse({ movies: [] }));
        return true;
      }

      if (url.pathname === "/api/music/playlists" && method === "GET") {
        await fulfillJSON(route, apiResponse({ playlists }));
        return true;
      }

      if (url.pathname === "/api/music/playlists" && method === "POST") {
        const body = route.request().postDataJSON() as CreatePlaylistRequest;
        createdPlaylistRequests.push(body);

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

  return unexpectedApiRequests;
}

test("musicians tab renders accessible count text and URL-backed pagination", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=musicians");

  const musiciansTab = page.getByRole("tab", { name: "Musicians" });
  await expect(musiciansTab).toHaveAttribute("aria-selected", "true");

  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Midnight Static, 1 album, 9 tracks" })).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/musiciansPage=2/);
  await expect
    .poll(() =>
      requestedMusicianRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/music/musicians" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === String(MUSICIANS_PER_PAGE)
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Northern Signal, 3 albums, 27 tracks" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("musicians tab shows an inline error with a working retry", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );

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
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("musician details keeps hero controls inside tablet viewport", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );
  await page.setViewportSize(VIEWPORTS.tablet);
  await page.goto("/music/musician/1");

  const playAllButton = page.getByRole("button", {
    name: "Play all 2 tracks by Aurora Pines",
    exact: true,
  });
  const shuffleButton = page.getByRole("button", {
    name: "Shuffle play all 2 tracks by Aurora Pines",
    exact: true,
  });

  await expect(playAllButton).toBeVisible();
  await expect(shuffleButton).toBeVisible();
  await expectNoHorizontalOverflow(playAllButton, "Play All button");
  await expectNoHorizontalOverflow(shuffleButton, "Shuffle button");
  await expectPageHasNoHorizontalScroll(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("Home sidebar links do not preload in-theaters data from music", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const requestedInTheatersRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
    [],
    requestedInTheatersRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=musicians");

  await page.getByRole("link", { name: /Igloo.*Home/ }).hover();
  await page.getByRole("link", { name: /Igloo.*Home/ }).focus();
  await page.getByRole("link", { name: "Home", exact: true }).hover();
  await page.getByRole("link", { name: "Home", exact: true }).focus();
  await page.waitForTimeout(250);

  expect(requestedInTheatersRequests).toEqual([]);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("music tabs avoid horizontal overflow on mobile", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );
  await page.setViewportSize(VIEWPORTS.phone);
  await page.goto("/music?tab=albums");

  const tablist = page.getByRole("tablist");
  const stats = page.getByRole("region", { name: /Library statistics:/ });

  const albumLink = page.getByRole("link", { name: "First Mock Album by Aurora Pines" });
  await expect(albumLink).toBeVisible();
  const albumCard = albumLink.locator("xpath=ancestor::article");
  const albumGrid = albumCard.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "music tablist");
  await expectNoHorizontalOverflow(stats, "music stats");
  await expectNoHorizontalOverflow(albumGrid, "album card grid");
  await expectNoHorizontalOverflow(albumCard, "album card");

  await page.getByRole("tab", { name: "Musicians" }).click();

  const musicianLink = page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" });
  await expect(musicianLink).toBeVisible();
  const musicianCard = musicianLink.locator("xpath=ancestor::article");
  const musicianGrid = musicianCard.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "music tablist");
  await expectNoHorizontalOverflow(stats, "music stats");
  await expectNoHorizontalOverflow(musicianGrid, "musician card grid");
  await expectNoHorizontalOverflow(musicianCard, "musician card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "musicians pagination");

  await page.getByRole("tab", { name: "Tracks" }).click();

  const tracksList = page.getByRole("list", { name: "Tracks" });
  const addLikedButton = page.getByRole("button", { name: "Add Alabaster to liked" });
  await expect(addLikedButton).toBeVisible();
  const tracksToolbar = page
    .getByRole("button", { name: "Play all tracks" })
    .locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' mb-4 ')][1]");
  const firstTrackRow = addLikedButton.locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' group ')][1]");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "music tablist");
  await expectNoHorizontalOverflow(stats, "music stats");
  await expectNoHorizontalOverflow(tracksToolbar, "tracks toolbar");
  await expectNoHorizontalOverflow(tracksList, "tracks list");
  await expectNoHorizontalOverflow(firstTrackRow, "first track row");
  await expectNoHorizontalOverflow(addLikedButton, "track liked action");
  await expectNoHorizontalOverflow(page.getByRole("button", { name: "More actions for Alabaster" }), "track more actions");
  await expectNoHorizontalOverflow(page.getByRole("button", { name: "Play Alabaster" }), "track play action");

  await page.getByRole("tab", { name: "Playlists" }).click();

  const likedTracksButton = page.getByRole("button", { name: "View liked tracks" });
  const createPlaylistButton = page.getByRole("button", { name: "Create new playlist" });
  const playlistLink = page.getByRole("link", { name: "Morning Rotation, 3 tracks, 9m 0s" });
  await expect(playlistLink).toBeVisible();
  const playlistCard = playlistLink.locator("xpath=ancestor::article");
  const playlistGrid = playlistCard.locator("xpath=parent::*");
  const playlistControls = likedTracksButton.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "music tablist");
  await expectNoHorizontalOverflow(stats, "music stats");
  await expectNoHorizontalOverflow(playlistControls, "playlist controls");
  await expectNoHorizontalOverflow(likedTracksButton, "view liked tracks button");
  await expectNoHorizontalOverflow(createPlaylistButton, "create playlist button");
  await expectNoHorizontalOverflow(playlistGrid, "playlist card grid");
  await expectNoHorizontalOverflow(playlistCard, "playlist card");

  await likedTracksButton.click();

  const backToPlaylistsButton = page.getByRole("button", { name: "Back to playlists" });
  const removeLikedButton = page.getByRole("button", { name: "Remove Heartline from liked" });
  await expect(removeLikedButton).toBeVisible();
  const likedTracksHeader = backToPlaylistsButton.locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' mb-6 ')][1]");
  const likedTracksList = removeLikedButton.locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' overflow-hidden ')][1]");
  const firstLikedTrackRow = removeLikedButton.locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' group ')][1]");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "music tablist");
  await expectNoHorizontalOverflow(stats, "music stats");
  await expectNoHorizontalOverflow(likedTracksHeader, "liked tracks header");
  await expectNoHorizontalOverflow(likedTracksList, "liked tracks list");
  await expectNoHorizontalOverflow(firstLikedTrackRow, "first liked track row");
  await expectNoHorizontalOverflow(removeLikedButton, "liked track liked action");
  await expectNoHorizontalOverflow(page.getByRole("button", { name: "More actions for Heartline" }), "liked track more actions");
  await expectNoHorizontalOverflow(page.getByRole("button", { name: "Play Heartline" }), "liked track play action");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "liked tracks pagination");
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("tracks tab exposes accessible controls and action menu targets", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=tracks");

  const tracksTab = page.getByRole("tab", { name: "Tracks" });
  await expect(tracksTab).toHaveAttribute("aria-selected", "true");

  await expect(page.getByRole("list", { name: "Tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play all tracks" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Shuffle all tracks" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Add Alabaster to liked" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Remove Borrowed Light from liked" })).toBeVisible();

  await page.getByRole("button", { name: "More actions for Alabaster" }).click();

  await expect(page.getByRole("menuitem", { name: "Add to Playlist" })).toBeVisible();
  const goToAlbum = page.getByRole("menuitem", { name: "Go to Album" });
  const goToArtist = page.getByRole("menuitem", { name: "Go to Artist" });

  await expect(goToAlbum).toBeVisible();
  await expect(goToArtist).toBeVisible();
  await expect(goToAlbum).toHaveAttribute("href", "/music/album/10");
  await expect(goToArtist).toHaveAttribute("href", "/music/musician/20");
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab lists playlists and creates a playlist from the toolbar dialog", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const createdPlaylistRequests: CreatePlaylistRequest[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
    createdPlaylistRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/music?tab=playlists");

  const playlistsTab = page.getByRole("tab", { name: "Playlists" });
  await expect(playlistsTab).toHaveAttribute("aria-selected", "true");

  await expect(page.getByText("2 playlists")).toBeVisible();

  const ownedPlaylist = page.getByRole("link", { name: "Morning Rotation, 3 tracks, 9m 0s" });
  const sharedPlaylist = page.getByRole("link", { name: "Shared Discoveries, 2 tracks, 7m 0s" });
  await expect(ownedPlaylist).toBeVisible();
  await expect(sharedPlaylist).toBeVisible();
  await expect(ownedPlaylist.locator("xpath=ancestor::article").getByText("Owner")).toBeVisible();
  await expect(sharedPlaylist.locator("xpath=ancestor::article").getByText("Owner")).toHaveCount(0);
  await expect(page.getByText("Owner")).toHaveCount(1);

  const likedTracksButton = page.getByRole("button", { name: "View liked tracks" });
  await expect(likedTracksButton).toBeVisible();
  await expect(likedTracksButton).toContainText("Liked tracks");

  const createPlaylistButton = page.getByRole("button", { name: "Create new playlist" });
  await expect(createPlaylistButton).toBeVisible();
  await expect(createPlaylistButton).toContainText("New playlist");

  await createPlaylistButton.click();

  const dialog = page.getByRole("dialog", { name: "Create New Playlist" });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name").fill("Fresh Queue");
  await dialog.getByLabel("Description").fill("Songs to sort later");
  await dialog.getByRole("button", { name: "Create Playlist" }).click();

  await expect.poll(() => createdPlaylistRequests).toEqual([
    {
      name: "Fresh Queue",
      description: "Songs to sort later",
      is_public: false,
    },
  ]);
  await expect(dialog).toBeHidden();
  await expect(createPlaylistButton).toBeFocused();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("playlists tab opens liked tracks subview with URL-backed pagination", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockMusicIndexApi(
    page,
    requestedMusicianRequests,
  );
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
