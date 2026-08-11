import type { QueryClient } from "@tanstack/react-query";
import {
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  LATEST_ALBUMS_KEY,
  LIKED_TRACK_IDS_KEY,
  LIKED_TRACKS_KEY,
  MUSIC_STATS_KEY,
  MUSICIAN_DETAILS_KEY,
  MUSICIANS_PAGINATED_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  TRACKS_INFINITE_KEY,
} from "@/lib/constants";
import { isApiFailure } from "@/lib/is-api-failure";

const MUSIC_LIBRARY_QUERY_KEYS = [
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
] as const;

export async function refreshMusicLibraryCache(queryClient: QueryClient) {
  await Promise.all(
    MUSIC_LIBRARY_QUERY_KEYS.map(async key => {
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
