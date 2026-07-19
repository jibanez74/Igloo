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
  DevicesListResponseType,
  QuickConnectLookupType,
  NotificationsListResponseType,
  UnreadNotificationCountResponseType,
  CreatePlaylistRequest,
  CreateWatchRoomRequestType,
  CreateWatchRoomResponseType,
  JoinWatchRoomResponseType,
  ContinueWatchingMovieType,
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
  LikedTracksResponseType,
  ShuffleTracksResponseType,
  SettingsType,
  SimpleAlbumType,
  TheaterMovieType,
  TmdbStatusType,
  TmdbSearchMoviesRequest,
  TracksListResponseType,
  UpdateMovieMetadataRequest,
  UpdateGeneralSettingsRequest,
  UpdateGeneralSettingsResponseType,
  UpdateLibrarySettingsRequest,
  UpdateLibrarySettingsResponseType,
  UpdatePlaybackSettingsRequest,
  UpdatePlaybackSettingsResponseType,
  UpdatePlaylistRequest,
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
  SpotifyTrackSearchRequest,
  SpotifyTrackSearchResultType,
  SpotifyStatusType,
} from "@/types";
import {
  ALBUMS_PER_PAGE,
  LIKED_TRACKS_PER_PAGE,
  MOVIES_PER_PAGE,
  MUSICIANS_PER_PAGE,
  SEARCH_PER_PAGE,
  SHUFFLE_TRACKS_LIMIT,
} from "@/lib/constants";

// ============================================================================
// Request handling (shared)
// ============================================================================

const ERROR_NOTFOUND: ApiFailureType = {
  error: true,
  message: "404 - The resource you requested was not found.",
  status: 404,
};

const NETWORK_ERROR: ApiFailureType = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
  status: 500,
};

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type ApiRequestOptions = {
  method?: HttpMethod;
  body?: unknown;
};

function withQuery(path: string, params: Record<string, string | number | boolean>) {
  const query = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    query.set(key, String(value));
  }

  return `${path}?${query}`;
}

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

    const data: unknown = await res.json();
    return data as ApiResponseType<T>;
  } catch (err) {
    if (!(err instanceof DOMException && err.name === "AbortError")) {
      console.warn(`apiRequest failed: ${endpoint}`, err);
    }
    return NETWORK_ERROR;
  }
}

// ============================================================================
// Auth
// ============================================================================

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

// ============================================================================
// Devices (Quick Connect)
// ============================================================================

export const lookupQuickConnect = (code: string) =>
  apiRequest<QuickConnectLookupType>("/api/quick-connect/lookup", {
    method: "POST",
    body: { code },
  });

export const approveQuickConnect = (code: string) =>
  apiRequest("/api/quick-connect/approve", {
    method: "POST",
    body: { code },
  });

export const getDevices = () =>
  apiRequest<DevicesListResponseType>("/api/devices");

export const renameDevice = (id: number, name: string) =>
  apiRequest(`/api/devices/${id}`, {
    method: "PATCH",
    body: { name },
  });

export const revokeDevice = (id: number) =>
  apiRequest(`/api/devices/${id}`, {
    method: "DELETE",
  });

// ============================================================================
// User settings
// ============================================================================

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

export const getUserPin = () =>
  apiRequest<{ pin: string | null }>("/api/user/pin");

export const updateUserPin = (pin: string, currentPin?: string) =>
  apiRequest<{ user: AuthUser }>("/api/user/pin", {
    method: "PUT",
    body:
      currentPin === undefined ? { pin } : { pin, current_pin: currentPin },
  });

export const updateUserAvatar = (avatar: string) =>
  apiRequest("/api/user/avatar", {
    method: "PUT",
    body: { avatar },
  });

// Bypasses apiRequest because it sends multipart FormData rather than JSON, but
// returns the same failure shape (NETWORK_ERROR) on a fetch rejection.
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

    const data: unknown = await response.json();
    return data as ApiResponseType<{ user: AuthUser }>;
  } catch {
    return NETWORK_ERROR;
  }
};

export const deleteUserAccount = () =>
  apiRequest("/api/user", {
    method: "DELETE",
  });

// ============================================================================
// Notifications
// ============================================================================

export const createNotification = (body: CreateNotificationRequest) =>
  apiRequest<CreateNotificationResponseType>("/api/notifications", {
    method: "POST",
    body,
  });

export const getNotifications = () =>
  apiRequest<NotificationsListResponseType>("/api/notifications");

export const getUnreadNotificationCount = () =>
  apiRequest<UnreadNotificationCountResponseType>(
    "/api/notifications/unread-count",
  );

export const markNotificationRead = (id: number) =>
  apiRequest(`/api/notifications/${id}/read`, { method: "POST" });

export const markAllNotificationsRead = () =>
  apiRequest("/api/notifications/read-all", { method: "POST" });

export const deleteNotification = (id: number) =>
  apiRequest(`/api/notifications/${id}`, { method: "DELETE" });

// ============================================================================
// Home / catalog + external metadata (TMDB / Spotify)
// ============================================================================

export const getLatestAlbums = () =>
  apiRequest<{ albums: SimpleAlbumType[] }>("/api/music/albums/latest");

export const getLatestMovies = () =>
  apiRequest<{ movies: LatestMovieType[] }>("/api/movies/latest");

export const getContinueWatchingMovies = () =>
  apiRequest<{ movies: ContinueWatchingMovieType[] }>(
    "/api/movies/continue-watching",
  );

export const getMoviesInTheaters = () =>
  apiRequest<{ movies: TheaterMovieType[] }>("/api/tmdb/movies/in-theaters");

export const getMovieInTheaterDetails = (id: number) =>
  apiRequest<{ movie: MovieDetailsType }>(`/api/tmdb/movies/${id}`);

