import type {
  AlbumDetailsResponseType,
  AlbumsListResponseType,
  AdminUserType,
  ApiFailureType,
  ApiResponseType,
  AuthUser,
  CreateMoviePlaylistRequest,
  CreateNotificationRequest,
  CreateNotificationResponseType,
  CreatePlaylistRequest,
  CreateWatchRoomRequestType,
  CreateWatchRoomResponseType,
  JoinWatchRoomResponseType,
  LatestMovieType,
  LibraryMovieDetailsResponse,
  GeneralSettingsResponseType,
  PlaybackSettingsResponseType,
  MovieDetailsType,
  MoviePlaylistDetailResponseType,
  MoviePlaylistRowType,
  MoviePlaylistsListResponseType,
  MovieTechnicalDetailsResponse,
  MovieWatchProgressType,
  MovieGenreWithCountType,
  MoviesLibraryPaginatedDataType,
  MoviesStatsDataType,
  TmdbSearchResultType,
  MusicianDetailsResponseType,
  MusicStatsType,
  MusiciansListResponseType,
  PlaylistDetailResponseType,
  PlaylistsListResponseType,
  PlaylistSummaryType,
  PlaylistTracksResponseType,
  RecentlyPlayedResponseType,
  LikedTracksResponseType,
  ShuffleTracksResponseType,
  SettingsType,
  SimpleAlbumType,
  TheaterMovieType,
  TmdbStatusType,
  TmdbSearchMoviesRequest,
  TopAlbumsResponseType,
  TopGenresResponseType,
  TopMusiciansResponseType,
  TopTracksResponseType,
  TracksListResponseType,
  UpdateMovieMetadataRequest,
  UpdateMoviePlaylistRequest,
  UpdateGeneralSettingsRequest,
  UpdateGeneralSettingsResponseType,
  UpdateLibrarySettingsRequest,
  UpdateLibrarySettingsResponseType,
  UpdatePlaybackSettingsRequest,
  UpdatePlaybackSettingsResponseType,
  UpdatePlaylistRequest,
  UserListeningStatsResponseType,
  WatchRoomInviteUsersResponseType,
  WatchRoomResponseType,
  WatchRoomType,
  SearchAllResponseType,
  SearchMoviesResponseType,
  SearchAlbumsResponseType,
  SearchMusiciansResponseType,
  SearchTracksResponseType,
  SpotifyAlbumSearchRequest,
  SpotifyAlbumSearchResultType,
  SpotifyStatusType,
} from "@/types";
import { MOVIES_PER_PAGE, SEARCH_PER_PAGE } from "@/lib/constants";

// API Client - Generic request handler
const ERROR_NOTFOUND: ApiFailureType = {
  error: true,
  message: "404 - The resource you requested was not found.",
};

const NETWORK_ERROR: ApiFailureType = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
};

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type ApiRequestOptions = {
  method?: HttpMethod;
  body?: unknown;
};

/**
 * Generic API request handler that consolidates error handling and fetch configuration.
 * All API functions should use this to avoid repetition.
 */
