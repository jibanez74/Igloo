import { expect, test, type Page, type Route } from "@playwright/test";

type NullableString = {
  String: string;
  Valid: boolean;
};

type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

type CreatePlaylistRequest = {
  name: string;
  description?: string;
  is_public: boolean;
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

function apiResponse(data: unknown) {
  return {
    error: false,
    data,
  };
}

const mockAlbums = [
  {
    id: 1,
    title: "First Mock Album",
    cover: nullableString(),
    musician: nullableString("Aurora Pines"),
    year: nullableInt64(2026),
  },
];

const pageOneMusicians = [
  {
    id: 1,
    name: "Aurora Pines",
    sort_name: "Aurora Pines",
    thumb: nullableString(),
    album_count: 2,
    track_count: 18,
  },
  {
    id: 2,
    name: "Midnight Static",
    sort_name: "Midnight Static",
    thumb: nullableString(),
    album_count: 1,
    track_count: 9,
  },
];

const pageTwoMusicians = [
  {
    id: 3,
    name: "Northern Signal",
    sort_name: "Northern Signal",
    thumb: nullableString(),
    album_count: 3,
    track_count: 27,
  },
];

const mockTracks = [
  {
    id: 1,
    title: "Alabaster",
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: "/music/alabaster.flac",
    album_id: nullableInt64(10),
    album_title: nullableString("Blue Record"),
    album_cover: nullableString(),
    musician_id: nullableInt64(20),
    musician_name: nullableString("The Band"),
  },
  {
    id: 2,
    title: "Borrowed Light",
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: "/music/borrowed-light.flac",
    album_id: nullableInt64(10),
    album_title: nullableString("Blue Record"),
    album_cover: nullableString(),
    musician_id: nullableInt64(20),
    musician_name: nullableString("The Band"),
  },
];

const mockPlaylists = [
  {
    id: 30,
    user_id: 1,
    name: "Morning Rotation",
    description: nullableString("Daily tracks for the first pass"),
    cover_image: nullableString(),
    is_public: false,
    folder_id: nullableInt64(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    track_count: 3,
    total_duration: 540000,
    is_owner: true,
    can_edit: true,
  },
  {
    id: 31,
    user_id: 2,
    name: "Shared Discoveries",
    description: nullableString("Tracks shared by another listener"),
    cover_image: nullableString(),
    is_public: false,
    folder_id: nullableInt64(),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    track_count: 2,
    total_duration: 420000,
    is_owner: false,
    can_edit: false,
  },
];

async function fulfillJSON(route: Route, body: unknown) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function mockMusicIndexApi(
  page: Page,
  requestedMusicianRequests: string[],
  createdPlaylistRequests: CreatePlaylistRequest[] = [],
) {
  const playlists = [...mockPlaylists];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Music User",
          email: "music@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/music/stats") {
      await fulfillJSON(route, apiResponse({
        total_albums: 6,
        total_tracks: 54,
        total_musicians: 3,
      }));
      return;
    }

    if (url.pathname === "/api/music/albums") {
      const pageNumber = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");

      await fulfillJSON(route, apiResponse({
        albums: mockAlbums,
        total: mockAlbums.length,
        page: pageNumber,
        per_page: perPage,
        total_pages: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/musicians") {
      const musicianPage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "24");
      requestedMusicianRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        musicians: musicianPage === 2 ? pageTwoMusicians : pageOneMusicians,
        total: 3,
        page: musicianPage,
        per_page: perPage,
        total_pages: 2,
      }));
      return;
    }

    if (url.pathname === "/api/music/playlists") {
      if (method === "GET") {
        await fulfillJSON(route, apiResponse({ playlists }));
        return;
      }

      if (method === "POST") {
        const body = route.request().postDataJSON() as CreatePlaylistRequest;
        createdPlaylistRequests.push(body);

        const playlist = {
          id: 100 + playlists.length,
          user_id: 1,
          name: body.name,
          description: nullableString(body.description ?? ""),
          cover_image: nullableString(),
          is_public: body.is_public,
          folder_id: nullableInt64(),
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
          track_count: 0,
          total_duration: 0,
          is_owner: true,
          can_edit: true,
        };

        playlists.push(playlist);
        await fulfillJSON(route, apiResponse({ playlist }));
        return;
      }

      await fulfillJSON(route, {
        error: true,
        message: `Unexpected playlists method: ${method}`,
      });
      return;
    }

    if (url.pathname === "/api/music/tracks/liked-ids") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [2] }));
      return;
    }

    if (url.pathname === "/api/music/tracks") {
      await fulfillJSON(route, apiResponse({
        tracks: mockTracks,
        total: mockTracks.length,
        offset: Number(url.searchParams.get("offset") ?? "0"),
        limit: Number(url.searchParams.get("limit") ?? "50"),
        has_more: false,
      }));
      return;
    }

    await fulfillJSON(route, {
      error: true,
      message: `Unexpected API request: ${url.pathname}`,
    });
  });
}

test("musicians tab renders accessible count text and URL-backed pagination", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];

  await mockMusicIndexApi(page, requestedMusicianRequests);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/music?tab=musicians");

  const musiciansTab = page.getByRole("tab", { name: "Musicians" });
  await expect(musiciansTab).toHaveAttribute("aria-selected", "true");

  await expect(page.getByRole("link", { name: "Aurora Pines, 2 albums, 18 tracks" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Midnight Static, 1 albums, 9 tracks" })).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/musiciansPage=2/);
  await expect.poll(() => requestedMusicianRequests).toContainEqual("/api/music/musicians?page=2&per_page=24");
  await expect(page.getByRole("link", { name: "Northern Signal, 3 albums, 27 tracks" })).toBeVisible();
});

test("tracks tab exposes accessible controls and action menu targets", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];

  await mockMusicIndexApi(page, requestedMusicianRequests);
  await page.setViewportSize({ width: 1440, height: 900 });
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
});

test("playlists tab lists playlists and creates a playlist from the toolbar dialog", async ({ page }) => {
  const requestedMusicianRequests: string[] = [];
  const createdPlaylistRequests: CreatePlaylistRequest[] = [];

  await mockMusicIndexApi(page, requestedMusicianRequests, createdPlaylistRequests);
  await page.setViewportSize({ width: 1440, height: 900 });
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
});
