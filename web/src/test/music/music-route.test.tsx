import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ALBUMS_PER_PAGE,
  CONTENT_FADE_TRANSITION_MS,
  LIKED_TRACKS_PER_PAGE,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MUSICIANS_PER_PAGE,
  TRACKS_INFINITE_PAGE_SIZE,
} from "@/lib/constants";
import type { PlaylistSummaryType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { runContentFadeTransitionTimeout } from "../helpers/content-fade-transition";
import { restoreMatchMedia, setReducedMotionPreference } from "../helpers/dom";
import { authUser, nullableInt64, nullableString } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const { audioPlayerActionsMock } = vi.hoisted(() => ({
  audioPlayerActionsMock: {
    playQueue: vi.fn(),
    playTrack: vi.fn(),
    startPlayAllPlayback: vi.fn(),
    startShufflePlayback: vi.fn(),
  },
}));

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => audioPlayerActionsMock,
}));

vi.mock("@/hooks/useAudioPlayerNowPlaying", () => ({
  useAudioPlayerNowPlaying: () => ({
    currentTrackId: null,
    isPlaying: false,
  }),
}));

function track(id: number, title: string) {
  return {
    id,
    title,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    album_id: nullableInt64(10),
    album_title: nullableString("Blue Record"),
    album_cover: nullableString(),
    musician_id: nullableInt64(20),
    musician_name: nullableString("The Band"),
  };
}

function playlist(
  id: number,
  name: string,
  fields: Partial<PlaylistSummaryType> = {},
): PlaylistSummaryType {
  return {
    id,
    user_id: 1,
    name,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    track_count: 0,
    total_duration: 0,
    is_owner: true,
    can_edit: true,
    ...fields,
  };
}

type MockMusicFetchOptions = {
  spotifyAvailable?: boolean;
  emptyMusicians?: boolean;
  emptyTracks?: boolean;
  emptyPlaylists?: boolean;
  emptyLikedTracks?: boolean;
  failFirstTracksRequest?: boolean;
  failFirstPlaylistsRequest?: boolean;
  failFirstLikedTracksRequest?: boolean;
};

