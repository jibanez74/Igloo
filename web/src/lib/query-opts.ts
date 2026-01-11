import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import {
  getAlbumDetails,
  getAuthUser,
  getLatestAlbums,
  getMovieInTheaterDetails,
  getMoviesInTheaters,
  getMusicStats,
  getTracksPaginated,
} from "@/lib/api";
import {
  ALBUM_DETAILS_KEY,
  AUTH_USER_KEY,
  LATEST_ALBUMS_KEY,
  MOVIE_DETAILS_KEY,
  MOVIES_IN_THEATERS_KEY,
  MUSIC_STATS_KEY,
  TRACKS_INFINITE_KEY,
} from "@/lib/constants";

export function authUserQueryOpts() {
  return queryOptions({
    queryKey: [AUTH_USER_KEY],
    queryFn: getAuthUser,
  });
}

export function latestAlbumsQueryOpts() {
  return queryOptions({
    queryKey: [LATEST_ALBUMS_KEY],
    queryFn: getLatestAlbums,
  });
}

export function inTheatersQueryOpts() {
  return queryOptions({
    queryKey: [MOVIES_IN_THEATERS_KEY],
    queryFn: getMoviesInTheaters,
  });
}

export function movieDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_DETAILS_KEY, id],
    queryFn: () => getMovieInTheaterDetails(id),
    enabled: id > 0,
  });
}

export function albumDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [ALBUM_DETAILS_KEY, id],
    queryFn: () => getAlbumDetails(id),
  });
}

export function tracksInfiniteQueryOpts(pageSize = 50) {
  return infiniteQueryOptions({
    queryKey: [TRACKS_INFINITE_KEY],
    queryFn: ({ pageParam = 0 }) => getTracksPaginated(pageSize, pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      if (lastPage.error || !lastPage.data?.has_more) return undefined;
      return lastPage.data.offset + lastPage.data.limit;
    },
  });
}

export function musicStatsQueryOpts() {
  return queryOptions({
    queryKey: [MUSIC_STATS_KEY],
    queryFn: getMusicStats,
  });
}
