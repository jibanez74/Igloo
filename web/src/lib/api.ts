import type {
  AlbumDetailsResponseType,
  AlbumsListResponseType,
  ApiFailureType,
  ApiResponseType,
  MovieDetailsType,
  MusicianDetailsResponseType,
  MusicStatsType,
  MusiciansListResponseType,
  PlaylistDetailResponseType,
  PlaylistsListResponseType,
  PlaylistSummaryType,
  PlaylistTracksResponseType,
  ShuffleTracksResponseType,
  SimpleAlbumType,
  TheaterMovieType,
  TracksListResponseType,
} from "@/types";

// ============================================================================
// API Client - Generic request handler
// ============================================================================

const ERROR_NOTFOUND: ApiFailureType = {
  error: true,
  message: "404 - The resource you requested was not found.",
};

const NETWORK_ERROR: ApiFailureType = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
};

type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

type ApiRequestOptions = {
  method?: HttpMethod;
  body?: unknown;
};

/**
 * Generic API request handler that consolidates error handling and fetch configuration.
 * All API functions should use this to avoid repetition.
 */
async function apiRequest<T>(
  endpoint: string,
  options: ApiRequestOptions = {}
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

// ============================================================================
// Authentication API
// ============================================================================

export const login = (email: string, password: string) =>
  apiRequest("/api/auth/login", {
    method: "POST",
    body: { email, password },
  });

export const getAuthUser = () => apiRequest("/api/auth/user");

// ============================================================================
// Home Page API
// ============================================================================

export const getLatestAlbums = () =>
  apiRequest<{ albums: SimpleAlbumType[] }>("/api/music/albums/latest");

export const getMoviesInTheaters = () =>
  apiRequest<{ movies: TheaterMovieType[] }>("/api/tmdb/movies/in-theaters");

export const getMovieInTheaterDetails = (id: number) =>
  apiRequest<{ movie: MovieDetailsType }>(`/api/tmdb/movies/${id}`);

// ============================================================================
// Music API - Albums
// ============================================================================

export const getAlbumDetails = (id: number) =>
  apiRequest<AlbumDetailsResponseType>(`/api/music/albums/details/${id}`);

export const deleteAlbum = (id: number) =>
  apiRequest<Record<string, never>>(`/api/music/albums/${id}`, {
    method: "DELETE",
  });

export const getAlbumsPaginated = (page: number, perPage: number = 24) =>
  apiRequest<AlbumsListResponseType>(
    `/api/music/albums?page=${page}&per_page=${perPage}`
  );

// ============================================================================
// Music API - Tracks
// ============================================================================

export const getTracksPaginated = (limit: number, offset: number) =>
  apiRequest<TracksListResponseType>(
    `/api/music/tracks?limit=${limit}&offset=${offset}`
  );

export const getShuffleTracks = (limit: number = 50) =>
  apiRequest<ShuffleTracksResponseType>(
    `/api/music/tracks/shuffle?limit=${limit}`
  );

export const toggleLikeTrack = (trackId: number) =>
  apiRequest<{ track_id: number; is_liked: boolean }>(
    `/api/music/tracks/${trackId}/like`,
    { method: "POST" }
  );

export const getLikedTrackIds = () =>
  apiRequest<{ liked_track_ids: number[] }>("/api/music/tracks/liked");

// ============================================================================
// Music API - Musicians
// ============================================================================

export const getMusiciansPaginated = (page: number, perPage: number = 24) =>
  apiRequest<MusiciansListResponseType>(
    `/api/music/musicians?page=${page}&per_page=${perPage}`
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
    `/api/music/playlists/${id}/tracks?limit=${limit}&offset=${offset}`
  );

export const createPlaylist = (data: {
  name: string;
  description?: string;
  is_public?: boolean;
}) =>
  apiRequest<{ playlist: PlaylistSummaryType }>("/api/music/playlists", {
    method: "POST",
    body: data,
  });

export const updatePlaylist = (
  id: number,
  data: { name: string; description?: string; is_public?: boolean }
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
    }
  );

export const removeTrackFromPlaylist = (playlistId: number, trackId: number) =>
  apiRequest<Record<string, never>>(
    `/api/music/playlists/${playlistId}/tracks/${trackId}`,
    { method: "DELETE" }
  );

export const reorderPlaylistTracks = (playlistId: number, trackIds: number[]) =>
  apiRequest<Record<string, never>>(
    `/api/music/playlists/${playlistId}/tracks/reorder`,
    {
      method: "PUT",
      body: { track_ids: trackIds },
    }
  );
