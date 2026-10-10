import {
  addMoviesToMoviePlaylist,
  addTracksToPlaylist,
  createMoviePlaylist,
  createPlaylist,
  deleteMoviePlaylist,
  deletePlaylist,
  updateMoviePlaylist,
  updatePlaylist,
} from "@/lib/api";
import {
  LIBRARY_NOUNS,
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  MOVIES_PLAYLISTS_TAB_SEARCH,
  MUSIC_PLAYLISTS_TAB_SEARCH,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
} from "@/lib/constants";
import type { ApiResponseType, BulkAddResponseType } from "@/types";

export type PlaylistKind = "music" | "movie";

/** One row of the AddToPlaylistDialog picker, the same for either kind. */
export type PlaylistPickerRow = {
  id: number;
  name: string;
  can_edit: boolean;
  item_count: number;
};

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
  /** Appends items to one playlist; the server counts duplicates as skipped. */
  addItems: (
    playlistId: number,
    itemIds: number[],
  ) => Promise<ApiResponseType<BulkAddResponseType>>;
  /** Query key of the library's playlists list. */
  listKey: string;
  /** Query key prefix of one playlist's details; the id follows it. */
  detailsKey: string;
  /** Query key prefix of one playlist's items; the id follows it. */
  itemsKey: string;
  /** What the playlist holds, for counts and toasts. */
  itemNoun: { singular: string; plural: string };
  /** The playlist page, reloaded after a rename so its head follows. */
  routeId: string;
  /** Where Delete lands: that library's Playlists tab. */
  index: { to: "/music"; search: typeof MUSIC_PLAYLISTS_TAB_SEARCH }
    | { to: "/movies"; search: typeof MOVIES_PLAYLISTS_TAB_SEARCH };
  createDescription: string;
};

// What differs between a music and a movie playlist for the shared
// PlaylistFormDialog, PlaylistOwnerActions and AddToPlaylistDialog: the client
// calls, the caches they touch and where the page lives.
export const PLAYLIST_KINDS: Record<PlaylistKind, PlaylistKindConfig> = {
  music: {
    create: createPlaylist,
    update: updatePlaylist,
    remove: deletePlaylist,
    addItems: addTracksToPlaylist,
    listKey: PLAYLISTS_KEY,
    detailsKey: PLAYLIST_DETAILS_KEY,
    itemsKey: PLAYLIST_TRACKS_KEY,
    itemNoun: LIBRARY_NOUNS.track,
    routeId: "/_auth/music/playlist/$id",
    index: { to: "/music", search: MUSIC_PLAYLISTS_TAB_SEARCH },
    createDescription:
      "Create a new playlist to organize your favorite tracks.",
  },
  movie: {
    create: createMoviePlaylist,
    update: updateMoviePlaylist,
    remove: deleteMoviePlaylist,
    addItems: addMoviesToMoviePlaylist,
    listKey: MOVIE_PLAYLISTS_KEY,
    detailsKey: MOVIE_PLAYLIST_DETAILS_KEY,
    itemsKey: MOVIE_PLAYLIST_MOVIES_KEY,
    itemNoun: LIBRARY_NOUNS.movie,
    routeId: "/_auth/movies/playlist/$id",
    index: { to: "/movies", search: MOVIES_PLAYLISTS_TAB_SEARCH },
    createDescription:
      "Create a playlist for movies. Track playlists stay on the Music page.",
  },
};
