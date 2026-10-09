import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { components } from "@/types/openapi.gen";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser, nullableString } from "../helpers/fixtures";
import { playlist, playlistDetail, playlistTrack } from "../helpers/music";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => ({
    playQueue: vi.fn(),
    playTrack: vi.fn(),
    shuffleQueue: vi.fn(),
    togglePlay: vi.fn(),
  }),
}));

vi.mock("@/hooks/useAudioPlayerNowPlaying", () => ({
  useAudioPlayerNowPlaying: () => ({
    currentTrackId: null,
    isPlaying: false,
  }),
}));

type Schema = components["schemas"];

type MockPlaylist = {
  detail: Schema["MusicPlaylistDetailData"];
  tracks: Schema["PlaylistTrack"][];
};

function roadTrip(
  fields: Partial<Schema["MusicPlaylistDetailData"]> = {},
  tracks = [
    playlistTrack({ id: 1, title: "Alabaster", position: 1, duration: 180 }),
    playlistTrack({ id: 2, title: "Borrowed Light", position: 2, duration: 240 }),
  ],
): MockPlaylist {
  return {
    detail: playlistDetail({
      playlist: playlist({
        id: 21,
        name: "Road Trip",
        description: nullableString("Songs for the long way round."),
      }),
      track_count: tracks.length,
      duration: tracks.reduce((sum, track) => sum + track.duration, 0),
      ...fields,
    }),
    tracks,
  };
}

function mockPlaylistFetch(mock: MockPlaylist) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(authUser());
    }

    if (url === "/api/music/playlists/21") {
      return jsonResponse({ error: false, data: mock.detail });
    }

    if (url.startsWith("/api/music/playlists/21/tracks?")) {
      return jsonResponse({
        error: false,
        data: {
          tracks: mock.tracks,
          total: mock.tracks.length,
          has_more: false,
          next_offset: mock.tracks.length,
        },
      });
    }

    if (url === "/api/music/tracks/liked-ids") {
      return jsonResponse({ error: false, data: { liked_track_ids: [] } });
    }

    return jsonResponse({ error: false, data: {} });
  });

  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
}

describe("music playlist route", () => {
  it("renders the header, the owner actions and the tracks", async () => {
    mockPlaylistFetch(roadTrip());

    await renderRoute("/music/playlist/21");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Road Trip" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Songs for the long way round.")).toBeInTheDocument();

    const stats = screen.getByRole("list", { name: "Playlist statistics" });
    expect(within(stats).getByText("2 tracks")).toBeInTheDocument();
    expect(within(stats).getByText("Owner")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Play all 2 tracks" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit playlist" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete playlist" })).toBeInTheDocument();

    expect(await screen.findByText("Alabaster")).toBeInTheDocument();
    expect(screen.getByText("Borrowed Light")).toBeInTheDocument();

    await waitFor(() => {
      expect(readDocumentHead().title).toBe("Road Trip - Igloo");
    });
  });

  it("hides Edit and Delete from a viewer who does not own the playlist", async () => {
    mockPlaylistFetch(roadTrip({ is_owner: false, can_edit: false }));

    await renderRoute("/music/playlist/21");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Road Trip" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Alabaster")).toBeInTheDocument();
    expect(screen.queryByText("Owner")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit playlist" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete playlist" })).not.toBeInTheDocument();
  });

  it("shows the empty copy and no Play pair for a playlist without tracks", async () => {
    mockPlaylistFetch(roadTrip({}, []));

    await renderRoute("/music/playlist/21");

    expect(
      await screen.findByText("No tracks in this playlist yet."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 tracks")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Play all/ })).not.toBeInTheDocument();
  });

  it("rejects a malformed playlist id without asking the API", async () => {
    const fetchMock = mockPlaylistFetch(roadTrip());

    await renderRoute("/music/playlist/abc");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("That playlist link is not valid.");
    expect(
      within(alert.parentElement as HTMLElement)
        .getByRole("link", { name: "Back to Playlists" })
        .getAttribute("href"),
    ).toContain("tab=playlists");
    expect(
      fetchMock.mock.calls.some(([input]) =>
        requestURL(input as RequestInfo | URL).startsWith("/api/music/playlists/"),
      ),
    ).toBe(false);
  });

  // A private playlist the viewer was not shared answers 403 "access denied";
  // the reader gets a sentence and a way back, never the server's constant.
  it("words a private playlist as no access, with a way back", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = requestURL(input);
        if (url === "/api/auth/user") {
          return jsonResponse(authUser());
        }
        if (url === "/api/music/playlists/21") {
          return jsonResponse({ error: true, message: "access denied" }, 403);
        }
        return jsonResponse({ error: false, data: {} });
      }),
    );

    await renderRoute("/music/playlist/21");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("No access");
    expect(alert).toHaveTextContent("You don't have access to this playlist.");
    expect(screen.queryByText(/access denied/)).not.toBeInTheDocument();
  });

  it("says a missing playlist was not found", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = requestURL(input);
        if (url === "/api/auth/user") {
          return jsonResponse(authUser());
        }
        if (url === "/api/music/playlists/21") {
          return jsonResponse({ error: true, message: "playlist not found" }, 404);
        }
        return jsonResponse({ error: false, data: {} });
      }),
    );

    await renderRoute("/music/playlist/21");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "We couldn't find that playlist.",
    );
  });
});
