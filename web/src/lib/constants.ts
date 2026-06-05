// keys for queries
export const AUTH_USER_KEY = "auth-user";
export const WATCH_ROOMS_KEY = "watch-rooms";
export const WATCH_ROOM_KEY = "watch-room";
export const WATCH_ROOM_INVITE_USERS_KEY = "watch-room-invite-users";
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
export const GENERAL_SETTINGS_KEY = "general-settings";
export const PLAYBACK_SETTINGS_KEY = "playback-settings";
export const ADMIN_USERS_KEY = "admin-users";
export const SEARCH_ALL_KEY = "search-all";
export const SEARCH_MOVIES_KEY = "search-movies";
export const SEARCH_ALBUMS_KEY = "search-albums";
export const SEARCH_MUSICIANS_KEY = "search-musicians";
export const SEARCH_TRACKS_KEY = "search-tracks";

/** Default page size for paginated /api/search/<entity> endpoints (matches server SEARCH_DEFAULT_PER_PAGE). */
export const SEARCH_PER_PAGE = 24;

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

/** Default page size for GET /api/movies/library (matches server MOVIES_LIBRARY_DEFAULT_PER_PAGE). */
export const MOVIES_PER_PAGE = 24;

/** Default search params when navigating to /movies from the sidebar (fresh entry). */
export const MOVIES_INDEX_DEFAULT_SEARCH = {
  tab: "all" as const,
  allPage: 1,
  sort: "asc" as const,
  genresPage: 1,
  playlistsPage: 1,
};

/** Search when opening /movies on the Playlists tab (e.g. back link from a movie playlist). */
export const MOVIES_PLAYLISTS_TAB_SEARCH = {
  ...MOVIES_INDEX_DEFAULT_SEARCH,
  tab: "playlists" as const,
};

// movies library page — query keys (TanStack Query)
export const MOVIES_LIBRARY_KEY = "movies-library";
export const MOVIES_GENRES_KEY = "movies-genres";
export const MOVIES_BY_GENRE_KEY = "movies-by-genre";
export const MOVIES_STATS_KEY = "movies-stats";
export const MOVIES_LIKED_KEY = "movies-liked";
export const MOVIE_LIKE_STATUS_KEY = "movie-like-status";
export const MOVIE_WATCH_PROGRESS_KEY = "movie-watch-progress";
export const MOVIE_PLAYLISTS_KEY = "movie-playlists";
export const MOVIE_PLAYLIST_DETAILS_KEY = "movie-playlist-details";
export const MOVIE_PLAYLIST_MOVIES_KEY = "movie-playlist-movies";

// virtual list item heights (in pixels)
export const VIRTUAL_LIST_LETTER_HEIGHT = 52;
export const VIRTUAL_LIST_TRACK_HEIGHT = 60;

// playlist query keys
export const PLAYLISTS_KEY = "playlists";
export const PLAYLIST_DETAILS_KEY = "playlist-details";
export const PLAYLIST_TRACKS_KEY = "playlist-tracks";
export const LIKED_TRACKS_KEY = "liked-tracks";
export const LIKED_TRACK_IDS_KEY = "liked-track-ids";

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

/** Derived from `STREAM_MODES` for Zod `z.enum`. */
export const STREAM_MODE_IDS = [
  "direct",
  "remux",
  "2160p_16mbps",
  "1080p_8mbps",
  "1080p_6mbps",
  "1080p_4mbps",
  "720p_3mbps",
] as const;

export const HLS_PLAYBACK_SESSION_QUERY_PARAM = "playback_session";
export const MOVIE_SEEK_STEP_SEC = 10;
export const MOVIE_VOLUME_STEP = 0.1;
export const MOVIE_CONTROLS_IDLE_MS = 3000;
export const MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS = 15_000;
export const MOVIE_WATCH_PROGRESS_MIN_SECONDS = 180;
export const MOVIE_WATCH_PROGRESS_COMPLETION_THRESHOLD = 0.98;
export const MOVIE_HLS_FORWARD_REBASE_THRESHOLD_SEC = 120;
export const MEDIA_ERR_NETWORK = 2;
export const MEDIA_ERR_DECODE = 3;
export const MEDIA_ERR_SRC_NOT_SUPPORTED = 4;

/** hls.js: max `onSessionLost` recoveries per logical stream (see `hlsStreamRecoveryKey`). */
export const HLS_SESSION_LOST_MAX_ATTEMPTS = 3;
/** Min ms between `onSessionLost` calls to avoid tight loops when `src` updates re-triggers 404. */
export const HLS_SESSION_LOST_MIN_INTERVAL_MS = 2000;