function mockMusicFetch(options: MockMusicFetchOptions = {}) {
  const spotifyAvailable = options.spotifyAvailable ?? true;
  const emptyMusicians = options.emptyMusicians ?? false;
  let tracksRequestCount = 0;
  let playlistsRequestCount = 0;
  let likedTracksRequestCount = 0;
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(
        authUser({ name: "Music User", email: "music@example.com" }),
      );
    }

    if (url === "/api/spotify/status") {
      return jsonResponse({
        error: false,
        data: {
          available: spotifyAvailable,
        },
      });
    }

    if (url === "/api/music/stats") {
      return jsonResponse({
        error: false,
        data: {
          total_albums: 1,
          total_tracks: 5,
          total_musicians: 1,
        },
      });
    }

    if (url === `/api/music/albums?page=1&per_page=${ALBUMS_PER_PAGE}`) {
      return jsonResponse({
        error: false,
        data: {
          albums: [
            {
              id: 1,
              title: "Blue Record",
              cover: nullableString(),
              musician: nullableString("The Band"),
              year: nullableInt64(2026),
            },
          ],
          total: 1,
          page: 1,
          per_page: ALBUMS_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === `/api/music/musicians?page=1&per_page=${MUSICIANS_PER_PAGE}`) {
      if (emptyMusicians) {
        return jsonResponse({
          error: false,
          data: {
            musicians: [],
            total: 0,
            page: 1,
            per_page: MUSICIANS_PER_PAGE,
            total_pages: 0,
          },
        });
      }

      return jsonResponse({
        error: false,
        data: {
          musicians: [
            {
              id: 2,
              name: "Nina Simone",
              sort_name: "Simone, Nina",
              thumb: nullableString(),
              album_count: 1,
              track_count: 5,
            },
          ],
          total: 1,
          page: 1,
          per_page: MUSICIANS_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === `/api/music/tracks?limit=${TRACKS_INFINITE_PAGE_SIZE}&offset=0`) {
      tracksRequestCount += 1;

      if (options.failFirstTracksRequest && tracksRequestCount === 1) {
        return jsonResponse({
          error: true,
          message: "The tracks library is temporarily unavailable.",
        });
      }

      const tracks = options.emptyTracks
        ? []
        : [track(1, "Alabaster"), track(2, "Borrowed Light")];
      return jsonResponse({
        error: false,
        data: {
          tracks,
          total: tracks.length,
          offset: 0,
          limit: TRACKS_INFINITE_PAGE_SIZE,
          has_more: false,
        },
      });
    }

    if (
      url === `/api/music/tracks/liked?page=1&per_page=${LIKED_TRACKS_PER_PAGE}`
    ) {
      likedTracksRequestCount += 1;

      if (options.failFirstLikedTracksRequest && likedTracksRequestCount === 1) {
        return jsonResponse({
          error: true,
          message: "Liked tracks are temporarily unavailable.",
        });
      }

      const tracks = options.emptyLikedTracks ? [] : [track(2, "Borrowed Light")];
      return jsonResponse({
        error: false,
        data: {
          tracks,
          total: tracks.length,
          page: 1,
          per_page: LIKED_TRACKS_PER_PAGE,
          total_pages: tracks.length === 0 ? 0 : 1,
          has_more: false,
        },
      });
    }

    if (url === "/api/music/tracks/liked-ids") {
      return jsonResponse({
        error: false,
        data: {
          liked_track_ids: [2],
        },
      });
    }

    if (url === "/api/music/playlists") {
      playlistsRequestCount += 1;

      if (options.failFirstPlaylistsRequest && playlistsRequestCount === 1) {
        return jsonResponse({
          error: true,
          message: "Playlists are temporarily unavailable.",
        });
      }

      if (options.emptyPlaylists) {
        return jsonResponse({ error: false, data: { playlists: [] } });
      }

      return jsonResponse({
        error: false,
        data: {
          playlists: [
            playlist(30, "Morning Rotation", { track_count: 3, total_duration: 540_000 }),
            playlist(31, "Shared Discoveries", {
              user_id: 2,
              track_count: 2,
              total_duration: 420_000,
              is_owner: false,
              can_edit: false,
            }),
          ],
        },
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
}

async function renderMusicRoute(
  initialEntry: string,
  options: MockMusicFetchOptions = {},
) {
  mockMusicFetch(options);

  return renderRoute(initialEntry);
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  restoreMatchMedia();
});

describe("music route tab transitions", () => {
  it("delays swapping from albums to musicians until the fade-out completes", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMusicRoute("/music/");

    expect(screen.getByText("Blue Record")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Musicians" }));

    expect(screen.getByText("Blue Record")).toBeInTheDocument();
    expect(screen.queryByText("Nina Simone")).not.toBeInTheDocument();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByText("Nina Simone")).toBeInTheDocument();
    });
  }, 10_000);

  it("switches tabs without waiting when reduced motion is enabled", async () => {
    setReducedMotionPreference(true);
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");

    await renderMusicRoute("/music/");

    await user.click(screen.getByRole("tab", { name: "Musicians" }));

    await waitFor(() => {
      expect(screen.getByText("Nina Simone")).toBeInTheDocument();
    });
    expect(
      setTimeoutSpy.mock.calls.some(
        ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
      ),
    ).toBe(false);
  });
});

describe("musicians tab empty state", () => {
  it("renders and announces the empty musicians state", async () => {
    setReducedMotionPreference(true);

    await renderMusicRoute("/music/?tab=musicians", { emptyMusicians: true });

    expect(
      await screen.findByText("No musicians found in your library."),
    ).toBeInTheDocument();

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(
          region => region.textContent === "No musicians found",
        ),
      ).toBe(true);
    });
  });
});

describe("music route section motion", () => {
  it("applies section entrance contracts without changing tab panel fade behavior", async () => {
    await renderMusicRoute("/music/");

    const heading = await screen.findByRole("heading", {
      name: "Music Library",
    });
    const stats = screen.getByRole("region", {
      name: "Library statistics: 1 album, 5 tracks, 1 musician",
    });
    const tabsRoot = screen.getByRole("tablist").closest('[data-slot="tabs"]');

    expect(heading.closest("header")?.className).toContain(
      MOTION_SECTION_ENTER_CLASS,
    );
    expect(stats.parentElement?.className).toContain(
      MOTION_SECTION_ENTER_DELAYED_CLASS,
    );
    expect(tabsRoot?.className).toContain(MOTION_SECTION_ENTER_DELAYED_CLASS);
    expect(
      screen.getByRole("tabpanel", { name: "Albums" }).firstElementChild
        ?.className,
    ).toContain(MOTION_SECTION_ENTER_CLASS);
  });
});

describe("music route more menu", () => {
  it("opens the Request Album dialog from More options", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/");

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Request Album" }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Request Album" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("Album title")).toHaveFocus();
    });
  });

  it("disables Request Album when Spotify search is unavailable", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/", { spotifyAvailable: false });

    await user.click(screen.getByRole("button", { name: "More options" }));

    const requestAlbumItem = await screen.findByRole("menuitem", {
      name: /Request Album unavailable/i,
    });

    expect(requestAlbumItem).toHaveAttribute("data-disabled");
    expect(requestAlbumItem).toHaveAttribute(
      "title",
      "Spotify search is unavailable on this server.",
    );
  });
});