export const getTmdbStatus = () =>
  apiRequest<TmdbStatusType>("/api/tmdb/status");

export const getSpotifyStatus = () =>
  apiRequest<SpotifyStatusType>("/api/spotify/status");

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

export const searchSpotifyTracks = (body: SpotifyTrackSearchRequest) =>
  apiRequest<{ results: SpotifyTrackSearchResultType[] }>(
    "/api/spotify/tracks/search",
    {
      method: "POST",
      body,
    },
  );

// ============================================================================
// Movies library (details, metadata editing, browsing, likes, watch progress)
// ============================================================================

export const getMovieDetails = (id: number) =>
  apiRequest<LibraryMovieDetailsResponse>(`/api/movies/details/${id}`);

export const getMovieTechnicalDetails = (id: number) =>
  apiRequest<MovieTechnicalDetailsResponse>(
    `/api/movies/${id}/technical-details`,
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

export const getMoviesLibrary = (
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    withQuery("/api/movies/library", {
      page,
      per_page: perPage,
      sort,
    }),
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
    withQuery(`/api/movies/genres/${genreId}/movies`, {
      page,
      per_page: perPage,
      sort,
    }),
  );

export const getMoviesStats = () =>
  apiRequest<MoviesStatsDataType>("/api/movies/stats");

export const getLikedMovies = (
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: "asc" | "desc" = "asc",
) =>
  apiRequest<MoviesLibraryPaginatedDataType>(
    withQuery("/api/movies/liked", {
      page,
      per_page: perPage,
      sort,
    }),
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

// ============================================================================
// Music (albums, tracks, musicians, stats, play events)
// ============================================================================

export const getAlbumDetails = (id: number) =>
  apiRequest<AlbumDetailsResponseType>(`/api/music/albums/details/${id}`);

export const deleteAlbum = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/albums/${id}`, {
    method: "DELETE",
  });

export const getAlbumsPaginated = (
  page: number,
  perPage: number = ALBUMS_PER_PAGE,
) =>
  apiRequest<AlbumsListResponseType>(
    withQuery("/api/music/albums", {
      page,
      per_page: perPage,
    }),
  );

export const getTracksPaginated = (limit: number, offset: number) =>
  apiRequest<TracksListResponseType>(
    withQuery("/api/music/tracks", { limit, offset }),
  );

export const getShuffleTracks = (limit: number = SHUFFLE_TRACKS_LIMIT) =>
  apiRequest<ShuffleTracksResponseType>(
    withQuery("/api/music/tracks/shuffle", { limit }),
  );

export const toggleLikeTrack = (trackId: number) =>
  apiRequest<{ track_id: number; is_liked: boolean }>(
    `/api/music/tracks/${trackId}/like`,
    { method: "POST" },
  );

export const getLikedTracks = (
  page: number,
  perPage: number = LIKED_TRACKS_PER_PAGE,
) =>
  apiRequest<LikedTracksResponseType>(
    withQuery("/api/music/tracks/liked", {
      page,
      per_page: perPage,
    }),
  );

export const getLikedTrackIds = () =>
  apiRequest<{ liked_track_ids: number[] }>("/api/music/tracks/liked-ids");

export const getMusiciansPaginated = (
  page: number,
  perPage: number = MUSICIANS_PER_PAGE,
) =>
  apiRequest<MusiciansListResponseType>(
    withQuery("/api/music/musicians", {
      page,
      per_page: perPage,
    }),
  );

export const getMusicianDetails = (id: number) =>
  apiRequest<MusicianDetailsResponseType>(`/api/music/musicians/${id}`);

export const getMusicStats = () =>
  apiRequest<MusicStatsType>("/api/music/stats");

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

// ============================================================================
// Playlists (music + movie)
// ============================================================================

export const getPlaylists = () =>
  apiRequest<PlaylistsListResponseType>("/api/music/playlists");

export const getPlaylistDetails = (id: number) =>
  apiRequest<PlaylistDetailResponseType>(`/api/music/playlists/${id}`);

export const getPlaylistTracks = (id: number, limit: number, offset: number) =>
  apiRequest<PlaylistTracksResponseType>(
    withQuery(`/api/music/playlists/${id}/tracks`, { limit, offset }),
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
    withQuery(`/api/movies/playlists/${playlistId}/movies`, {
      page,
      per_page: perPage,
      sort,
    }),
  );

export const createMoviePlaylist = (data: CreateMoviePlaylistRequest) =>
  apiRequest<{ playlist: MoviePlaylistRowType }>("/api/movies/playlists", {
    method: "POST",
    body: data,
  });

// ============================================================================
// Settings
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

// ============================================================================
// Admin user management
// ============================================================================

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
// Search (FTS5-backed library search across movies, albums, musicians, tracks)
// ============================================================================

export const searchAll = (q: string) =>
  apiRequest<SearchAllResponseType>(
    withQuery("/api/search", { q }),
  );

export const searchMovies = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchMoviesResponseType>(
    withQuery("/api/search/movies", {
      q,
      page,
      per_page: perPage,
    }),
  );

export const searchAlbums = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchAlbumsResponseType>(
    withQuery("/api/search/albums", {
      q,
      page,
      per_page: perPage,
    }),
  );

export const searchMusicians = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchMusiciansResponseType>(
    withQuery("/api/search/musicians", {
      q,
      page,
      per_page: perPage,
    }),
  );

export const searchTracks = (
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
) =>
  apiRequest<SearchTracksResponseType>(
    withQuery("/api/search/tracks", {
      q,
      page,
      per_page: perPage,
    }),
  );

// ============================================================================
// Watch rooms
// ============================================================================

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
