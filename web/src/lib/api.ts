import type { PersonData } from "@/types/people";
import type { MovieScanStatus, MusicScanStatus, ShowScanStatus } from "@/types/settings";
import { mediaApiBasePath, movieMediaRef } from "@/lib/media-ref";
import type {
  AddMoviesToPlaylistRequest,
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
  ContinueWatchingDataType,
  LatestMovieType,
  LatestShowsDataType,
  ShowsLibraryDataType,
  ShowsStatsDataType,
  ShowGenresDataType,
  ShowDetailsDataType,
  ShowSeasonEpisodesDataType,
  LibraryMovieDetailsResponse,
  GeneralSettingsResponseType,
  PlaybackSettingsResponseType,
  MovieDetailsType,
  MoviePlaylistDetailResponseType,
  MoviePlaylistRowType,
  MoviePlaylistsListResponseType,
  MovieTechnicalDetailsResponse,
  PlaybackMediaRef,
  ShowEpisodePlaybackDataType,
  ShowEpisodeTechnicalDetailsDataType,
  WatchProgressType,
  MovieGenreWithCountType,
  MoviesLibraryPaginatedDataType,
  MoviesStatsDataType,
  TmdbSearchResultType,
  MusicianDetailsResponseType,
  MusicStatsType,
  MusiciansListResponseType,
  PlaylistDetailResponseType,
  PlaylistsListResponseType,
  PlaylistMutationResponseType,
  PlaylistTracksResponseType,
  LikedTracksResponseType,
  ShuffleTracksResponseType,
  SettingsType,
  LatestAlbumsResponseType,
  LikedTrackIdsResponseType,
  RecordedPlayResponseType,
  TrackLikeToggleResponseType,
  BulkAddResponseType,
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
  UpdatePlaylistRequest,
  UpdateMoviePlaylistRequest,
  WatchRoomInviteUsersResponseType,
  WatchRoomResponseType,
  WatchRoomType,
  PagedSearchTab,
  SearchAllResponseType,
  SearchCategoryData,
  SpotifyAlbumSearchRequest,
  SpotifyAlbumSearchResponseType,
  SpotifyTrackSearchRequest,
  SpotifyTrackSearchResponseType,
  SpotifyStatusType,
  TrailerPreferencesData,
  UpdateTrailerPreferencesRequest,
  MoviePrerollData,
  SortOrder,
} from "@/types";
import {
  ALBUMS_PER_PAGE,
  LIKED_TRACKS_PER_PAGE,
  MOVIES_PER_PAGE,
  SHOWS_PER_PAGE,
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
  /**
   * Aborting resolves the usual network-failure envelope; it never throws.
   * Each query factory passes its query's signal through the GET helpers, so
   * a request nobody observes any more is aborted, and React Query, which has
   * already settled that query, discards the envelope.
   */
  signal?: AbortSignal;
  /**
   * The caller answers a 401 itself: the credentials the request carried
   * were wrong (login, the current password or PIN), or, for logout, the
   * session had already ended. Any other 401 means the server ended the
   * session, which onSessionExpired reports.
   */
  handlesUnauthorized?: boolean;
};

const sessionExpiredListeners = new Set<() => void>();

/**
 * Calls `listener` whenever the server answers a request with a 401 the
 * caller does not handle itself: the session has expired, was revoked, or
 * its user is gone. Returns the unsubscribe function.
 */
export function onSessionExpired(listener: () => void) {
  sessionExpiredListeners.add(listener);
  return () => {
    sessionExpiredListeners.delete(listener);
  };
}

function notifySessionExpired() {
  for (const listener of sessionExpiredListeners) {
    listener();
  }
}

function warnUnlessAborted(endpoint: string, err: unknown) {
  if (!(err instanceof DOMException && err.name === "AbortError")) {
    console.warn(`apiRequest failed: ${endpoint}`, err);
  }
}

function withQuery(path: string, params: Record<string, string | number | boolean>) {
  const query = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    query.set(key, String(value));
  }

  return `${path}?${query}`;
}

