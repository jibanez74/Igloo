import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PLAYLIST_TRACKS_KEY, PLAYLIST_TRACKS_PAGE_SIZE } from "@/lib/constants";
import { playlistDetailsQueryOpts } from "@/lib/query-opts";
import type { components } from "@/types/openapi.gen";
import { jsonResponse, requestURL } from "../helpers/api";
import { audioPlayerActionsMock } from "../helpers/audio-player";
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

  // A viewer's playlist renders through the virtual list, which mounts only a
  // window of rows; each still says where it sits in the whole playlist.
  it("gives the virtual list's rows their place in the whole playlist", async () => {
    const tracks = [
      playlistTrack({ id: 1, title: "Alabaster", position: 1, duration: 180 }),
      playlistTrack({ id: 2, title: "Borrowed Light", position: 2, duration: 240 }),
      playlistTrack({ id: 3, title: "Cinder", position: 3, duration: 200 }),
    ];
    const mock = roadTrip({ track_count: 40, is_owner: false, can_edit: false }, tracks);
    const fetchMock = mockPlaylistFetch(mock);
    const respond = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation((input, init) => {
      if (requestURL(input) === TRACKS_PAGE_URL(0)) {
        return jsonResponse({
          error: false,
          data: { tracks, total: 40, has_more: true, next_offset: 3 },
        });
      }
      return respond(input, init);
    });

    await renderRoute("/music/playlist/21");

    const list = await screen.findByRole("list", { name: "Playlist tracks" });
    const rows = within(list).getAllByRole("listitem");
    expect(rows).toHaveLength(3);
    expect(rows.map((row) => row.getAttribute("aria-posinset"))).toEqual(["1", "2", "3"]);
    for (const row of rows) {
      expect(row).toHaveAttribute("aria-setsize", "40");
    }
    expect(within(rows[0]).getByText("Alabaster")).toBeInTheDocument();
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

  describe("loading the rest of a playing playlist", () => {
    const pages = [
      [
        playlistTrack({ id: 1, title: "Alabaster", position: 1 }),
        playlistTrack({ id: 2, title: "Borrowed Light", position: 2 }),
      ],
      [playlistTrack({ id: 3, title: "Cinder", position: 3 })],
      [playlistTrack({ id: 4, title: "Driftwood", position: 4 })],
    ];
    const offsets = [0, 2, 3];

    afterEach(() => {
      audioPlayerActionsMock.isQueueLive.mockImplementation(() => true);
    });

    // Three pages. Every page after the first waits until it is released:
    // in jsdom every row counts as visible, so the list's own loader would
    // otherwise fetch the whole playlist before Play is pressed and leave the
    // button nothing to drain. release() frees them all, releasePage one.
    function mockPagedPlaylist() {
      const gates = new Map(
        offsets.slice(1).map((offset) => {
          let open = () => {};
          const opened = new Promise<void>((resolve) => {
            open = resolve;
          });
          return [offset, { open, opened }] as const;
        }),
      );
      const releasePage = (offset: number) => gates.get(offset)!.open();
      const release = () => gates.forEach((gate) => gate.open());
      const fetchMock = mockPlaylistFetch(roadTrip({ track_count: 4 }, pages.flat()));
      const respond = fetchMock.getMockImplementation()!;
      fetchMock.mockImplementation((input, init) => {
        const index = offsets.findIndex((offset) => requestURL(input) === TRACKS_PAGE_URL(offset));
        if (index < 0) return respond(input, init);
        const last = index === offsets.length - 1;
        const page = () =>
          jsonResponse({
            error: false,
            data: {
              tracks: pages[index],
              total: 4,
              has_more: !last,
              next_offset: last ? 4 : offsets[index + 1],
            },
          });
        return index === 0 ? page() : gates.get(offsets[index])!.opened.then(page);
      });
      const requested = (offset: number) =>
        fetchMock.mock.calls.some(([input]) => requestURL(input) === TRACKS_PAGE_URL(offset));
      return { release, releasePage, requested };
    }

    const startPlayAll = async () => {
      const user = userEvent.setup();
      const view = await renderRoute("/music/playlist/21");
      expect(await screen.findByText("Alabaster")).toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Play all 4 tracks" }));
      return view;
    };

    // The control for the test below: every page is queued while the queue
    // they are for is still the one playing.
    it("queues every page while the queue is still playing", async () => {
      audioPlayerActionsMock.playQueue.mockReturnValueOnce(7);
      const { release } = mockPagedPlaylist();

      await startPlayAll();
      release();

      await waitFor(() => {
        expect(audioPlayerActionsMock.extendQueue).toHaveBeenCalledTimes(1);
      });
      const [queued, queueId] = audioPlayerActionsMock.extendQueue.mock.calls[0];
      expect(queued.map((track) => track.id)).toEqual([1, 2, 3, 4]);
      expect(queueId).toBe(7);
      expect(showActionFailedMock).not.toHaveBeenCalled();
    });

    // Replacing the queue (another start, Stop, or a sign-out, which stops the
    // player) leaves nothing to load the rest for: the drain ends at the next
    // page, adds nothing, and says nothing about a shortfall.
    it("stops at the next page once the queue has been replaced", async () => {
      let live = true;
      audioPlayerActionsMock.playQueue.mockReturnValueOnce(7);
      audioPlayerActionsMock.isQueueLive.mockImplementation(
        (queueId) => live && queueId === 7,
      );
      const { release } = mockPagedPlaylist();

      await startPlayAll();
      const playAll = screen.getByRole("button", { name: "Play all 4 tracks" });
      expect(within(playAll).getByRole("status")).toBeInTheDocument();
      live = false;
      release();

      // The button stops spinning once the drain is over.
      await waitFor(() => {
        expect(within(playAll).queryByRole("status")).toBeNull();
      });
      expect(audioPlayerActionsMock.extendQueue).not.toHaveBeenCalled();
      expect(showActionFailedMock).not.toHaveBeenCalled();
      expect(audioPlayerActionsMock.isQueueLive).toHaveBeenCalledWith(7);
    });

    // The queue outlives the page: a reader who presses Play all and walks
    // away still gets the whole playlist, and no failure toast for a page that
    // was only cancelled because the page's own query observer went away.
    // Left while page 3 is in flight: a cancelled page inside the loop adds
    // no page, which is what used to end the drain short.
    it("keeps loading the rest after the reader leaves the page", async () => {
      audioPlayerActionsMock.playQueue.mockReturnValueOnce(7);
      const { releasePage, requested } = mockPagedPlaylist();

      const { unmount } = await startPlayAll();
      releasePage(2);
      await waitFor(() => expect(requested(3)).toBe(true));
      unmount();
      releasePage(3);

      await waitFor(() => {
        expect(audioPlayerActionsMock.extendQueue).toHaveBeenCalledTimes(1);
      });
      const [queued] = audioPlayerActionsMock.extendQueue.mock.calls[0];
      expect(queued.map((track) => track.id)).toEqual([1, 2, 3, 4]);
      expect(showActionFailedMock).not.toHaveBeenCalled();
    });

    // Unmounted so only the drain asks for pages: on a mounted page the list's
    // own loader fetches every visible page too, which would hide whether the
    // drain stopped.
    it("asks for no page after the one in flight once the queue is replaced", async () => {
      let live = true;
      audioPlayerActionsMock.playQueue.mockReturnValueOnce(7);
      audioPlayerActionsMock.isQueueLive.mockImplementation(
        (queueId) => live && queueId === 7,
      );
      const { release, requested } = mockPagedPlaylist();

      const { unmount } = await startPlayAll();
      unmount();
      live = false;
      release();

      // Asked once between pages, which ends the drain, and once after it.
      await waitFor(() => {
        expect(audioPlayerActionsMock.isQueueLive).toHaveBeenCalledTimes(2);
      });
      expect(requested(3)).toBe(false);
      expect(audioPlayerActionsMock.extendQueue).not.toHaveBeenCalled();
      expect(showActionFailedMock).not.toHaveBeenCalled();
    });
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
