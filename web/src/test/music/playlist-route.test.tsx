import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PLAYLIST_TRACKS_KEY, PLAYLIST_TRACKS_PAGE_SIZE } from "@/lib/constants";
import { playlistDetailsQueryOpts } from "@/lib/query-opts";
import type { components } from "@/types/openapi.gen";
import { jsonResponse, requestURL } from "../helpers/api";
import { layOutTrackRows, pressKey } from "../helpers/drag";
import { authUser, nullableString, userSummary } from "../helpers/fixtures";
import { playlist, playlistDetail, playlistTrack } from "../helpers/music";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));
vi.mock("@/hooks/useAudioPlayerNowPlaying", () => import("../helpers/audio-player"));

const showActionFailedMock = vi.fn();
const showRemovedMock = vi.fn();

vi.mock("@/lib/toast-helpers", async () => ({
  ...(await vi.importActual<typeof import("@/lib/toast-helpers")>(
    "@/lib/toast-helpers",
  )),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  showRemoved: (...args: unknown[]) => showRemovedMock(...args),
}));

const TRACKS_PAGE_URL = (offset: number) =>
  `/api/music/playlists/21/tracks?limit=${PLAYLIST_TRACKS_PAGE_SIZE}&offset=${offset}`;

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
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(authUser());
    }

    if (url === "/api/music/playlists/21" && init?.method === "DELETE") {
      return jsonResponse({ error: false, message: "Playlist deleted successfully" });
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
    expect(within(stats).queryByText(/^By /)).not.toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Play all 2 tracks" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit playlist" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete playlist" })).toBeInTheDocument();

    expect(await screen.findByText("Alabaster")).toBeInTheDocument();
    expect(screen.getByText("Borrowed Light")).toBeInTheDocument();

    await waitFor(() => {
      expect(readDocumentHead().title).toBe("Road Trip - Igloo");
    });
  });

  it("names the owner instead of Edit and Delete for a viewer who does not own the playlist", async () => {
    mockPlaylistFetch(
      roadTrip({
        is_owner: false,
        can_edit: false,
        owner: userSummary({ id: 2, name: "Riley" }),
      }),
    );

    await renderRoute("/music/playlist/21");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Road Trip" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Alabaster")).toBeInTheDocument();
    const stats = screen.getByRole("list", { name: "Playlist statistics" });
    expect(within(stats).getByText("By Riley")).toBeInTheDocument();
    expect(screen.queryByText("Owner")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit playlist" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete playlist" })).not.toBeInTheDocument();
  });

  it("deletes the playlist after confirming and returns to the playlists tab", async () => {
    const fetchMock = mockPlaylistFetch(roadTrip());
    const user = userEvent.setup();

    const { router, queryClient } = await renderRoute("/music/playlist/21");

    await user.click(await screen.findByRole("button", { name: "Delete playlist" }));

    const confirm = await screen.findByRole("alertdialog", { name: "Delete playlist" });
    expect(confirm).toHaveTextContent("Are you sure you want to delete “Road Trip”?");
    await user.click(within(confirm).getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/music");
    });
    expect(router.state.location.search).toMatchObject({ tab: "playlists" });
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          requestURL(input as RequestInfo | URL) === "/api/music/playlists/21" &&
          (init as RequestInit | undefined)?.method === "DELETE",
      ),
    ).toBe(true);
    // Once its page is gone, nothing of the deleted playlist stays cached for
    // Back to render again.
    await waitFor(() => {
      expect(
        queryClient.getQueryData(playlistDetailsQueryOpts(21).queryKey),
      ).toBeUndefined();
    });
    expect(
      queryClient.getQueriesData({ queryKey: [PLAYLIST_TRACKS_KEY, 21] }),
    ).toEqual([]);
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

  it("offers Try again instead of the empty copy when the tracks fail to load", async () => {
    const user = userEvent.setup();
    const fetchMock = mockPlaylistFetch(roadTrip());
    const respond = fetchMock.getMockImplementation()!;
    // Failing until Try again: the loader's warm-up and the list's own
    // mount retry both see the failure.
    let failTracks = true;
    fetchMock.mockImplementation((input, init) => {
      if (failTracks && requestURL(input) === TRACKS_PAGE_URL(0)) {
        return jsonResponse({ error: true, message: "tracks query failed" }, 503);
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn’t load this playlist’s tracks.",
    );
    expect(
      screen.queryByText("No tracks in this playlist yet."),
    ).not.toBeInTheDocument();

    failTracks = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByText("Alabaster")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("stops loading the rest after a failed page and says how much was queued", async () => {
    const user = userEvent.setup();
    const mock = roadTrip({ track_count: 3 });
    const fetchMock = mockPlaylistFetch(mock);
    const respond = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((input, init) => {
      const url = requestURL(input);
      if (url === TRACKS_PAGE_URL(0)) {
        return jsonResponse({
          error: false,
          data: { tracks: mock.tracks, total: 3, has_more: true, next_offset: 2 },
        });
      }
      if (url === TRACKS_PAGE_URL(2)) {
        return jsonResponse({ error: true, message: "tracks query failed" }, 503);
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");
    expect(await screen.findByText("Alabaster")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Play all 3 tracks" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "play playlist",
        "Only 2 of 3 tracks could be loaded.",
      );
    });
    expect(screen.getByText("Alabaster")).toBeInTheDocument();
  });

  it("reports a refused track removal instead of confirming it", async () => {
    const user = userEvent.setup();
    const fetchMock = mockPlaylistFetch(roadTrip());
    const respond = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((input, init) => {
      if (
        requestURL(input) === "/api/music/playlists/21/tracks/1" &&
        init?.method === "DELETE"
      ) {
        return jsonResponse(
          { error: true, message: "You can't edit this playlist." },
          403,
        );
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");
    await user.click(
      await screen.findByRole("button", { name: "More actions for Alabaster" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /Remove from Playlist/ }),
    );

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "remove track",
        expect.objectContaining({ error: true, status: 403 }),
      );
    });
    expect(showRemovedMock).not.toHaveBeenCalled();
  });

  it("refreshes the header's details after removing a track", async () => {
    const user = userEvent.setup();
    const fetchMock = mockPlaylistFetch(roadTrip());
    const respond = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((input, init) => {
      if (
        requestURL(input) === "/api/music/playlists/21/tracks/1" &&
        init?.method === "DELETE"
      ) {
        return jsonResponse({ error: false, message: "Track removed" });
      }
      return respond(input, init);
    });
    const detailsRequests = () =>
      fetchMock.mock.calls.filter(
        ([input, init]) =>
          requestURL(input) === "/api/music/playlists/21" &&
          (init?.method ?? "GET") === "GET",
      ).length;

    await renderRoute("/music/playlist/21");
    await user.click(
      await screen.findByRole("button", { name: "More actions for Alabaster" }),
    );
    const before = detailsRequests();
    await user.click(
      await screen.findByRole("menuitem", { name: /Remove from Playlist/ }),
    );

    await waitFor(() => {
      expect(showRemovedMock).toHaveBeenCalledWith("Track", "from playlist");
    });
    await waitFor(() => {
      expect(detailsRequests()).toBeGreaterThan(before);
    });
  });

  it("falls back to the server's order when a reorder is refused", async () => {
    const fetchMock = mockPlaylistFetch(roadTrip());
    const respond = fetchMock.getMockImplementation()!;
    const reorderRequests: unknown[] = [];
    fetchMock.mockImplementation((input, init) => {
      if (requestURL(input) === "/api/music/playlists/21/tracks/reorder") {
        reorderRequests.push(JSON.parse(String(init?.body)));
        return jsonResponse(
          { error: true, message: "You can't edit this playlist." },
          403,
        );
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");
    await screen.findByText("Alabaster");
    layOutTrackRows(["Alabaster", "Borrowed Light"]);
    const titlesInOrder = () =>
      screen
        .getAllByText(/^(Alabaster|Borrowed Light)$/)
        .map(element => element.textContent);

    const handle = (
      await screen.findAllByRole("button", { name: "Drag to reorder" })
    )[1];
    handle.focus();
    pressKey(handle, "Space");
    await screen.findByText(/^Picked up Borrowed Light/);
    pressKey(handle, "ArrowUp");
    await screen.findByText("Borrowed Light is over Alabaster");
    pressKey(handle, "Space");

    await waitFor(() => {
      expect(reorderRequests).toEqual([{ track_ids: [2, 1] }]);
    });
    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "reorder tracks",
        expect.objectContaining({ status: 403 }),
      );
    });
    await waitFor(() => {
      expect(titlesInOrder()).toEqual(["Alabaster", "Borrowed Light"]);
    });
  });

  it("keeps a saved reorder once the server's list comes back", async () => {
    const mock = roadTrip();
    const fetchMock = mockPlaylistFetch(mock);
    const respond = fetchMock.getMockImplementation()!;
    let saved = false;
    fetchMock.mockImplementation((input, init) => {
      const url = requestURL(input);
      if (url === "/api/music/playlists/21/tracks/reorder") {
        saved = true;
        return jsonResponse({ error: false, message: "Tracks reordered" });
      }
      if (saved && url === TRACKS_PAGE_URL(0)) {
        return jsonResponse({
          error: false,
          data: {
            tracks: [...mock.tracks].reverse(),
            total: 2,
            has_more: false,
            next_offset: 2,
          },
        });
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");
    await screen.findByText("Alabaster");
    layOutTrackRows(["Alabaster", "Borrowed Light"]);
    const titlesInOrder = () =>
      screen
        .getAllByText(/^(Alabaster|Borrowed Light)$/)
        .map(element => element.textContent);

    const handle = (
      await screen.findAllByRole("button", { name: "Drag to reorder" })
    )[1];
    handle.focus();
    pressKey(handle, "Space");
    await screen.findByText(/^Picked up Borrowed Light/);
    pressKey(handle, "ArrowUp");
    await screen.findByText("Borrowed Light is over Alabaster");
    pressKey(handle, "Space");

    await waitFor(() => expect(saved).toBe(true));
    await waitFor(() => {
      expect(titlesInOrder()).toEqual(["Borrowed Light", "Alabaster"]);
    });
    expect(showActionFailedMock).not.toHaveBeenCalled();
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

    expect(
      await screen.findByRole("heading", { level: 1, name: "No access" }),
    ).toBeInTheDocument();
    const alert = screen.getByRole("alert");
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
