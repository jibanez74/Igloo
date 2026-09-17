import type { QueryClient } from "@tanstack/react-query";
import {
  EPISODE_KEY,
  EPISODE_TECHNICAL_DETAILS_KEY,
  LATEST_SHOWS_KEY,
  SHOW_DETAILS_KEY,
  SHOW_SEASON_EPISODES_KEY,
  SHOWS_BY_GENRE_KEY,
  SHOWS_GENRES_KEY,
  SHOWS_LIBRARY_KEY,
  SHOWS_STATS_KEY,
} from "@/lib/constants";
import {
  invalidateLibraryQueryKeys,
  refreshLibraryQueryKeys,
} from "@/lib/library-refresh";

const SHOW_LIBRARY_QUERY_KEYS = [
  SHOWS_STATS_KEY,
  SHOWS_LIBRARY_KEY,
  SHOWS_GENRES_KEY,
  SHOWS_BY_GENRE_KEY,
  LATEST_SHOWS_KEY,
  SHOW_DETAILS_KEY,
  SHOW_SEASON_EPISODES_KEY,
  EPISODE_KEY,
  EPISODE_TECHNICAL_DETAILS_KEY,
] as const;

export function invalidateShowLibraryQueries(queryClient: QueryClient) {
  invalidateLibraryQueryKeys(queryClient, SHOW_LIBRARY_QUERY_KEYS);
}

export async function refreshShowLibraryCache(queryClient: QueryClient) {
  await refreshLibraryQueryKeys(queryClient, SHOW_LIBRARY_QUERY_KEYS);
}
