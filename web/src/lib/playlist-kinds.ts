import {
  createMoviePlaylist,
  createPlaylist,
  deleteMoviePlaylist,
  deletePlaylist,
  updateMoviePlaylist,
  updatePlaylist,
} from "@/lib/api";
import {
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLISTS_KEY,
  MOVIES_PLAYLISTS_TAB_SEARCH,
  MUSIC_PLAYLISTS_TAB_SEARCH,
  PLAYLIST_DETAILS_KEY,
  PLAYLISTS_KEY,
} from "@/lib/constants";
import type { ApiResponseType } from "@/types";

export type PlaylistKind = "music" | "movie";

// The fields both playlist kinds' create and update bodies share. The movie
// functions accept supersets (an optional movie_id), so they assign here.
type PlaylistCreateBody = {
  name: string;
  description?: string;
  is_public?: boolean;
};

type PlaylistUpdateBody = PlaylistCreateBody & {
  cover_image?: string;
};

type PlaylistMutationResult = Promise<ApiResponseType<Record<string, unknown>>>;

type PlaylistKindConfig = {
  create: (data: PlaylistCreateBody) => PlaylistMutationResult;
  update: (id: number, data: PlaylistUpdateBody) => PlaylistMutationResult;
  remove: (id: number) => PlaylistMutationResult;
  /** Query key of the library's playlists list. */
  listKey: string;
  /** Query key prefix of one playlist's details; the id follows it. */
  detailsKey: string;
  /** The playlist page, reloaded after a rename so its head follows. */
  routeId: string;
  /** Where Delete lands: that library's Playlists tab. */
  index: { to: "/music"; search: typeof MUSIC_PLAYLISTS_TAB_SEARCH }
    | { to: "/movies"; search: typeof MOVIES_PLAYLISTS_TAB_SEARCH };
  createDescription: string;
};

// What differs between a music and a movie playlist for the shared
// PlaylistFormDialog and PlaylistOwnerActions: the client calls, the caches
// they touch and where the page lives.
export const PLAYLIST_KINDS: Record<PlaylistKind, PlaylistKindConfig> = {
  music: {
    create: createPlaylist,
    update: updatePlaylist,
    remove: deletePlaylist,
    listKey: PLAYLISTS_KEY,
    detailsKey: PLAYLIST_DETAILS_KEY,
    routeId: "/_auth/music/playlist/$id",
    index: { to: "/music", search: MUSIC_PLAYLISTS_TAB_SEARCH },
    createDescription:
      "Create a new playlist to organize your favorite tracks.",
  },
  movie: {
    create: createMoviePlaylist,
    update: updateMoviePlaylist,
    remove: deleteMoviePlaylist,
    listKey: MOVIE_PLAYLISTS_KEY,
    detailsKey: MOVIE_PLAYLIST_DETAILS_KEY,
    routeId: "/_auth/movies/playlist/$id",
    index: { to: "/movies", search: MOVIES_PLAYLISTS_TAB_SEARCH },
    createDescription:
      "Create a playlist for movies. Track playlists stay on the Music page.",
  },
};