/** hls.js: manifest / level / fragment request timeout (ms). */
export const HLS_JS_LOAD_TIMEOUT_MS = 120_000;
/** Resume HLS sessions this far before the target so short rewinds work without rebasing. */
export const HLS_RESUME_REWIND_BUFFER_SEC = 10;
/** hls.js: seconds of already-played buffer to keep behind the playhead. */
export const HLS_JS_BACK_BUFFER_LENGTH_SEC = 120;

// playback settings dialog (movie play UI)
export const PLAYBACK_SETTINGS_SUMMARY_LOADING = "Loading playback options…";
/** Subtitle `<select>` / Radix value when subtitles are off. */
export const SUBTITLE_TRACK_SELECT_OFF_VALUE = "off";
/** Single audio stream placeholder option value/index. */
export const AUDIO_TRACK_SELECT_DEFAULT_VALUE = "0";
export const AUDIO_TRACK_DEFAULT_LABEL = "Default";
export const SUBTITLES_NONE_LABEL = "None";
/**
 * Radix Select content uses `data-slot="select-content"` in components/ui/select.tsx.
 * Used to avoid closing a dialog when interacting with a portaled select.
 */
export const SELECT_CONTENT_SLOT_SELECTOR = "[data-slot='select-content']";
export const PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS =
  "w-full rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white outline-none focus-visible:ring-2 focus-visible:ring-amber-400 disabled:opacity-50";
export const PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS =
  "w-full min-w-0 border-slate-700 bg-slate-800 text-white";
export const PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS =
  "z-100 border-slate-700 bg-slate-800";

// subtitle codecs that are image-based and cannot be converted to WebVTT
export const BITMAP_SUBTITLE_CODECS = [
  "hdmv_pgs_subtitle",
  "dvd_subtitle",
  "dvb_subtitle",
] as const;

/** ISO 639-2 three-letter codes → ISO 639-1 two-letter codes for the supported languages.
 * ffprobe often emits 3-letter codes; the user-facing language picker uses 2-letter codes. */
export const ISO_639_2_TO_1: Record<string, string> = {
  ara: "ar",
  ces: "cs",
  cze: "cs",
  dan: "da",
  deu: "de",
  ger: "de",
  ell: "el",
  gre: "el",
  eng: "en",
  spa: "es",
  fin: "fi",
  fra: "fr",
  fre: "fr",
  heb: "he",
  hin: "hi",
  hun: "hu",
  ita: "it",
  jpn: "ja",
  kor: "ko",
  nld: "nl",
  dut: "nl",
  nor: "no",
  pol: "pl",
  por: "pt",
  ron: "ro",
  rum: "ro",
  rus: "ru",
  swe: "sv",
  tha: "th",
  tur: "tr",
  ukr: "uk",
  vie: "vi",
  zho: "zh",
  chi: "zh",
};

/** ISO 639-1 two-letter codes → English display names (audio + subtitle labels). */
export const LANGUAGE_NAMES: Record<string, string> = {
  ar: "Arabic",
  cs: "Czech",
  da: "Danish",
  de: "German",
  el: "Greek",
  en: "English",
  es: "Spanish",
  fi: "Finnish",
  fr: "French",
  he: "Hebrew",
  hi: "Hindi",
  hu: "Hungarian",
  it: "Italian",
  ja: "Japanese",
  ko: "Korean",
  nl: "Dutch",
  no: "Norwegian",
  pl: "Polish",
  pt: "Portuguese",
  ro: "Romanian",
  ru: "Russian",
  sv: "Swedish",
  th: "Thai",
  tr: "Turkish",
  uk: "Ukrainian",
  vi: "Vietnamese",
  zh: "Chinese",
};

/** Key crew: max writing-department rows before "Show all crew" */
export const MOVIE_DETAILS_KEY_CREW_WRITERS_CAP = 3;

/**
 * tw-animate-css enter for library movie details (same family as MovieCard / AlbumCard).
 * Stagger with `delay-*` + `motion-reduce:delay-0` on wrappers.
 */
export const MOVIE_DETAILS_CONTENT_ENTER_CLASS =
  "animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-300 ease-out motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:translate-y-0";

// Settings page transitions
export const SETTINGS_PAGE_TRANSITION_MS = 200;
export const SETTINGS_PAGE_VIEW_TRANSITION_NAME = "settings-page";
export const SETTINGS_PAGE_CONTENT_ENTER_CLASS =
  "animate-in fade-in-0 fill-mode-both duration-200 ease-out motion-reduce:animate-none motion-reduce:opacity-100";
export const SETTINGS_PAGE_CONTENT_EXIT_CLASS =
  "animate-out fade-out-0 fill-mode-both duration-200 ease-in motion-reduce:animate-none motion-reduce:opacity-100";

// watch rooms
export const WATCH_ROOM_SEEK_STEP_SEC = 10;
export const WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC = 1.5;
export const WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS = 1200;
