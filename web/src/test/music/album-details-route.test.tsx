import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DETAIL_PAGE_CONTENT_ENTER_CLASS } from "@/lib/constants";
import type { AlbumDetailsResponseType, TrackType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { nullableFloat64, nullableInt64, nullableString } from "../helpers/fixtures";
import { readDocumentHead, renderRoute } from "../helpers/render-route";
import {
  getDetailMotionWrappers,
  getHeroMotionWrapper,
  getLowerMotionWrapper,
} from "../helpers/motion";

vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));
vi.mock("@/hooks/useAudioPlayerNowPlaying", () => import("../helpers/audio-player"));

const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
  showDeleted: vi.fn(),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
  showDeleted: toastMocks.showDeleted,
}));

function albumTrack(
  id: number,
  albumId: number,
  title: string,
  trackIndex: number,
): TrackType {
  return {
    id,
    title,
    codec: "flac",
    mime_type: "audio/flac",
    track_index: trackIndex,
    duration: 180_000,
    disc: 1,
    channel_layout: "stereo",
    bit_rate: 900000,
    album_id: nullableInt64(albumId),
    musician_id: nullableInt64(albumId + 100),
  };
}

function albumDetailsResponse(
  id: number,
  title: string,
  artistName: string,
  trackTitle: string,
): AlbumDetailsResponseType {
  return {
    album: {
      id,
      title,
      sort_title: title,
      musician: nullableString(artistName),
      spotify_id: nullableString(`spotify-${id}`),
      spotify_popularity: nullableFloat64(76),
      release_date: nullableString("2026-01-01"),
      year: nullableInt64(2026),
      total_tracks: nullableInt64(1),
      cover: nullableString(`/covers/${id}.jpg`),
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    tracks: [albumTrack(id * 10, id, trackTitle, 1)],
    artists: [
      {
        id: id + 100,
        name: artistName,
        thumb: nullableString(""),
      },
    ],
    track_genres: [
      {
        track_id: id * 10,
        tag: "Alternative",
      },
    ],
    album_genres: ["Alternative"],
    total_duration: 180_000,
  };
}

type MockAlbumDetailsFetchOptions = {
  deleteAlbumResponse?: {
    body: unknown;
    status?: number;
  };
  isAdmin?: boolean;
  additionalAlbums?: Array<[number, AlbumDetailsResponseType]>;
};

function mockAlbumDetailsFetch({
  deleteAlbumResponse,
  isAdmin = false,
  additionalAlbums = [],
}: MockAlbumDetailsFetchOptions = {}) {
  const detailsById = new Map<number, AlbumDetailsResponseType>([
    [42, albumDetailsResponse(42, "Blue Record", "The Band", "Alabaster")],
    [43, albumDetailsResponse(43, "Red Record", "The Trio", "Ember")],
    ...additionalAlbums,
  ]);

  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Album User",
            email: "albums@example.com",
            is_admin: isAdmin,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    const detailsMatch = url.match(/^\/api\/music\/albums\/details\/(\d+)$/);
    if (detailsMatch) {
      const albumId = Number.parseInt(detailsMatch[1], 10);
      const payload = detailsById.get(albumId);
      if (payload) {
        return jsonResponse({
          error: false,
          data: payload,
        });
      }
    }

    if (url === "/api/music/tracks/liked-ids") {
      return jsonResponse({
        error: false,
        data: {
          liked_track_ids: [420],
        },
      });
    }

    if (url === "/api/music/albums/42" && method === "DELETE") {
      return jsonResponse(
        deleteAlbumResponse?.body ?? {
          error: false,
        },
        deleteAlbumResponse?.status ?? 200,
      );
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
  return fetchMock;
}

async function renderAlbumDetailsRoute(
  initialEntry: string,
  options?: MockAlbumDetailsFetchOptions,
) {
  mockAlbumDetailsFetch(options);

  return renderRoute(initialEntry);
}

describe("album details route motion", () => {
  it("renders the album detail page with the three-stage stagger contract", async () => {
    const { container } = await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    const wrappers = getDetailMotionWrappers(container);
    const heroWrapper = getHeroMotionWrapper(container);
    const lowerWrapper = getLowerMotionWrapper(container);
    const backdropWrapper = wrappers.find(
      (element) =>
        element !== heroWrapper &&
        element !== lowerWrapper &&
        element.className === DETAIL_PAGE_CONTENT_ENTER_CLASS,
    );

    expect(wrappers).toHaveLength(3);
    expect(backdropWrapper).toBeDefined();
    expect(heroWrapper?.className).toContain("delay-75 motion-reduce:delay-0");
    expect(lowerWrapper?.className).toContain(
      "delay-150 motion-reduce:delay-0",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:translate-y-0",
    );
  });

  it("replays the detail-page stagger when navigating between album ids on the same route", async () => {
    const { container, router } = await renderAlbumDetailsRoute(
      "/music/album/42",
    );

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("Alabaster")).toBeInTheDocument();

    const firstHeroWrapper = getHeroMotionWrapper(container);
    expect(firstHeroWrapper).toBeDefined();

    await act(async () => {
      await router.navigate({
        to: "/music/album/$id",
        params: { id: "43" },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Red Record/i }),
      ).toBeInTheDocument();
    });

    const secondHeroWrapper = getHeroMotionWrapper(container);

    expect(secondHeroWrapper).toBeDefined();
    expect(secondHeroWrapper).not.toBe(firstHeroWrapper);
    expect(screen.getByText("Ember")).toBeInTheDocument();
    expect(screen.queryByText("Alabaster")).not.toBeInTheDocument();
  });
});

