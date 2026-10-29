import { InfiniteQueryObserver } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTINUE_WATCHING_KEY,
  SEARCH_ALL_KEY,
  SEARCH_CATEGORY_KEYS,
  WATCH_ROOM_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import {
  invalidateArtworkQueries,
  invalidateLibraryQueries,
  refreshLibraryWithToasts,
} from "@/lib/library-cache";
import { tracksInfiniteQueryOpts } from "@/lib/query-opts";
import { jsonResponse } from "../helpers/api";
import { createTestQueryClient } from "../helpers/render";

const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
}));

const KEYS = {
  searchAll: [SEARCH_ALL_KEY, "harbor"],
  searchMovies: [SEARCH_CATEGORY_KEYS.movies, "harbor", 1, 24],
  searchShows: [SEARCH_CATEGORY_KEYS.shows, "harbor", 1, 24],
  searchAlbums: [SEARCH_CATEGORY_KEYS.albums, "harbor", 1, 24],
  continueWatching: [CONTINUE_WATCHING_KEY],
  watchRooms: [WATCH_ROOMS_KEY],
  watchRoom: [WATCH_ROOM_KEY, 5],
};

function seededClient() {
  const queryClient = createTestQueryClient();
  for (const key of Object.values(KEYS)) {
    queryClient.setQueryData(key, { error: false, data: {} });
  }
  return queryClient;
}

function staleKeys(queryClient: ReturnType<typeof createTestQueryClient>) {
  return Object.entries(KEYS)
    .filter(([, key]) => queryClient.getQueryState(key)?.isInvalidated)
    .map(([name]) => name);
}

describe("invalidateLibraryQueries", () => {
  it("reaches the movie search results, continue watching and the watch rooms", () => {
    const queryClient = seededClient();

    invalidateLibraryQueries(queryClient, "movies");

    expect(staleKeys(queryClient)).toEqual([
      "searchAll",
      "searchMovies",
      "continueWatching",
      "watchRooms",
      "watchRoom",
    ]);
  });

  it("reaches the show search results and continue watching", () => {
    const queryClient = seededClient();

    invalidateLibraryQueries(queryClient, "shows");

    expect(staleKeys(queryClient)).toEqual([
      "searchAll",
      "searchShows",
      "continueWatching",
    ]);
  });

  it("reaches the music search results only", () => {
    const queryClient = seededClient();

    invalidateLibraryQueries(queryClient, "music");

    expect(staleKeys(queryClient)).toEqual(["searchAll", "searchAlbums"]);
  });
});

describe("invalidateArtworkQueries", () => {
  it("reaches an open watch room, whose poster may be a deleted copy", () => {
    const queryClient = seededClient();

    invalidateArtworkQueries(queryClient);

    expect(staleKeys(queryClient)).toContain("watchRoom");
  });
});

describe("refreshLibraryWithToasts", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reports a refresh whose track page failed instead of toasting success", async () => {
    let failTracks = false;
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        failTracks
          ? jsonResponse({ error: true, message: "tracks query failed" }, 503)
          : jsonResponse({
              error: false,
              data: { tracks: [], total: 0, offset: 0, limit: 50, has_more: false },
            }),
      ),
    );
    const queryClient = createTestQueryClient();
    const observer = new InfiniteQueryObserver(
      queryClient,
      tracksInfiniteQueryOpts(),
    );
    const unsubscribe = observer.subscribe(() => {});
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true));

    failTracks = true;
    await refreshLibraryWithToasts(queryClient, "music");
    unsubscribe();

    expect(showSuccessMock).not.toHaveBeenCalled();
    expect(showActionFailedMock).toHaveBeenCalledWith(
      "refresh library",
      "Unable to refresh the music library. Please try again.",
    );
  });
});