async function apiRequest<T extends Record<string, unknown>>(
  endpoint: string,
  options: ApiRequestOptions = {},
): Promise<ApiResponseType<T>> {
  const { method = "GET", body } = options;

  try {
    const res = await fetch(endpoint, {
      method,
      credentials: "include",
      headers: body ? { "Content-Type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    return await res.json();
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

// Authentication API
export const login = (email: string, password: string) =>
  apiRequest("/api/auth/login", {
    method: "POST",
    body: { email, password },
  });

export const logout = () =>
  apiRequest("/api/auth/logout", {
    method: "DELETE",
  });

export const getAuthUser = () =>
  apiRequest<{ user: AuthUser }>("/api/auth/user");

// User Settings API
export const updateUserName = (name: string) =>
  apiRequest("/api/user/name", {
    method: "PUT",
    body: { name },
  });

export const updateUserEmail = (email: string) =>
  apiRequest("/api/user/email", {
    method: "PUT",
    body: { email },
  });

export const updateUserPassword = (
  currentPassword: string,
  newPassword: string,
) =>
  apiRequest("/api/user/password", {
    method: "PUT",
    body: { current_password: currentPassword, new_password: newPassword },
  });

export const updateUserAvatar = (avatar: string) =>
  apiRequest("/api/user/avatar", {
    method: "PUT",
    body: { avatar },
  });

export const uploadUserAvatar = async (
  file: File,
): Promise<ApiResponseType<{ user: AuthUser }>> => {
  const formData = new FormData();
  formData.append("avatar", file);

  try {
    const response = await fetch("/api/user/avatar/upload", {
      method: "POST",
      body: formData,
      credentials: "include",
    });

    const data = await response.json();
    return data;
  } catch {
    return {
      error: true,
      message: "Failed to upload avatar",
    };
  }
};

export const deleteUserAccount = () =>
  apiRequest("/api/user", {
    method: "DELETE",
  });

export const createNotification = (body: CreateNotificationRequest) =>
  apiRequest<CreateNotificationResponseType>("/api/notifications", {
    method: "POST",
    body,
  });

// Home Page API
export const getLatestAlbums = () =>
  apiRequest<{ albums: SimpleAlbumType[] }>("/api/music/albums/latest");

export const getLatestMovies = () =>
  apiRequest<{ movies: LatestMovieType[] }>("/api/movies/latest");

export const getMoviesInTheaters = () =>
  apiRequest<{ movies: TheaterMovieType[] }>("/api/tmdb/movies/in-theaters");

export const getMovieInTheaterDetails = (id: number) =>
  apiRequest<{ movie: MovieDetailsType }>(`/api/tmdb/movies/${id}`);

export const getTmdbStatus = () =>
  apiRequest<TmdbStatusType>("/api/tmdb/status");

export const getSpotifyStatus = () =>
  apiRequest<SpotifyStatusType>("/api/spotify/status");

// movie details
export const getMovieDetails = (id: number) =>
  apiRequest<LibraryMovieDetailsResponse>(`/api/movies/details/${id}`);

export const getMovieTechnicalDetails = (id: number) =>
  apiRequest<MovieTechnicalDetailsResponse>(
    `/api/movies/${id}/technical-details`,
  );

export const searchTmdbMovies = (body: TmdbSearchMoviesRequest) =>
  apiRequest<{ results: TmdbSearchResultType[] }>("/api/tmdb/movies/search", {
    method: "POST",
    body,
  });

export const searchSpotifyAlbums = (body: SpotifyAlbumSearchRequest) =>
  apiRequest<{ results: SpotifyAlbumSearchResultType[] }>(
    "/api/spotify/albums/search",
    {
      method: "POST",
      body,
    },
  );

export const tmdbSearchMovies = (
  movieId: number,
  body: TmdbSearchMoviesRequest,
) =>
  apiRequest<{ results: TmdbSearchResultType[] }>(
    `/api/movies/${movieId}/tmdb-search`,
    { method: "POST", body },
  );

export const identifyMovie = (movieId: number, tmdbId: number) =>
  apiRequest<Record<string, never>>(`/api/movies/${movieId}/identify`, {
    method: "PUT",
    body: { tmdb_id: tmdbId },
  });

export const updateMovieMetadata = (
  movieId: number,
  body: UpdateMovieMetadataRequest,
) =>
  apiRequest<Record<string, never>>(`/api/movies/${movieId}`, {
    method: "PATCH",
    body,
  });

export const deleteMovie = (movieId: number, deleteFile: boolean) =>
  apiRequest<Record<string, never>>(`/api/movies/${movieId}`, {
    method: "DELETE",
    body: { delete_file: deleteFile },
  });

// ============================================================================
// Movies library page (GET /api/movies/library, /stats, /liked, playlists, like)
// ============================================================================

export const getMoviesLibrary = (
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    `/api/movies/library?page=${page}&per_page=${perPage}&sort=${sort}`,
  );

export const getMovieGenresWithCounts = () =>
  apiRequest<{ genres: MovieGenreWithCountType[] }>("/api/movies/genres");

export const getMoviesByGenre = (
  genreId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    `/api/movies/genres/${genreId}/movies?page=${page}&per_page=${perPage}&sort=${sort}`,
  );

export const getMoviesStats = () =>
  apiRequest<MoviesStatsDataType>("/api/movies/stats");

export const getLikedMovies = (
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    `/api/movies/liked?page=${page}&per_page=${perPage}&sort=${sort}`,
  );

export const getMovieLikeStatus = (movieId: number) =>
  apiRequest<{ is_liked: boolean }>(`/api/movies/${movieId}/like-status`);

export const toggleLikeMovie = (movieId: number) =>
  apiRequest<{ movie_id: number; is_liked: boolean }>(
    `/api/movies/${movieId}/like`,
    { method: "POST" },
  );

export const getMovieWatchProgress = (movieId: number) =>
  apiRequest<MovieWatchProgressType>(`/api/movies/${movieId}/watch-progress`);

export const updateMovieWatchProgress = (
  movieId: number,
  progressSec: number,
  durationSec: number,
) =>
  apiRequest<{ watched: boolean }>(`/api/movies/${movieId}/watch-progress`, {
    method: "PUT",
    body: { progress_sec: progressSec, duration_sec: durationSec },
  });

export const deleteMovieWatchProgress = (movieId: number) =>
  apiRequest<{ cleared: boolean }>(`/api/movies/${movieId}/watch-progress`, {
    method: "DELETE",
  });

export const setMovieWatched = (movieId: number, watched: boolean) =>
  apiRequest<{ movie_id: number; watched: boolean }>(
    `/api/movies/${movieId}/watch-progress/watched`,
    {
      method: "PUT",
      body: { watched },
    },
  );

export const getMoviePlaylists = () =>
  apiRequest<MoviePlaylistsListResponseType>("/api/movies/playlists");

export const getMoviePlaylistDetails = (id: number) =>
  apiRequest<MoviePlaylistDetailResponseType>(`/api/movies/playlists/${id}`);

export const getMoviePlaylistMovies = (
  playlistId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    `/api/movies/playlists/${playlistId}/movies?page=${page}&per_page=${perPage}&sort=${sort}`,
  );

export const createMoviePlaylist = (data: CreateMoviePlaylistRequest) =>
  apiRequest<{ playlist: MoviePlaylistRowType }>("/api/movies/playlists", {
    method: "POST",
    body: data,
  });

export const updateMoviePlaylist = (
  id: number,
  data: UpdateMoviePlaylistRequest,
) =>
  apiRequest<{ playlist: MoviePlaylistRowType }>(`/api/movies/playlists/${id}`, {
    method: "PUT",
    body: data,
  });

export const deleteMoviePlaylist = (id: number) =>
  apiRequest<Record<string, never>>(`/api/movies/playlists/${id}`, {
    method: "DELETE",
  });

export const addMoviesToMoviePlaylist = (
  playlistId: number,
  movieIds: number[],
) =>
  apiRequest<{ added: number; skipped: number }>(
    `/api/movies/playlists/${playlistId}/movies`,
    {
      method: "POST",
      body: { movie_ids: movieIds },
    },
  );

export const removeMovieFromMoviePlaylist = (
  playlistId: number,
  movieId: number,
) =>
  apiRequest<Record<string, never>>(
    `/api/movies/playlists/${playlistId}/movies/${movieId}`,
    { method: "DELETE" },
  );

// Music API - Albums
export const getAlbumDetails = (id: number) =>
  apiRequest<AlbumDetailsResponseType>(`/api/music/albums/details/${id}`);

export const deleteAlbum = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/albums/${id}`, {
    method: "DELETE",
  });

export const getAlbumsPaginated = (page: number, perPage: number = 24) =>
  apiRequest<AlbumsListResponseType>(
    `/api/music/albums?page=${page}&per_page=${perPage}`,
  );

// ============================================================================
// Music API - Tracks
// ============================================================================

export const getTracksPaginated = (limit: number, offset: number) =>
  apiRequest<TracksListResponseType>(
    `/api/music/tracks?limit=${limit}&offset=${offset}`,
  );

export const getShuffleTracks = (limit: number = 50) =>
  apiRequest<ShuffleTracksResponseType>(
    `/api/music/tracks/shuffle?limit=${limit}`,
  );

export const toggleLikeTrack = (trackId: number) =>
  apiRequest<{ track_id: number; is_liked: boolean }>(
    `/api/music/tracks/${trackId}/like`,
    { method: "POST" },
  );

export const getLikedTracks = (page: number, perPage: number = 50) =>
  apiRequest<LikedTracksResponseType>(
    `/api/music/tracks/liked?page=${page}&per_page=${perPage}`,
  );

export const getLikedTrackIds = () =>
  apiRequest<{ liked_track_ids: number[] }>("/api/music/tracks/liked-ids");

// ============================================================================
// Music API - Musicians
// ============================================================================

export const getMusiciansPaginated = (page: number, perPage: number = 24) =>
  apiRequest<MusiciansListResponseType>(
    `/api/music/musicians?page=${page}&per_page=${perPage}`,
  );

export const getMusicianDetails = (id: number) =>
  apiRequest<MusicianDetailsResponseType>(`/api/music/musicians/${id}`);

// ============================================================================
// Music API - Stats
// ============================================================================

export const getMusicStats = () =>
  apiRequest<MusicStatsType>("/api/music/stats");

// ============================================================================
// Playlist API
// ============================================================================

export const getPlaylists = () =>
  apiRequest<PlaylistsListResponseType>("/api/music/playlists");

export const getPlaylistDetails = (id: number) =>
  apiRequest<PlaylistDetailResponseType>(`/api/music/playlists/${id}`);

export const getPlaylistTracks = (id: number, limit: number, offset: number) =>
  apiRequest<PlaylistTracksResponseType>(
    `/api/music/playlists/${id}/tracks?limit=${limit}&offset=${offset}`,
  );

export const createPlaylist = (data: CreatePlaylistRequest) =>
  apiRequest<{ playlist: PlaylistSummaryType }>("/api/music/playlists", {
    method: "POST",
    body: data,
  });

export const updatePlaylist = (
  id: number,
  data: UpdatePlaylistRequest,
) =>
  apiRequest<{ playlist: PlaylistSummaryType }>(`/api/music/playlists/${id}`, {
    method: "PUT",
    body: data,
  });

export const deletePlaylist = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/playlists/${id}`, {
    method: "DELETE",
  });

export const addTracksToPlaylist = (playlistId: number, trackIds: number[]) =>
  apiRequest<{ added: number; skipped: number }>(
    `/api/music/playlists/${playlistId}/tracks`,
    {
      method: "POST",
      body: { track_ids: trackIds },
    },
  );

export const removeTrackFromPlaylist = (playlistId: number, trackId: number) =>
  apiRequest<Record<string, never>>(
    `/api/music/playlists/${playlistId}/tracks/${trackId}`,
    { method: "DELETE" },
  );

export const reorderPlaylistTracks = (playlistId: number, trackIds: number[]) =>
  apiRequest<Record<string, never>>(
    `/api/music/playlists/${playlistId}/tracks/reorder`,
    {
      method: "PUT",
      body: { track_ids: trackIds },
    },
  );

// ============================================================================
// Music API - User Listening Stats
// ============================================================================

export const recordPlayEvent = (
  trackId: number,
  durationPlayed: number,
  completed: boolean,
) =>
  apiRequest<{ recorded: boolean }>("/api/music/user-stats/play", {
    method: "POST",
    body: {
      track_id: trackId,
      duration_played: durationPlayed,
      completed,
    },
  });

export const getUserListeningStats = () =>
  apiRequest<UserListeningStatsResponseType>("/api/music/user-stats/overview");

export const getUserTopTracks = (limit: number = 20, offset: number = 0) =>
  apiRequest<TopTracksResponseType>(
    `/api/music/user-stats/top-tracks?limit=${limit}&offset=${offset}`,
  );

export const getUserTopMusicians = (limit: number = 10, offset: number = 0) =>
  apiRequest<TopMusiciansResponseType>(
    `/api/music/user-stats/top-musicians?limit=${limit}&offset=${offset}`,
  );

export const getUserTopGenres = (limit: number = 10) =>
  apiRequest<TopGenresResponseType>(
    `/api/music/user-stats/top-genres?limit=${limit}`,
  );

export const getUserTopAlbums = (limit: number = 10, offset: number = 0) =>
  apiRequest<TopAlbumsResponseType>(
    `/api/music/user-stats/top-albums?limit=${limit}&offset=${offset}`,
  );

export const getUserRecentlyPlayed = (limit: number = 20, offset: number = 0) =>
  apiRequest<RecentlyPlayedResponseType>(
    `/api/music/user-stats/recently-played?limit=${limit}&offset=${offset}`,
  );

// ============================================================================
// Settings API
// ============================================================================

export const getSettings = () => apiRequest<SettingsType>("/api/settings");

export const getGeneralSettings = () =>
  apiRequest<GeneralSettingsResponseType>("/api/settings/general");

export const updateGeneralSettings = (data: UpdateGeneralSettingsRequest) =>
  apiRequest<UpdateGeneralSettingsResponseType>("/api/settings/general", {
    method: "PUT",
    body: data,
  });

export const updateLibrarySettings = (data: UpdateLibrarySettingsRequest) =>
  apiRequest<UpdateLibrarySettingsResponseType>("/api/settings/libraries", {
    method: "PUT",
    body: data,
  });

export const getPlaybackSettings = () =>
  apiRequest<PlaybackSettingsResponseType>("/api/settings/playback");

export const updatePlaybackSettings = (data: UpdatePlaybackSettingsRequest) =>
  apiRequest<UpdatePlaybackSettingsResponseType>("/api/settings/playback", {
    method: "PUT",
    body: data,
  });

export const triggerMusicScan = () =>
  apiRequest<{ message: string }>("/api/settings/scan/music", {
    method: "POST",
  });

export const triggerMovieScan = () =>
  apiRequest<{ message: string }>("/api/settings/scan/movies", {
    method: "POST",
  });

// Admin user management API
export const adminGetUsers = () =>
  apiRequest<{ users: AdminUserType[] }>("/api/admin/users");

export const adminCreateUser = (data: {
  name: string;
  email: string;
  password: string;
  is_admin: boolean;
}) =>
  apiRequest<{ user: AdminUserType }>("/api/admin/users", {
    method: "POST",
    body: data,
  });

export const adminUpdateUser = (
  id: number,
  data: { name: string; email: string; is_admin: boolean },
) =>
  apiRequest<{ user: AdminUserType }>(`/api/admin/users/${id}`, {
    method: "PATCH",
    body: data,
  });

export const adminDeleteUser = (id: number) =>
  apiRequest<Record<string, never>>(`/api/admin/users/${id}`, {
    method: "DELETE",
  });

export const adminResetUserPassword = (id: number, password: string) =>
  apiRequest<Record<string, never>>(`/api/admin/users/${id}/password`, {
    method: "PUT",
    body: { password },
  });

// ============================================================================
// Search API (FTS5-backed library search across movies, albums, musicians, tracks)
// ============================================================================

export const searchAll = (q: string) =>
  apiRequest<SearchAllResponseType>(
    `/api/search?q=${encodeURIComponent(q)}`,
  );

export const searchMovies = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchMoviesResponseType>(
    `/api/search/movies?q=${encodeURIComponent(q)}&page=${page}&per_page=${perPage}`,
  );

export const searchAlbums = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchAlbumsResponseType>(
    `/api/search/albums?q=${encodeURIComponent(q)}&page=${page}&per_page=${perPage}`,
  );

export const searchMusicians = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchMusiciansResponseType>(
    `/api/search/musicians?q=${encodeURIComponent(q)}&page=${page}&per_page=${perPage}`,
  );

export const searchTracks = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchTracksResponseType>(
    `/api/search/tracks?q=${encodeURIComponent(q)}&page=${page}&per_page=${perPage}`,
  );

// Watch rooms API
export const getWatchRoomInviteUsers = () =>
  apiRequest<WatchRoomInviteUsersResponseType>("/api/users");

export const createWatchRoom = (
  body: CreateWatchRoomRequestType,
) =>
  apiRequest<CreateWatchRoomResponseType>("/api/watch-rooms", {
    method: "POST",
    body,
  });

export const getWatchRoom = (id: number) =>
  apiRequest<WatchRoomResponseType>(`/api/watch-rooms/${id}`);

export const joinWatchRoom = (id: number) =>
  apiRequest<JoinWatchRoomResponseType>(
    `/api/watch-rooms/${id}/join`,
    { method: "POST" },
  );

export const getWatchRooms = () =>
  apiRequest<{ rooms: WatchRoomType[] }>("/api/watch-rooms");

export const deleteWatchRoom = (id: number) =>
  apiRequest(`/api/watch-rooms/${id}`, { method: "DELETE" });
