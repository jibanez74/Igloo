import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import {
  getAlbumDetails,
  getAlbumsPaginated,
  getAuthUser,
  getLatestAlbums,
  getLatestMovies,
  getMovieDetails,
  getMovieInTheaterDetails,
  getMoviesInTheaters,
  getMusicianDetails,
  getMusiciansPaginated,
  getMusicStats,
  getPlaylistDetails,
  getPlaylists,
  getPlaylistTracks,
  getSettings,
  getTracksPaginated,
} from "@/lib/api";
import {
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  AUTH_USER_KEY,
  LATEST_ALBUMS_KEY,
  LATEST_MOVIES_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_DETAILS_KEY,
  MOVIES_IN_THEATERS_KEY,
  MUSICIAN_DETAILS_KEY,
  MUSICIANS_PAGINATED_KEY,
  MUSIC_STATS_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  SETTINGS_KEY,
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

export function latestMoviesQueryOpts() {
  return queryOptions({
    queryKey: [LATEST_MOVIES_KEY],
    queryFn: getLatestMovies,
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

export function libraryMovieDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [LIBRARY_MOVIE_DETAILS_KEY, id],
    queryFn: () => getMovieDetails(id),
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
    getNextPageParam: lastPage => {
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

export function settingsQueryOpts() {
  return queryOptions({
    queryKey: [SETTINGS_KEY],
    queryFn: getSettings,
  });
}

export function albumsPaginatedQueryOpts(page: number, perPage: number = 24) {
  return queryOptions({
    queryKey: [ALBUMS_PAGINATED_KEY, page, perPage],
    queryFn: () => getAlbumsPaginated(page, perPage),
  });
}

export function musiciansPaginatedQueryOpts(
  page: number,
  perPage: number = 24,
) {
  return queryOptions({
    queryKey: [MUSICIANS_PAGINATED_KEY, page, perPage],
    queryFn: () => getMusiciansPaginated(page, perPage),
  });
}

export function musicianDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MUSICIAN_DETAILS_KEY, id],
    queryFn: () => getMusicianDetails(id),
  });
}

// Playlist query options
export function playlistsQueryOpts() {
  return queryOptions({
    queryKey: [PLAYLISTS_KEY],
    queryFn: getPlaylists,
  });
}

export function playlistDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [PLAYLIST_DETAILS_KEY, id],
    queryFn: () => getPlaylistDetails(id),
    enabled: id > 0,
  });
}

export function playlistTracksInfiniteQueryOpts(
  playlistId: number,
  pageSize = 50,
) {
  return infiniteQueryOptions({
    queryKey: [PLAYLIST_TRACKS_KEY, playlistId],
    queryFn: ({ pageParam = 0 }) =>
      getPlaylistTracks(playlistId, pageSize, pageParam),
    initialPageParam: 0,
    getNextPageParam: lastPage => {
      if (lastPage.error || !lastPage.data?.has_more) return undefined;
      return lastPage.data.next_offset;
    },
    enabled: playlistId > 0,
  });
}