describe("album details guards", () => {
  it("shows the not-found card with a way back for a malformed album id", async () => {
    await renderAlbumDetailsRoute("/music/album/abc");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("That album link is not valid.");
    expect(screen.getByRole("link", { name: "Back to Music" })).toHaveAttribute(
      "href",
      "/music",
    );
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});

describe("album details content", () => {
  it("links the hero artist name, artist badge, and details entry to the musician page", async () => {
    await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    const artistLinks = screen.getAllByRole("link", { name: "The Band" });
    // Hero name, artist badge pill, and the Album Details entry
    expect(artistLinks).toHaveLength(3);
    artistLinks.forEach((link) => {
      expect(link).toHaveAttribute("href", "/music/musician/142");
    });
  });

  it("titles the page with the album and its artist", async () => {
    await renderAlbumDetailsRoute("/music/album/42");

    await waitFor(() => {
      expect(readDocumentHead()).toMatchObject({
        title: "Blue Record by The Band - Igloo",
        description:
          "Listen to Blue Record by The Band - 1 track in your Igloo music library.",
        descriptionCount: 1,
      });
    });
  });

  it("renders disc headers, Spotify popularity and the liked state from liked-ids", async () => {
    const twoDiscs = albumDetailsResponse(42, "Blue Record", "The Band", "Alabaster");
    twoDiscs.tracks.push({
      ...albumTrack(421, 42, "Second Disc Opener", 1),
      disc: 2,
    });

    await renderAlbumDetailsRoute("/music/album/42", {
      additionalAlbums: [[42, twoDiscs]],
    });

    expect(
      await screen.findByRole("heading", { level: 1, name: "Blue Record" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Track List" })).toBeInTheDocument();
    expect(screen.getByText("Disc 1")).toBeInTheDocument();
    expect(screen.getByText("Disc 2")).toBeInTheDocument();
    expect(
      screen.getByRole("group", { name: "Spotify popularity 76 out of 100" }),
    ).toBeInTheDocument();

    // Track 420 is liked in the mock; the other row offers to like.
    expect(
      await screen.findByRole("button", { name: "Remove Alabaster from liked" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add Second Disc Opener to liked" }),
    ).toBeInTheDocument();
  });

  it("hides the delete menu from non-admins", async () => {
    await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Play Album" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "More options" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the hero artist name as plain text when it matches no album artist", async () => {
    const compilation = albumDetailsResponse(
      44,
      "Mixed Signals",
      "The Band",
      "Opening Track",
    );
    compilation.album.musician = nullableString("Various Artists");

    await renderAlbumDetailsRoute("/music/album/44", {
      additionalAlbums: [[44, compilation]],
    });

    expect(
      await screen.findByRole("heading", { name: /Mixed Signals/i }),
    ).toBeInTheDocument();

    expect(screen.getAllByText("Various Artists").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("link", { name: "Various Artists" }),
    ).not.toBeInTheDocument();
    // The badge for the actual artist still links
    expect(screen.getByRole("link", { name: "The Band" })).toHaveAttribute(
      "href",
      "/music/musician/144",
    );
  });

  it("shows an empty state and hides playback buttons for an album with no tracks", async () => {
    const emptyAlbum = albumDetailsResponse(
      45,
      "Silent Record",
      "The Band",
      "Unused",
    );
    emptyAlbum.tracks = [];
    emptyAlbum.track_genres = [];
    emptyAlbum.total_duration = 0;

    await renderAlbumDetailsRoute("/music/album/45", {
      isAdmin: true,
      additionalAlbums: [[45, emptyAlbum]],
    });

    expect(
      await screen.findByRole("heading", { name: /Silent Record/i }),
    ).toBeInTheDocument();

    expect(screen.getByText("No tracks in this album")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Play Album" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Shuffle play album" }),
    ).not.toBeInTheDocument();
    // Admins keep the delete path for broken/empty albums
    expect(
      screen.getByRole("button", { name: "More options" }),
    ).toBeInTheDocument();
  });

  it("keeps the a11y contract: labeled pill lists, skip links, and page summary", async () => {
    await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("list", { name: "Album details" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: "Genres: Alternative" }),
    ).toBeInTheDocument();

    const durationTime = screen.getByText("3m 0s", { selector: "time" });
    expect(durationTime).toHaveAttribute("dateTime", "PT180S");
    // aria-label does not reliably name a <time>, so the spoken words live in
    // an sr-only span and the formatted value is hidden (§1.7).
    expect(durationTime).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByText("Total duration 3m 0s")).toHaveClass("sr-only");

    const skipNav = screen.getByRole("navigation", {
      name: "Skip to section",
    });
    expect(skipNav).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Skip to track list" }),
    ).toHaveAttribute("href", "#tracklist-heading");

    const article = screen.getByRole("article");
    expect(article).toHaveAttribute("aria-describedby", "album-42-summary");
    const summary = document.getElementById("album-42-summary");
    expect(summary?.textContent).toContain("Blue Record by The Band");
    expect(summary?.textContent).toContain("1 track");
  });

  it("renders the enriched album details section", async () => {
    await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    const details = within(
      screen.getByRole("region", { name: "Album Details" }),
    );
    expect(details.getByText("Audio Quality")).toBeInTheDocument();
    expect(details.getByText("FLAC · 900 kbps · stereo")).toBeInTheDocument();
    expect(details.getByText("Genres")).toBeInTheDocument();
    expect(details.getByText("Alternative")).toBeInTheDocument();
    expect(details.getByText("Artist")).toBeInTheDocument();
    expect(details.getByRole("link", { name: "The Band" })).toHaveAttribute(
      "href",
      "/music/musician/142",
    );
    // Single disc album: no Discs entry
    expect(details.queryByText("Discs")).not.toBeInTheDocument();
  });
});

describe("album details deletion", () => {
  it("keeps the delete dialog open and reenabled after an API error response", async () => {
    const user = userEvent.setup();
    const { router } = await renderAlbumDetailsRoute("/music/album/42", {
      isAdmin: true,
      deleteAlbumResponse: {
        body: {
          error: true,
          message: "Unable to delete album",
        },
      },
    });

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(screen.getByRole("menuitem", { name: "Delete Album" }));

    const dialog = await screen.findByRole("alertdialog", {
      name: "Delete Album",
    });
    const confirmButton = screen.getByRole("button", {
      name: "Delete Album",
    });
    const cancelButton = screen.getByRole("button", { name: "Cancel" });

    await user.click(confirmButton);

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "delete album",
        "Unable to delete album",
      );
    });

    expect(router.state.location.pathname).toBe("/music/album/42");
    expect(dialog).toBeInTheDocument();
    expect(confirmButton).toBeEnabled();
    expect(cancelButton).toBeEnabled();
  });
});

// A track can join a playlist from wherever it is listed, and the row's links
// only lead away from the page: an album row offers the artist, never the
// album it already sits on.
describe("album details track actions", () => {
  it("offers Add to Playlist and Go to Artist on each track row, but not Go to Album", async () => {
    const user = userEvent.setup();
    await renderAlbumDetailsRoute("/music/album/42");

    expect(
      await screen.findByRole("heading", { name: /Blue Record/i }),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "More actions for Alabaster" }),
    );

    expect(
      await screen.findByRole("menuitem", { name: "Add to Playlist" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Go to Artist" })).toHaveAttribute(
      "href",
      "/music/musician/142",
    );
    expect(
      screen.queryByRole("menuitem", { name: "Go to Album" }),
    ).not.toBeInTheDocument();
  });
});
