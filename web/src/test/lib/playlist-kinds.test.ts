import { describe, expect, it } from "vitest";
import {
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
} from "@/lib/constants";
import { invalidatePlaylistContents } from "@/lib/playlist-kinds";
import { createTestQueryClient } from "../helpers/render";

function seed(keys: readonly (readonly unknown[])[]) {
  const queryClient = createTestQueryClient();
  for (const key of keys) {
    queryClient.setQueryData(key, { error: false, data: {} });
  }
  return queryClient;
}

function isStale(
  queryClient: ReturnType<typeof createTestQueryClient>,
  key: readonly unknown[],
) {
  return queryClient.getQueryState(key)?.isInvalidated ?? false;
}

describe("invalidatePlaylistContents", () => {
  it("marks each grown playlist's items and details, and the list once", async () => {
    const touched = [
      [PLAYLIST_TRACKS_KEY, 1, 50],
      [PLAYLIST_DETAILS_KEY, 1],
      [PLAYLIST_TRACKS_KEY, 2, 50],
      [PLAYLIST_DETAILS_KEY, 2],
      [PLAYLISTS_KEY],
    ] as const;
    const untouched = [
      [PLAYLIST_TRACKS_KEY, 3, 50],
      [PLAYLIST_DETAILS_KEY, 3],
      [MOVIE_PLAYLISTS_KEY],
      [MOVIE_PLAYLIST_DETAILS_KEY, 1],
    ] as const;
    const queryClient = seed([...touched, ...untouched]);

    await invalidatePlaylistContents(queryClient, "music", [1, 2]);

    for (const key of touched) expect(isStale(queryClient, key)).toBe(true);
    for (const key of untouched) expect(isStale(queryClient, key)).toBe(false);
  });

  it("uses the movie playlist keys for a movie playlist", async () => {
    const queryClient = seed([
      [MOVIE_PLAYLIST_MOVIES_KEY, 4, 1, 24, "asc"],
      [MOVIE_PLAYLIST_DETAILS_KEY, 4],
      [MOVIE_PLAYLISTS_KEY],
      [PLAYLISTS_KEY],
    ]);

    await invalidatePlaylistContents(queryClient, "movie", [4]);

    expect(isStale(queryClient, [MOVIE_PLAYLIST_MOVIES_KEY, 4, 1, 24, "asc"])).toBe(true);
    expect(isStale(queryClient, [MOVIE_PLAYLIST_DETAILS_KEY, 4])).toBe(true);
    expect(isStale(queryClient, [MOVIE_PLAYLISTS_KEY])).toBe(true);
    expect(isStale(queryClient, [PLAYLISTS_KEY])).toBe(false);
  });
});
