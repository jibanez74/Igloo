// keys for queries
export const AUTH_USER_KEY = "auth-user";
export const MOVIES_IN_THEATERS_KEY = "movies-in-theaters";
export const LATEST_MOVIES_KEY = "latest-movies";
export const LATEST_SHOWS_KEY = "latest-shows";
export const LATEST_ALBUMS_KEY = "latest-albums";
export const MOVIES_KEY = "movies";
export const MOVIE_DETAILS_KEY = "movie-details";
export const LIBRARY_MOVIE_DETAILS_KEY = "library-movie-details";
export const MOVIE_TECHNICAL_DETAILS_KEY = "movie-technical-details";
export const ALBUMS_KEY = "albums";
export const ALBUM_DETAILS_KEY = "album-details";
export const MUSICIANS_KEY = "musicians";
export const MUSICIAN_DETAILS_KEY = "musician-details";
export const TRACKS_KEY = "tracks";
export const TRACKS_INFINITE_KEY = "tracks-infinite";
export const ALBUMS_PAGINATED_KEY = "albums-paginated";
export const MUSICIANS_PAGINATED_KEY = "musicians-paginated";
export const MUSIC_STATS_KEY = "music-stats";
export const SETTINGS_KEY = "settings";

// tmdb (paths only from API; frontend builds full URLs inline)
export const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p";
export const TMDB_BACKDROP_SIZE = "w1280";
export const TMDB_POSTER_SIZE = "w500";
export const TMDB_PROFILE_SIZE = "w185";
export const TMDB_LOGO_SIZE = "w92";

// pagination for music page
export const ALBUMS_PER_PAGE = 24;
export const MUSICIANS_PER_PAGE = 24;
export const PLAYLISTS_PER_PAGE = 24;

// virtual list item heights (in pixels)
export const VIRTUAL_LIST_LETTER_HEIGHT = 52;
export const VIRTUAL_LIST_TRACK_HEIGHT = 60;

// playlist query keys
export const PLAYLISTS_KEY = "playlists";
export const PLAYLIST_DETAILS_KEY = "playlist-details";
export const PLAYLIST_TRACKS_KEY = "playlist-tracks";

// playback — single source of truth for stream modes (ids + labels + metadata)
export const STREAM_MODES = [
  {
    id: "direct",
    label: "Original file — plays as-is",
    type: "direct",
    maxHeight: 0,
  },
  {
    id: "remux",
    label: "Original video, adjusted audio",
    type: "remux",
    maxHeight: 0,
  },
  {
    id: "2160p_16mbps",
    label: "4K — highest quality",
    type: "transcode",
    maxHeight: 2160,
  },
  {
    id: "1080p_8mbps",
    label: "1080p — best quality",
    type: "transcode",
    maxHeight: 1080,
  },
  {
    id: "1080p_6mbps",
    label: "1080p — high quality",
    type: "transcode",
    maxHeight: 1080,
  },
  {
    id: "1080p_4mbps",
    label: "1080p — balanced",
    type: "transcode",
    maxHeight: 1080,
  },
  {
    id: "720p_3mbps",
    label: "720p — lower bandwidth",
    type: "transcode",
    maxHeight: 720,
  },
] as const;

export type StreamModeId = (typeof STREAM_MODES)[number]["id"];

/** Derived from `STREAM_MODES` for Zod `z.enum`. */
export const STREAM_MODE_IDS = STREAM_MODES.map(m => m.id) as unknown as readonly [
  StreamModeId,
  ...StreamModeId[],
];

/** Key crew: max writing-department rows before "Show all crew" */
export const MOVIE_DETAILS_KEY_CREW_WRITERS_CAP = 3;

/**
 * tw-animate-css enter for library movie details (same family as MovieCard / AlbumCard).
 * Stagger with `delay-*` + `motion-reduce:delay-0` on wrappers.
 */
export const MOVIE_DETAILS_CONTENT_ENTER_CLASS =
  "animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-300 ease-out motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:translate-y-0";
