import type { QueryClient } from "@tanstack/react-query";
import {
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  CONTINUE_WATCHING_KEY,
  EPISODE_KEY,
  EPISODE_TECHNICAL_DETAILS_KEY,
  LATEST_ALBUMS_KEY,
  LATEST_MOVIES_KEY,
  LATEST_SHOWS_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  LIKED_TRACK_IDS_KEY,
  LIKED_TRACKS_KEY,
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIES_BY_GENRE_KEY,
  MOVIES_GENRES_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIES_STATS_KEY,
  MUSIC_STATS_KEY,
  MUSICIAN_DETAILS_KEY,
  MUSICIANS_PAGINATED_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  SEARCH_ALL_KEY,
  SEARCH_CATEGORY_KEYS,
  SHOW_DETAILS_KEY,
  SHOW_SEASON_EPISODES_KEY,
  SHOWS_BY_GENRE_KEY,
  SHOWS_GENRES_KEY,
  SHOWS_LIBRARY_KEY,
  SHOWS_STATS_KEY,
  TRACKS_INFINITE_KEY,
  WATCH_ROOM_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import { isApiFailure } from "@/lib/is-api-failure";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";

// Every query a scan or a bulk change to one library can leave stale: its
// lists, details, statistics and playlists, the search results that list its
// items, and the rows elsewhere that show them (continue watching; the watch
// room list's and an open room's movie title and poster).
const LIBRARY_QUERY_KEYS = {
  movies: [
    MOVIES_STATS_KEY,
    MOVIES_LIBRARY_KEY,
    MOVIES_GENRES_KEY,
    MOVIES_BY_GENRE_KEY,
    MOVIES_LIKED_KEY,
    LATEST_MOVIES_KEY,
    LIBRARY_MOVIE_DETAILS_KEY,
    MOVIE_TECHNICAL_DETAILS_KEY,
    MOVIE_PLAYLISTS_KEY,
    MOVIE_PLAYLIST_DETAILS_KEY,
    MOVIE_PLAYLIST_MOVIES_KEY,
    SEARCH_ALL_KEY,
    SEARCH_CATEGORY_KEYS.movies,
    CONTINUE_WATCHING_KEY,
    WATCH_ROOMS_KEY,
    WATCH_ROOM_KEY,
  ],
  music: [
    MUSIC_STATS_KEY,
    ALBUMS_PAGINATED_KEY,
    ALBUM_DETAILS_KEY,
    LATEST_ALBUMS_KEY,
    TRACKS_INFINITE_KEY,
    LIKED_TRACKS_KEY,
    LIKED_TRACK_IDS_KEY,
    MUSICIANS_PAGINATED_KEY,
    MUSICIAN_DETAILS_KEY,
    PLAYLISTS_KEY,
    PLAYLIST_DETAILS_KEY,
    PLAYLIST_TRACKS_KEY,
    SEARCH_ALL_KEY,
    SEARCH_CATEGORY_KEYS.albums,
    SEARCH_CATEGORY_KEYS.musicians,
    SEARCH_CATEGORY_KEYS.tracks,
  ],
  shows: [
    SHOWS_STATS_KEY,
    SHOWS_LIBRARY_KEY,
    SHOWS_GENRES_KEY,
    SHOWS_BY_GENRE_KEY,
    LATEST_SHOWS_KEY,
    SHOW_DETAILS_KEY,
    SHOW_SEASON_EPISODES_KEY,
    EPISODE_KEY,
    EPISODE_TECHNICAL_DETAILS_KEY,
    SEARCH_ALL_KEY,
    SEARCH_CATEGORY_KEYS.shows,
    CONTINUE_WATCHING_KEY,
  ],
} as const;

export type Library = keyof typeof LIBRARY_QUERY_KEYS;

/** Capitalised, singular: it opens the toast copy and is lowercased mid-sentence. */
const REFRESH_TOAST_NOUNS: Record<Library, string> = {
  movies: "Movie",
  music: "Music",
  shows: "Show",
};

/** Marks every query under the library's keys stale; active ones refetch on their own. */
export function invalidateLibraryQueries(
  queryClient: QueryClient,
  library: Library,
) {
  for (const key of LIBRARY_QUERY_KEYS[library]) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}

const artworkResetListeners = new Set<() => void>();

/**
 * Calls `listener` whenever invalidateArtworkQueries runs, for state that
 * holds image URLs outside the query cache, such as the audio queue. Returns
 * the unsubscribe function.
 */
export function onArtworkReset(listener: () => void) {
  artworkResetListeners.add(listener);
  return () => {
    artworkResetListeners.delete(listener);
  };
}

/**
 * Marks stale every query that shows library artwork: each library's, and the
 * person pages, whose photos and credits come from the libraries too. A
 * settings save that stops serving the downloaded copies calls it, since the
 * cached image URLs point at files the server deletes or no longer serves;
 * the onArtworkReset listeners then refresh the URLs they hold.
 */
export function invalidateArtworkQueries(queryClient: QueryClient) {
  for (const library of Object.keys(LIBRARY_QUERY_KEYS) as Library[]) {
    invalidateLibraryQueries(queryClient, library);
  }
  void queryClient.invalidateQueries({ queryKey: ["person"] });
  for (const listener of artworkResetListeners) {
    listener();
  }
}

/**
 * Drops the library's inactive queries and refetches the active ones,
 * rejecting if any refetch fails or lands on an API error envelope, so the
 * caller's toast reflects what the user will actually see.
 */
async function refreshLibraryQueries(queryClient: QueryClient, library: Library) {
  await Promise.all(
    LIBRARY_QUERY_KEYS[library].map(async key => {
      queryClient.removeQueries({ queryKey: [key], type: "inactive" });
      await queryClient.refetchQueries(
        { queryKey: [key], type: "active" },
        { throwOnError: true },
      );

      const refreshedQueries = queryClient.getQueriesData({
        queryKey: [key],
        type: "active",
      });

      for (const [, data] of refreshedQueries) {
        if (isApiFailure(data)) {
          throw new Error(data.message);
        }
      }
    }),
  );
}

/**
 * Refreshes a library's cache with the shared success/failure toasts. Never
 * throws, so component handlers can reset their UI state with plain
 * sequential code instead of a `finally` block (which bails React Compiler
 * out of the whole component).
 */
export async function refreshLibraryWithToasts(
  queryClient: QueryClient,
  library: Library,
): Promise<void> {
  const noun = REFRESH_TOAST_NOUNS[library];

  try {
    await refreshLibraryQueries(queryClient, library);
    showSuccess("Library refreshed", `${noun} library data is up to date.`);
  } catch (error) {
    console.error(`Failed to refresh ${noun.toLowerCase()} library:`, error);
    showActionFailed(
      "refresh library",
      `Unable to refresh the ${noun.toLowerCase()} library. Please try again.`,
    );
  }
}
