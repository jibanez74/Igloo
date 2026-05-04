import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import {
  adminGetUsers,
  getAlbumDetails,
  getAlbumsPaginated,
  getAuthUser,
  getGeneralSettings,
  getPlaybackSettings,
  getLatestAlbums,
  getLatestMovies,
  getLikedMovies,
  getLikedTracks,
  getLikedTrackIds,
  getMovieDetails,
  getMovieInTheaterDetails,
  getMoviePlaylistDetails,
  getMoviePlaylistMovies,
  getMoviePlaylists,
  getMoviesInTheaters,
  getMovieGenresWithCounts,
  getMovieLikeStatus,
  getMoviesByGenre,
  getMoviesLibrary,
  getMoviesStats,
  getMovieTechnicalDetails,
  getMovieWatchProgress,
  getMusicianDetails,
  getMusiciansPaginated,
  getMusicStats,
  getPlaylistDetails,
  getPlaylists,
  getPlaylistTracks,
  getSettings,
  getTracksPaginated,
  getWatchRoom,
  getWatchRoomInviteUsers,
  getWatchRooms,
  searchAll,
  searchAlbums,
  searchMovies,
  searchMusicians,
  searchTracks,
} from "@/lib/api";
import {
  ADMIN_USERS_KEY,
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  AUTH_USER_KEY,
  GENERAL_SETTINGS_KEY,
  PLAYBACK_SETTINGS_KEY,
  LATEST_ALBUMS_KEY,
  LATEST_MOVIES_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
  MOVIE_DETAILS_KEY,
  MOVIES_IN_THEATERS_KEY,
  MOVIES_BY_GENRE_KEY,
  MOVIES_GENRES_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIE_LIKE_STATUS_KEY,
  MOVIES_PER_PAGE,
  MOVIES_STATS_KEY,
  MUSICIAN_DETAILS_KEY,
  MUSICIANS_PAGINATED_KEY,
  MUSIC_STATS_KEY,
  LIKED_TRACK_IDS_KEY,
  LIKED_TRACKS_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  SETTINGS_KEY,
  TRACKS_INFINITE_KEY,
  WATCH_ROOM_KEY,
  WATCH_ROOM_INVITE_USERS_KEY,
  WATCH_ROOMS_KEY,
  SEARCH_ALL_KEY,
  SEARCH_MOVIES_KEY,
  SEARCH_ALBUMS_KEY,
  SEARCH_MUSICIANS_KEY,
  SEARCH_TRACKS_KEY,
  SEARCH_PER_PAGE,
} from "@/lib/constants";

/**
 * TanStack Query cache tuning (v5: `gcTime` replaces former `cacheTime`).
 *
 * - staleTime: while fresh, the query will not refetch on mount/window focus.
 * - gcTime: how long inactive cached data is kept after the last observer unmounts.
 *
 * Library / user-editable data uses shorter stale times; TMDB-style catalogs longer.
 */
const MIN = 60_000;
/** 1 min — current user; also long scrolling lists where tracks can change */
const STALE_1M = MIN;
const STALE_LIST = 2 * MIN;
const STALE_CATALOG = 5 * MIN;
const STALE_THEATERS = 10 * MIN;
const STALE_TECH = 10 * MIN;
const STALE_30S = 30_000;

const GC_DEFAULT = 10 * MIN;
const GC_LONG = 30 * MIN;

export function adminUsersQueryOpts() {
  return queryOptions({
    queryKey: [ADMIN_USERS_KEY],
    queryFn: adminGetUsers,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function authUserQueryOpts() {
  return queryOptions({
    queryKey: [AUTH_USER_KEY],
    queryFn: getAuthUser,
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function latestMoviesQueryOpts() {
  return queryOptions({
    queryKey: [LATEST_MOVIES_KEY],
    queryFn: getLatestMovies,
    staleTime: STALE_CATALOG,
    gcTime: GC_LONG,
  });
}

export function latestAlbumsQueryOpts() {
  return queryOptions({
    queryKey: [LATEST_ALBUMS_KEY],
    queryFn: getLatestAlbums,
    staleTime: STALE_CATALOG,
    gcTime: GC_LONG,
  });
}

export function inTheatersQueryOpts() {
  return queryOptions({
    queryKey: [MOVIES_IN_THEATERS_KEY],
    queryFn: getMoviesInTheaters,
    staleTime: STALE_THEATERS,
    gcTime: GC_LONG,
  });
}

export function movieDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_DETAILS_KEY, id],
    queryFn: () => getMovieInTheaterDetails(id),
    enabled: id > 0,
    staleTime: STALE_CATALOG,
    gcTime: GC_LONG,
  });
}

export function libraryMovieDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [LIBRARY_MOVIE_DETAILS_KEY, id],
    queryFn: () => getMovieDetails(id),
    enabled: id > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function movieTechnicalDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_TECHNICAL_DETAILS_KEY, id],
    queryFn: () => getMovieTechnicalDetails(id),
    enabled: id > 0,
    staleTime: STALE_TECH,
    gcTime: GC_LONG,
  });
}