describe("music route tracks tab", () => {
  it("shows an API failure and loads tracks after retrying", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/?tab=tracks", {
      failFirstTracksRequest: true,
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The tracks library is temporarily unavailable.",
    );
    expect(
      screen.queryByText("No tracks found in your library."),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByRole("list", { name: "Tracks" })).toBeInTheDocument();
    expect(screen.getByText("Alabaster")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("labels the virtualized track list and track action menus", async () => {
    await renderMusicRoute("/music/?tab=tracks");

    const tracksList = await screen.findByRole("list", { name: "Tracks" });

    expect(tracksList).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Tracks starting with A" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Tracks starting with B" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "More actions for Alabaster" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "More actions for Borrowed Light" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Play all tracks" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Shuffle all tracks" })).toBeEnabled();

    // The row like buttons are named from /api/music/tracks/liked-ids (track 2).
    expect(
      await screen.findByRole("button", { name: "Remove Borrowed Light from liked" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add Alabaster to liked" }),
    ).toBeInTheDocument();

    const trackRows = within(tracksList).getAllByRole("listitem");

    expect(trackRows).toHaveLength(2);
    expect(trackRows[0]).toHaveAttribute("aria-posinset", "1");
    expect(trackRows[0]).toHaveAttribute("aria-setsize", "2");
    expect(trackRows[1]).toHaveAttribute("aria-posinset", "2");
    expect(trackRows[1]).toHaveAttribute("aria-setsize", "2");
  });
});

describe("music route playlists tab", () => {
  it("lists playlists with the owner badge and the toolbar actions", async () => {
    await renderMusicRoute("/music/?tab=playlists");

    const owned = await screen.findByRole("link", {
      name: "Morning Rotation, 3 tracks, 9m 0s",
    });
    const shared = screen.getByRole("link", {
      name: "Shared Discoveries, 2 tracks, 7m 0s",
    });
    expect(screen.getByText("2 playlists")).toBeInTheDocument();

    // Only the caller's own playlists carry the Owner badge.
    expect(within(owned.closest("article")!).getByText("Owner")).toBeInTheDocument();
    expect(within(shared.closest("article")!).queryByText("Owner")).not.toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "View liked tracks" }),
    ).toHaveTextContent("Liked tracks");
    expect(
      screen.getByRole("button", { name: "Create new playlist" }),
    ).toHaveTextContent("New playlist");
  });

  it("renders the minimal empty state with the toolbar as the only call to action", async () => {
    await renderMusicRoute("/music/?tab=playlists", { emptyPlaylists: true });

    expect(
      await screen.findByText("No playlists yet. Use New playlist to group tracks."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 playlists")).toBeInTheDocument();

    // One "New playlist" action: the toolbar button, not a second CTA in the
    // empty state (design-system §3.4).
    expect(screen.getAllByRole("button", { name: /playlist/i })).toHaveLength(1);
    expect(
      screen.queryByRole("button", { name: /create your first playlist/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "No playlists yet" }),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(region => region.textContent === "No playlists yet"),
      ).toBe(true);
    });
  });

  it("renders the minimal empty state for liked tracks", async () => {
    await renderMusicRoute("/music/?tab=playlists&playlistsView=liked", {
      emptyLikedTracks: true,
    });

    expect(
      await screen.findByText(
        "No liked tracks yet. Tap the heart on any track to add it here.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "No liked tracks yet" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back to playlists" })).toBeInTheDocument();
  });

  it("shows a playlists failure instead of the empty state and recovers on retry", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/?tab=playlists", {
      failFirstPlaylistsRequest: true,
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Playlists are temporarily unavailable.",
    );
    expect(
      screen.queryByText("No playlists yet. Use New playlist to group tracks."),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(
      await screen.findByRole("link", { name: "Morning Rotation, 3 tracks, 9m 0s" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows a liked tracks failure under the header and recovers on retry", async () => {
    const user = userEvent.setup();

    await renderMusicRoute("/music/?tab=playlists&playlistsView=liked", {
      failFirstLikedTracksRequest: true,
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Liked tracks are temporarily unavailable.",
    );
    expect(
      screen.queryByText(
        "No liked tracks yet. Tap the heart on any track to add it here.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("0 tracks")).not.toBeInTheDocument();
    // The header stays, so the user can still leave the failed view.
    expect(screen.getByRole("button", { name: "Back to playlists" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByText("Borrowed Light")).toBeInTheDocument();
    expect(screen.getByText("1 track")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("music route tracks tab empty state", () => {
  it("renders and announces the empty tracks state", async () => {
    await renderMusicRoute("/music/?tab=tracks", { emptyTracks: true });

    expect(
      await screen.findByText("No tracks found in your library."),
    ).toBeInTheDocument();

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(region => region.textContent === "No tracks found"),
      ).toBe(true);
    });
  });
});