/** One page of a paged list: `page`/`per_page`, plus `sort` where the list takes one. */
function getPage<T extends Record<string, unknown>>(
  path: string,
  page: number,
  perPage: number,
  { sort, signal }: { sort?: SortOrder; signal?: AbortSignal } = {},
) {
  return apiRequest<T>(
    withQuery(path, {
      page,
      per_page: perPage,
      ...(sort === undefined ? {} : { sort }),
    }),
    { signal },
  );
}

/**
 * Generic API request handler that consolidates error handling and fetch configuration.
 * All API functions should use this to avoid repetition.
 */
async function apiRequest<T extends Record<string, unknown>>(
  endpoint: string,
  options: ApiRequestOptions = {},
): Promise<ApiResponseType<T>> {
  const { method = "GET", body, signal, handlesUnauthorized = false } = options;

  let res: Response;
  try {
    res = await fetch(endpoint, {
      method,
      credentials: "include",
      headers: body ? { "Content-Type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      signal,
    });
  } catch (err) {
    warnUnlessAborted(endpoint, err);
    return NETWORK_ERROR;
  }

  // Before the body is read, so a 401 a proxy answered without JSON still
  // counts, and outside the try, so a listener's own failure is not reported
  // as a network error.
  if (res.status === 401 && !handlesUnauthorized) {
    notifySessionExpired();
  }

  if (res.status === 404) {
    return ERROR_NOTFOUND;
  }

  try {
    const data = (await res.json()) as ApiResponseType<T>;
    // Stamp the status so a surface can word a failure by what happened
    // (401 on login, 403 on a room) while keeping the server's message as its
    // fallback.
    return data.error ? { ...data, status: res.status } : data;
  } catch (err) {
    warnUnlessAborted(endpoint, err);
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
    handlesUnauthorized: true,
  });

export const logout = () =>
  apiRequest("/api/auth/logout", {
    method: "DELETE",
    handlesUnauthorized: true,
  });

export const getAuthUser = (signal?: AbortSignal) =>
  apiRequest<{ user: AuthUser }>("/api/auth/user", { signal });

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

export const getDevices = (signal?: AbortSignal) =>
  apiRequest<DevicesListResponseType>("/api/devices", { signal });

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
  apiRequest<{ user: AuthUser }>("/api/user/name", {
    method: "PUT",
    body: { name },
  });

export const updateUserEmail = (email: string) =>
  apiRequest<{ user: AuthUser }>("/api/user/email", {
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
    handlesUnauthorized: true,
  });

export const getUserPin = (signal?: AbortSignal) =>
  apiRequest<{ pin: string | null }>("/api/user/pin", { signal });

export const updateUserPin = (pin: string, currentPin?: string) =>
  apiRequest<{ user: AuthUser }>("/api/user/pin", {
    method: "PUT",
    body:
      currentPin === undefined ? { pin } : { pin, current_pin: currentPin },
    handlesUnauthorized: true,
  });

export const updateUserAvatar = (avatar: string) =>
  apiRequest<{ user: AuthUser }>("/api/user/avatar", {
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

    if (response.status === 401) {
      notifySessionExpired();
    }

    if (response.status === 404) {
      return ERROR_NOTFOUND;
    }

    const uploadFailed: ApiFailureType = {
      error: true,
      message: `${response.status} - The avatar could not be uploaded.`,
      status: response.status,
    };

    // A malformed body — empty, non-JSON, or literal `null` — must not fall
    // through to the catch below, which would report the generic 500 instead
    // of the status the server actually sent.
    const data: unknown = await response.json().catch(() => null);

    if (typeof data !== "object" || data === null) {
      return response.ok ? NETWORK_ERROR : uploadFailed;
    }

    const body = data as ApiResponseType<{ user: AuthUser }>;

    // An HTTP failure whose body is not already an error envelope would
    // otherwise be reported to the caller as a successful upload.
    if (!response.ok && !body.error) {
      return uploadFailed;
    }

    return body;
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

export const createNotification = (body: CreateNotificationRequest): Promise<CreateNotificationResponseType | ApiFailureType> =>
  apiRequest("/api/notifications", {
    method: "POST",
    body,
  });

export const getNotifications = (signal?: AbortSignal) =>
  apiRequest<NotificationsListResponseType>("/api/notifications", { signal });

export const getUnreadNotificationCount = (signal?: AbortSignal) =>
  apiRequest<UnreadNotificationCountResponseType>(
    "/api/notifications/unread-count",
    { signal },
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

export const getLatestAlbums = (signal?: AbortSignal) =>
  apiRequest<LatestAlbumsResponseType>("/api/music/albums/latest", { signal });

export const getLatestMovies = (signal?: AbortSignal) =>
  apiRequest<{ movies: LatestMovieType[] }>("/api/movies/latest", { signal });

export const getLatestShows = (signal?: AbortSignal) =>
  apiRequest<LatestShowsDataType>("/api/shows/latest", { signal });

export const getShowDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<ShowDetailsDataType>(`/api/shows/details/${id}`, { signal });

export const getShowSeasonEpisodes = (
  showId: number,
  seasonNumber: number,
  signal?: AbortSignal,
) =>
  apiRequest<ShowSeasonEpisodesDataType>(
    `/api/shows/${showId}/seasons/${seasonNumber}/episodes`,
    { signal },
  );

export const getShowEpisode = (episodeId: number, signal?: AbortSignal) =>
  apiRequest<ShowEpisodePlaybackDataType>(
    `/api/shows/episodes/${episodeId}`,
    { signal },
  );

export const getShowEpisodeTechnicalDetails = (
  episodeId: number,
  signal?: AbortSignal,
) =>
  apiRequest<ShowEpisodeTechnicalDetailsDataType>(
    `/api/shows/episodes/${episodeId}/technical-details`,
    { signal },
  );

export const getShowsLibrary = (
  page: number,
  perPage: number = SHOWS_PER_PAGE,
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<ShowsLibraryDataType>(
    "/api/shows/library",
    page,
    perPage,
    { sort, signal },
  );

export const getShowGenresWithCounts = (signal?: AbortSignal) =>
  apiRequest<ShowGenresDataType>("/api/shows/genres", { signal });

export const getShowsByGenre = (
  genreId: number,
  page: number,
  perPage: number = SHOWS_PER_PAGE,
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<ShowsLibraryDataType>(
    `/api/shows/genres/${genreId}/shows`,
    page,
    perPage,
    { sort, signal },
  );

export const getShowsStats = (signal?: AbortSignal) =>
  apiRequest<ShowsStatsDataType>("/api/shows/stats", { signal });

export const getContinueWatching = (signal?: AbortSignal) =>
  apiRequest<ContinueWatchingDataType>("/api/continue-watching", { signal });

export const getMoviesInTheaters = (signal?: AbortSignal) =>
  apiRequest<{ movies: TheaterMovieType[] }>(
    "/api/tmdb/movies/in-theaters",
    { signal },
  );

export const getMovieInTheaterDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<{ movie: MovieDetailsType }>(`/api/tmdb/movies/${id}`, { signal });

export const getTmdbStatus = (signal?: AbortSignal) =>
  apiRequest<TmdbStatusType>("/api/tmdb/status", { signal });

export const getSpotifyStatus = (signal?: AbortSignal) =>
  apiRequest<SpotifyStatusType>("/api/spotify/status", { signal });

export const searchTmdbMovies = (body: TmdbSearchMoviesRequest) =>
  apiRequest<{ results: TmdbSearchResultType[] }>("/api/tmdb/movies/search", {
    method: "POST",
    body,
  });

export const searchSpotifyAlbums = (body: SpotifyAlbumSearchRequest) =>
  apiRequest<SpotifyAlbumSearchResponseType>(
    "/api/spotify/albums/search",
    {
      method: "POST",
      body,
    },
  );

export const searchSpotifyTracks = (body: SpotifyTrackSearchRequest) =>
  apiRequest<SpotifyTrackSearchResponseType>(
    "/api/spotify/tracks/search",
    {
      method: "POST",
      body,
    },
  );

// ============================================================================
// Movies library (details, metadata editing, browsing, likes, watch progress)
// ============================================================================

export const getMovieDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<LibraryMovieDetailsResponse>(
    `/api/movies/details/${id}`,
    { signal },
  );

export const getMovieTechnicalDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<MovieTechnicalDetailsResponse>(
    `/api/movies/${id}/technical-details`,
    { signal },
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
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<MoviesLibraryPaginatedDataType>(
    "/api/movies/library",
    page,
    perPage,
    { sort, signal },
  );

export const getMovieGenresWithCounts = (signal?: AbortSignal) =>
  apiRequest<{ genres: MovieGenreWithCountType[] }>(
    "/api/movies/genres",
    { signal },
  );

export const getMoviesByGenre = (
  genreId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<MoviesLibraryPaginatedDataType>(
    `/api/movies/genres/${genreId}/movies`,
    page,
    perPage,
    { sort, signal },
  );

export const getMoviesStats = (signal?: AbortSignal) =>
  apiRequest<MoviesStatsDataType>("/api/movies/stats", { signal });

export const getLikedMovies = (
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<MoviesLibraryPaginatedDataType>(
    "/api/movies/liked",
    page,
    perPage,
    { sort, signal },
  );

export const getMovieLikeStatus = (movieId: number, signal?: AbortSignal) =>
  apiRequest<{ is_liked: boolean }>(
    `/api/movies/${movieId}/like-status`,
    { signal },
  );

export const toggleLikeMovie = (movieId: number) =>
  apiRequest<{ movie_id: number; is_liked: boolean }>(
    `/api/movies/${movieId}/like`,
    { method: "POST" },
  );

// Watch progress is one contract for movies and TV episodes; only the route
// prefix differs (see mediaApiBasePath). The movie-named wrappers below keep
// the movie call sites readable.

export const getMediaWatchProgress = (
  media: PlaybackMediaRef,
  signal?: AbortSignal,
) =>
  apiRequest<WatchProgressType>(
    `${mediaApiBasePath(media)}/watch-progress`,
    { signal },
  );

export const updateMediaWatchProgress = (
  media: PlaybackMediaRef,
  progressSec: number,
  durationSec: number,
  saveSessionId: string,
  saveSequence: number,
) =>
  apiRequest<{ watched: boolean }>(
    `${mediaApiBasePath(media)}/watch-progress`,
    {
      method: "PUT",
      body: {
        progress_sec: progressSec,
        duration_sec: durationSec,
        save_session_id: saveSessionId,
        save_sequence: saveSequence,
      },
    },
  );

export const deleteMediaWatchProgress = (media: PlaybackMediaRef) =>
  apiRequest<{ cleared: boolean }>(
    `${mediaApiBasePath(media)}/watch-progress`,
    { method: "DELETE" },
  );

export const setMediaWatched = (media: PlaybackMediaRef, watched: boolean) =>
  apiRequest<{ watched: boolean }>(
    `${mediaApiBasePath(media)}/watch-progress/watched`,
    {
      method: "PUT",
      body: { watched },
    },
  );

export const setMovieWatched = (movieId: number, watched: boolean) =>
  setMediaWatched(movieMediaRef(movieId), watched);

// ============================================================================
// Music (albums, tracks, musicians, stats, play events)
// ============================================================================

export const getAlbumDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<AlbumDetailsResponseType>(
    `/api/music/albums/details/${id}`,
    { signal },
  );

export const deleteAlbum = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/albums/${id}`, {
    method: "DELETE",
  });

export const getAlbumsPaginated = (
  page: number,
  perPage: number = ALBUMS_PER_PAGE,
  signal?: AbortSignal,
) =>
  getPage<AlbumsListResponseType>(
    "/api/music/albums",
    page,
    perPage,
    { signal },
  );

export const getTracksPaginated = (
  limit: number,
  offset: number,
  signal?: AbortSignal,
) =>
  apiRequest<TracksListResponseType>(
    withQuery("/api/music/tracks", { limit, offset }),
    { signal },
  );

// excludeTrackIds tells the server what the caller already holds so the random
// sample is drawn around it. Sent comma-separated rather than as repeated
// params - the server accepts either and one entry keeps the URL short.
export const getShuffleTracks = (
  limit: number = SHUFFLE_TRACKS_LIMIT,
  excludeTrackIds: number[] = [],
) =>
  apiRequest<ShuffleTracksResponseType>(
    withQuery("/api/music/tracks/shuffle", {
      limit,
      ...(excludeTrackIds.length > 0
        ? { exclude: excludeTrackIds.join(",") }
        : {}),
    }),
  );

export const toggleLikeTrack = (trackId: number) =>
  apiRequest<TrackLikeToggleResponseType>(
    `/api/music/tracks/${trackId}/like`,
    { method: "POST" },
  );

export const getLikedTracks = (
  page: number,
  perPage: number = LIKED_TRACKS_PER_PAGE,
  signal?: AbortSignal,
) =>
  getPage<LikedTracksResponseType>(
    "/api/music/tracks/liked",
    page,
    perPage,
    { signal },
  );

export const getLikedTrackIds = (signal?: AbortSignal) =>
  apiRequest<LikedTrackIdsResponseType>(
    "/api/music/tracks/liked-ids",
    { signal },
  );

export const getMusiciansPaginated = (
  page: number,
  perPage: number = MUSICIANS_PER_PAGE,
  signal?: AbortSignal,
) =>
  getPage<MusiciansListResponseType>(
    "/api/music/musicians",
    page,
    perPage,
    { signal },
  );

export const getMusicianDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<MusicianDetailsResponseType>(
    `/api/music/musicians/${id}`,
    { signal },
  );

export const getMusicStats = (signal?: AbortSignal) =>
  apiRequest<MusicStatsType>("/api/music/stats", { signal });

export const recordPlayEvent = (
  trackId: number,
  durationPlayed: number,
  completed: boolean,
) =>
  apiRequest<RecordedPlayResponseType>("/api/music/user-stats/play", {
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

export const getPlaylists = (signal?: AbortSignal) =>
  apiRequest<PlaylistsListResponseType>("/api/music/playlists", { signal });

export const getPlaylistDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<PlaylistDetailResponseType>(
    `/api/music/playlists/${id}`,
    { signal },
  );

export const getPlaylistTracks = (
  id: number,
  limit: number,
  offset: number,
  signal?: AbortSignal,
) =>
  apiRequest<PlaylistTracksResponseType>(
    withQuery(`/api/music/playlists/${id}/tracks`, { limit, offset }),
    { signal },
  );

export const createPlaylist = (data: CreatePlaylistRequest) =>
  apiRequest<PlaylistMutationResponseType>("/api/music/playlists", {
    method: "POST",
    body: data,
  });

export const updatePlaylist = (
  id: number,
  data: UpdatePlaylistRequest,
) =>
  apiRequest<PlaylistMutationResponseType>(`/api/music/playlists/${id}`, {
    method: "PUT",
    body: data,
  });

export const deletePlaylist = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/playlists/${id}`, {
    method: "DELETE",
  });

export const addTracksToPlaylist = (playlistId: number, trackIds: number[]) =>
  apiRequest<BulkAddResponseType>(
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

export const getMoviePlaylists = (signal?: AbortSignal) =>
  apiRequest<MoviePlaylistsListResponseType>(
    "/api/movies/playlists",
    { signal },
  );

export const getMoviePlaylistDetails = (id: number, signal?: AbortSignal) =>
  apiRequest<MoviePlaylistDetailResponseType>(
    `/api/movies/playlists/${id}`,
    { signal },
  );

export const getMoviePlaylistMovies = (
  playlistId: number,
  page: number,
  perPage: number = MOVIES_PER_PAGE,
  sort: SortOrder = "asc",
  signal?: AbortSignal,
) =>
  getPage<MoviesLibraryPaginatedDataType>(
    `/api/movies/playlists/${playlistId}/movies`,
    page,
    perPage,
    { sort, signal },
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
  apiRequest<{ playlist: MoviePlaylistRowType }>(
    `/api/movies/playlists/${id}`,
    { method: "PUT", body: data },
  );

export const deleteMoviePlaylist = (id: number) =>
  apiRequest<Record<string, never>>(`/api/movies/playlists/${id}`, {
    method: "DELETE",
  });

export const addMoviesToMoviePlaylist = (
  playlistId: number,
  movieIds: number[],
) =>
  apiRequest<BulkAddResponseType>(
    `/api/movies/playlists/${playlistId}/movies`,
    {
      method: "POST",
      body: { movie_ids: movieIds } satisfies AddMoviesToPlaylistRequest,
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

// ============================================================================
// Settings
// ============================================================================

export const getSettings = (signal?: AbortSignal) =>
  apiRequest<SettingsType>("/api/settings", { signal });

export const getGeneralSettings = (signal?: AbortSignal) =>
  apiRequest<GeneralSettingsResponseType>("/api/settings/general", { signal });

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

export const getPlaybackSettings = (signal?: AbortSignal) =>
  apiRequest<PlaybackSettingsResponseType>(
    "/api/settings/playback",
    { signal },
  );

export const updatePlaybackSettings = (data: UpdatePlaybackSettingsRequest) =>
  apiRequest<PlaybackSettingsResponseType>("/api/settings/playback", {
    method: "PUT",
    body: data,
  });

// ============================================================================
// Trailer pre-roll
// ============================================================================

export const getTrailerPreferences = (signal?: AbortSignal) =>
  apiRequest<TrailerPreferencesData>(
    "/api/user/preferences/trailers",
    { signal },
  );

export const updateTrailerPreferences = (data: UpdateTrailerPreferencesRequest) =>
  apiRequest<TrailerPreferencesData>("/api/user/preferences/trailers", {
    method: "PUT",
    body: data,
  });

export const getMoviePreroll = (movieId: number, signal?: AbortSignal) =>
  apiRequest<MoviePrerollData>(`/api/movies/${movieId}/preroll`, { signal });

export const triggerMusicScan = () =>
  apiRequest<{ message: string }>("/api/settings/scan/music", {
    method: "POST",
  });

export const getMusicScanStatus = (signal?: AbortSignal) =>
  apiRequest<MusicScanStatus>("/api/settings/scan/music", { signal });

export const getMovieScanStatus = (signal?: AbortSignal) =>
  apiRequest<MovieScanStatus>("/api/settings/scan/movies", { signal });

export const triggerMovieScan = () =>
  apiRequest<{ message: string }>("/api/settings/scan/movies", {
    method: "POST",
  });

export const triggerShowScan = () =>
  apiRequest<{ message: string }>("/api/settings/scan/shows", {
    method: "POST",
  });

export const getShowScanStatus = (signal?: AbortSignal) =>
  apiRequest<ShowScanStatus>("/api/settings/scan/shows", { signal });

// ============================================================================
// Admin user management
// ============================================================================

export const adminGetUsers = (signal?: AbortSignal) =>
  apiRequest<{ users: AdminUserType[] }>("/api/admin/users", { signal });

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
// Search (FTS5-backed library search across movies, shows, albums, musicians,
// tracks)
// ============================================================================

export const searchAll = (q: string, signal?: AbortSignal) =>
  apiRequest<SearchAllResponseType>(
    withQuery("/api/search", { q }),
    { signal },
  );

export const searchCategory = <K extends PagedSearchTab>(
  kind: K,
  q: string,
  page: number,
  perPage: number = SEARCH_PER_PAGE,
  signal?: AbortSignal,
) =>
  apiRequest<SearchCategoryData[K]>(
    withQuery(`/api/search/${kind}`, {
      q,
      page,
      per_page: perPage,
    }),
    { signal },
  );

// ============================================================================
// Watch rooms
// ============================================================================

export const getWatchRoomInviteUsers = (signal?: AbortSignal) =>
  apiRequest<WatchRoomInviteUsersResponseType>("/api/users", { signal });

export const createWatchRoom = (
  body: CreateWatchRoomRequestType,
) =>
  apiRequest<CreateWatchRoomResponseType>("/api/watch-rooms", {
    method: "POST",
    body,
  });

export const getWatchRoom = (id: number, signal?: AbortSignal) =>
  apiRequest<WatchRoomResponseType>(`/api/watch-rooms/${id}`, { signal });

export const joinWatchRoom = (id: number) =>
  apiRequest<JoinWatchRoomResponseType>(
    `/api/watch-rooms/${id}/join`,
    { method: "POST" },
  );

export const getWatchRooms = (signal?: AbortSignal) =>
  apiRequest<{ rooms: WatchRoomType[] }>("/api/watch-rooms", { signal });

export const deleteWatchRoom = (id: number) =>
  apiRequest(`/api/watch-rooms/${id}`, { method: "DELETE" });

export const getPerson = (id: number, signal?: AbortSignal) =>
  apiRequest<PersonData>(`/api/people/${id}`, { signal });
