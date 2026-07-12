import type { QueryClient } from "@tanstack/react-query";
import {
  LATEST_MOVIES_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIES_BY_GENRE_KEY,
  MOVIES_GENRES_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIES_STATS_KEY,
} from "@/lib/constants";
import { isApiFailure } from "@/lib/is-api-failure";

const MOVIE_LIBRARY_QUERY_KEYS = [
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
] as const;

export function invalidateMovieLibraryQueries(queryClient: QueryClient) {
  MOVIE_LIBRARY_QUERY_KEYS.forEach(key => {
    void queryClient.invalidateQueries({ queryKey: [key] });
  });
}

export async function refreshMovieLibraryCache(queryClient: QueryClient) {
  await Promise.all(
    MOVIE_LIBRARY_QUERY_KEYS.map(async key => {
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