export function movieWatchProgressQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_WATCH_PROGRESS_KEY, id],
    queryFn: () => getMovieWatchProgress(id),
    enabled: id > 0,
    staleTime: STALE_30S,
    gcTime: GC_DEFAULT,
  });
}

export function albumDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [ALBUM_DETAILS_KEY, id],
    queryFn: () => getAlbumDetails(id),
    enabled: id > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function tracksInfiniteQueryOpts(pageSize = 50) {
  return infiniteQueryOptions({
    queryKey: [TRACKS_INFINITE_KEY, pageSize],
    queryFn: ({ pageParam = 0 }) => getTracksPaginated(pageSize, pageParam),
    initialPageParam: 0,
    getNextPageParam: lastPage => {
      if (lastPage.error || !lastPage.data?.has_more) return undefined;
      return lastPage.data.offset + lastPage.data.limit;
    },
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function musicStatsQueryOpts() {
  return queryOptions({
    queryKey: [MUSIC_STATS_KEY],
    queryFn: getMusicStats,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function settingsQueryOpts() {
  return queryOptions({
    queryKey: [SETTINGS_KEY],
    queryFn: getSettings,
    staleTime: STALE_CATALOG,
    gcTime: GC_LONG,
  });
}

export function generalSettingsQueryOpts() {
  return queryOptions({
    queryKey: [GENERAL_SETTINGS_KEY],
    queryFn: getGeneralSettings,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function playbackSettingsQueryOpts() {
  return queryOptions({
    queryKey: [PLAYBACK_SETTINGS_KEY],
    queryFn: getPlaybackSettings,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function albumsPaginatedQueryOpts(page: number, perPage: number = 24) {
  return queryOptions({
    queryKey: [ALBUMS_PAGINATED_KEY, page, perPage],
    queryFn: () => getAlbumsPaginated(page, perPage),
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function musiciansPaginatedQueryOpts(
  page: number,
  perPage: number = 24,
) {
  return queryOptions({
    queryKey: [MUSICIANS_PAGINATED_KEY, page, perPage],
    queryFn: () => getMusiciansPaginated(page, perPage),
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function musicianDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MUSICIAN_DETAILS_KEY, id],
    queryFn: () => getMusicianDetails(id),
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

// Playlist query options
export function playlistsQueryOpts() {
  return queryOptions({
    queryKey: [PLAYLISTS_KEY],
    queryFn: getPlaylists,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function playlistDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [PLAYLIST_DETAILS_KEY, id],
    queryFn: () => getPlaylistDetails(id),
    enabled: id > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function playlistTracksInfiniteQueryOpts(
  playlistId: number,
  pageSize = 50,
) {
  return infiniteQueryOptions({
    queryKey: [PLAYLIST_TRACKS_KEY, playlistId, pageSize],
    queryFn: ({ pageParam = 0 }) =>
      getPlaylistTracks(playlistId, pageSize, pageParam),
    initialPageParam: 0,
    getNextPageParam: lastPage => {
      if (lastPage.error || !lastPage.data?.has_more) return undefined;
      return lastPage.data.next_offset;
    },
    enabled: playlistId > 0,
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

// Movies library page (/movies)
export function moviesLibraryQueryOpts(
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) {
  return queryOptions({
    queryKey: [MOVIES_LIBRARY_KEY, page, perPage, sort],
    queryFn: () => getMoviesLibrary(page, perPage, sort),
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function moviesGenresQueryOpts() {
  return queryOptions({
    queryKey: [MOVIES_GENRES_KEY],
    queryFn: getMovieGenresWithCounts,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function moviesByGenreQueryOpts(
  genreId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) {
  return queryOptions({
    queryKey: [MOVIES_BY_GENRE_KEY, genreId, page, perPage, sort],
    queryFn: () => getMoviesByGenre(genreId, page, perPage, sort),
    enabled: genreId > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function moviesStatsQueryOpts() {
  return queryOptions({
    queryKey: [MOVIES_STATS_KEY],
    queryFn: getMoviesStats,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function likedMoviesQueryOpts(
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) {
  return queryOptions({
    queryKey: [MOVIES_LIKED_KEY, page, perPage, sort],
    queryFn: () => getLikedMovies(page, perPage, sort),
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function movieLikeStatusQueryOpts(movieId: number) {
  return queryOptions({
    queryKey: [MOVIE_LIKE_STATUS_KEY, movieId],
    queryFn: () => getMovieLikeStatus(movieId),
    enabled: movieId > 0,
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function moviePlaylistsQueryOpts() {
  return queryOptions({
    queryKey: [MOVIE_PLAYLISTS_KEY],
    queryFn: getMoviePlaylists,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function moviePlaylistDetailsQueryOpts(id: number) {
  return queryOptions({
    queryKey: [MOVIE_PLAYLIST_DETAILS_KEY, id],
    queryFn: () => getMoviePlaylistDetails(id),
    enabled: id > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function moviePlaylistMoviesQueryOpts(
  playlistId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) {
  return queryOptions({
    queryKey: [MOVIE_PLAYLIST_MOVIES_KEY, playlistId, page, perPage, sort],
    queryFn: () => getMoviePlaylistMovies(playlistId, page, perPage, sort),
    enabled: playlistId > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function likedTrackIdsQueryOpts() {
  return queryOptions({
    queryKey: [LIKED_TRACK_IDS_KEY],
    queryFn: getLikedTrackIds,
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function likedTracksQueryOpts(page: number, perPage: number = 50) {
  return queryOptions({
    queryKey: [LIKED_TRACKS_KEY, page, perPage],
    queryFn: () => getLikedTracks(page, perPage),
    staleTime: STALE_1M,
    gcTime: GC_DEFAULT,
  });
}

export function watchRoomsQueryOpts() {
  return queryOptions({
    queryKey: [WATCH_ROOMS_KEY],
    queryFn: getWatchRooms,
    staleTime: STALE_30S,
    gcTime: GC_DEFAULT,
  });
}

export function watchRoomQueryOpts(id: number) {
  return queryOptions({
    queryKey: [WATCH_ROOM_KEY, id],
    queryFn: () => getWatchRoom(id),
    enabled: id > 0,
    staleTime: STALE_30S,
    gcTime: GC_DEFAULT,
  });
}

export function watchRoomInviteUsersQueryOpts(enabled: boolean = true) {
  return queryOptions({
    queryKey: [WATCH_ROOM_INVITE_USERS_KEY],
    queryFn: getWatchRoomInviteUsers,
    enabled,
    staleTime: STALE_30S,
    gcTime: GC_DEFAULT,
  });
}

// Library search (FTS5-backed). All factories disable themselves for empty
// queries so the loader/component can call them unconditionally.

export function searchAllQueryOpts(q: string) {
  const trimmed = q.trim();
  return queryOptions({
    queryKey: [SEARCH_ALL_KEY, trimmed],
    queryFn: () => searchAll(trimmed),
    enabled: trimmed.length > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function searchMoviesQueryOpts(
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) {
  const trimmed = q.trim();
  return queryOptions({
    queryKey: [SEARCH_MOVIES_KEY, trimmed, page, perPage],
    queryFn: () => searchMovies(trimmed, page, perPage),
    enabled: trimmed.length > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function searchAlbumsQueryOpts(
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) {
  const trimmed = q.trim();
  return queryOptions({
    queryKey: [SEARCH_ALBUMS_KEY, trimmed, page, perPage],
    queryFn: () => searchAlbums(trimmed, page, perPage),
    enabled: trimmed.length > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function searchMusiciansQueryOpts(
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) {
  const trimmed = q.trim();
  return queryOptions({
    queryKey: [SEARCH_MUSICIANS_KEY, trimmed, page, perPage],
    queryFn: () => searchMusicians(trimmed, page, perPage),
    enabled: trimmed.length > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}

export function searchTracksQueryOpts(
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) {
  const trimmed = q.trim();
  return queryOptions({
    queryKey: [SEARCH_TRACKS_KEY, trimmed, page, perPage],
    queryFn: () => searchTracks(trimmed, page, perPage),
    enabled: trimmed.length > 0,
    staleTime: STALE_LIST,
    gcTime: GC_DEFAULT,
  });
}
