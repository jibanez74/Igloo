// TanStack Query keys. Keep these values stable and use the exported constants
// for query options, cache reads/writes, invalidation, and tests.
export const ADMIN_USERS_KEY = "admin-users";
export const AUTH_USER_KEY = "auth-user";
export const SETTINGS_KEY = "settings";
export const GENERAL_SETTINGS_KEY = "general-settings";
export const PLAYBACK_SETTINGS_KEY = "playback-settings";

export const NOTIFICATIONS_KEY = "notifications";
export const NOTIFICATIONS_UNREAD_COUNT_KEY = "notifications-unread-count";

export const WATCH_ROOMS_KEY = "watch-rooms";
export const WATCH_ROOM_KEY = "watch-room";
export const WATCH_ROOM_INVITE_USERS_KEY = "watch-room-invite-users";

export const TMDB_STATUS_KEY = "tmdb-status";
export const SPOTIFY_STATUS_KEY = "spotify-status";

export const MOVIES_IN_THEATERS_KEY = "movies-in-theaters";
export const LATEST_MOVIES_KEY = "latest-movies";
export const MOVIE_DETAILS_KEY = "movie-details";
export const LIBRARY_MOVIE_DETAILS_KEY = "library-movie-details";
export const MOVIE_TECHNICAL_DETAILS_KEY = "movie-technical-details";
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

export const LATEST_ALBUMS_KEY = "latest-albums";
export const ALBUM_DETAILS_KEY = "album-details";
export const MUSICIAN_DETAILS_KEY = "musician-details";
export const TRACKS_INFINITE_KEY = "tracks-infinite";
export const ALBUMS_PAGINATED_KEY = "albums-paginated";
export const MUSICIANS_PAGINATED_KEY = "musicians-paginated";
export const MUSIC_STATS_KEY = "music-stats";
export const PLAYLISTS_KEY = "playlists";
export const PLAYLIST_DETAILS_KEY = "playlist-details";
export const PLAYLIST_TRACKS_KEY = "playlist-tracks";
export const LIKED_TRACKS_KEY = "liked-tracks";
export const LIKED_TRACK_IDS_KEY = "liked-track-ids";

export const SEARCH_ALL_KEY = "search-all";
export const SEARCH_MOVIES_KEY = "search-movies";
export const SEARCH_ALBUMS_KEY = "search-albums";
export const SEARCH_MUSICIANS_KEY = "search-musicians";
export const SEARCH_TRACKS_KEY = "search-tracks";

// Pagination and list-size defaults. These values mirror server defaults where
// noted and should be used by API wrappers, query options, routes, and tests.
export const SEARCH_PER_PAGE = 24;
export const ALBUMS_PER_PAGE = 24;
export const MUSICIANS_PER_PAGE = 24;
export const MOVIES_PER_PAGE = 24;
export const TRACKS_INFINITE_PAGE_SIZE = 50;
export const PLAYLIST_TRACKS_PAGE_SIZE = 50;
export const LIKED_TRACKS_PER_PAGE = 50;
export const SHUFFLE_TRACKS_LIMIT = 50;

// Route search defaults. Reuse these when navigating so links and loaders agree
// on the canonical starting search state for a route.
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

// TMDB image proxy settings. API responses provide paths only; the frontend
// builds same-origin proxy URLs with these sizes.
export const TMDB_IMAGE_BASE = "/api/tmdb/images";
export const TMDB_BACKDROP_SIZE = "w1280";
export const TMDB_POSTER_SIZE = "w500";
export const TMDB_PROFILE_SIZE = "w185";
export const TMDB_LOGO_SIZE = "w92";

// Virtual-list measurements in pixels. These keep virtualized rows stable.
export const VIRTUAL_LIST_LETTER_HEIGHT = 52;
export const VIRTUAL_LIST_TRACK_HEIGHT = 60;

// Playback and HLS constants. Stream modes are the source of truth for IDs,
// labels, and profile metadata used by route validation and playback UI.
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

function streamModeIds<T extends readonly { id: string }[]>(modes: T) {
  return modes.map((mode) => mode.id) as {
    readonly [K in keyof T]: T[K] extends { readonly id: infer Id }
      ? Id
      : never;
  };
}

export const STREAM_MODE_IDS = streamModeIds(STREAM_MODES);

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

/** Max session-lost recoveries per stream window (enforced in `useHlsSessionRecovery`). */
export const HLS_SESSION_LOST_MAX_ATTEMPTS = 3;
/** Min ms between recovery attempts to avoid tight loops when `src` updates re-trigger 404. */
export const HLS_SESSION_LOST_MIN_INTERVAL_MS = 2000;

/** hls.js: manifest / level / fragment request timeout (ms). */
export const HLS_JS_LOAD_TIMEOUT_MS = 120_000;
/** Resume HLS sessions this far before the target so short rewinds work without rebasing. */
export const HLS_RESUME_REWIND_BUFFER_SEC = 10;
/** hls.js: seconds of already-played buffer to keep behind the playhead. */
export const HLS_JS_BACK_BUFFER_LENGTH_SEC = 120;

// Playback settings dialog UI values and class contracts.
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
  "w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50";
export const PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS =
  "w-full min-w-0 border-border bg-muted text-foreground";
export const PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS =
  "z-100 border-border bg-muted";

// Subtitle and language constants used to choose supported subtitle behavior
// and format audio/subtitle labels.
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

// Shared motion tokens and class contracts. Keep reduced-motion behavior in
// these constants so components stay consistent.
export const MOTION_DURATION_MICRO_MS = 150;
export const MOTION_DURATION_STANDARD_MS = 200;
export const MOTION_DURATION_PAGE_MS = 300;

export const MOTION_MICRO_CONTROL_CLASS =
  "transition-[background-color,border-color,color,box-shadow,opacity] duration-150 ease-out motion-reduce:transition-none";
export const MOTION_MICRO_OPACITY_CLASS =
  "transition-opacity duration-150 motion-reduce:transition-none";
export const MOTION_PROGRESS_FILL_CLASS =
  "transition-[width] duration-150 ease-out motion-reduce:transition-none";
export const MOTION_PROGRESS_THUMB_REVEAL_CLASS = MOTION_MICRO_OPACITY_CLASS;
export const MOTION_CONTROL_THUMB_TRANSFORM_CLASS =
  "transition-transform duration-200 motion-reduce:transition-none";
export const MOTION_SETTINGS_SURFACE_CLASS =
  "transition-colors duration-200 motion-reduce:transition-none";
export const MOTION_TRACK_ROW_CLASS =
  "transition-[background-color,opacity,box-shadow] duration-150 motion-reduce:transition-none";
export const MOTION_TRACK_PLAY_BUTTON_CLASS =
  "transition-[background-color,opacity] duration-150 motion-reduce:transition-none";
export const MOTION_TRACK_ICON_BUTTON_CLASS =
  "transition-[color,opacity] duration-150 motion-reduce:transition-none";
export const MOTION_TRACK_MENU_TRIGGER_CLASS =
  "transition-colors duration-150 motion-reduce:transition-none";
export const MOTION_ROW_SURFACE_CLASS =
  "transition-[background-color,border-color,box-shadow] duration-200 ease-out motion-reduce:transition-colors";
export const MOTION_PLAYER_CHROME_PANEL_CLASS =
  "transition-[opacity,transform] duration-200 ease-out motion-reduce:transition-none motion-reduce:transform-none";
export const MOTION_MEDIA_OVERLAY_CLASS =
  "transition-opacity duration-200 ease-out motion-reduce:transition-none";
export const MOTION_MEDIA_DIALOG_SURFACE_CLASS =
  "border-border bg-card shadow-2xl shadow-black/40 motion-reduce:animate-none";
export const MOTION_MEDIA_OVERLAY_ENTER_CLASS =
  "animate-in fade-in zoom-in-95 slide-in-from-bottom-2 fill-mode-both duration-200 ease-out motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:scale-100 motion-reduce:translate-y-0";
export const MOTION_PLAYER_CHROME_ENTER_CLASS =
  "animate-in fade-in slide-in-from-bottom fill-mode-both duration-200 ease-out motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:translate-y-0";
export const MOTION_PLAYER_CHROME_BUTTON_CLASS = MOTION_MICRO_CONTROL_CLASS;
export const MOTION_PAGE_ENTER_CLASS =
  "animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-300 ease-out motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:translate-y-0";
export const MOTION_SECTION_ENTER_CLASS =
  "animate-in fade-in-0 fill-mode-both duration-200 ease-out motion-reduce:animate-none motion-reduce:opacity-100";
export const MOTION_SECTION_ENTER_DELAYED_CLASS =
  "animate-in fade-in-0 fill-mode-both duration-200 ease-out delay-75 motion-reduce:animate-none motion-reduce:opacity-100 motion-reduce:delay-0";
export const MOTION_LOADING_STATE_CLASS =
  "animate-pulse motion-reduce:animate-none";
export const MOTION_SPINNER_STATE_CLASS =
  "animate-spin motion-reduce:animate-none";
export const MOTION_DECORATIVE_PING_CLASS =
  "animate-ping motion-reduce:animate-none";
export const MOTION_DECORATIVE_BOUNCE_CLASS =
  "animate-bounce motion-reduce:animate-none";
export const MOTION_DECORATIVE_STATE_CLASS =
  "transition-[opacity,transform] duration-300 ease-out motion-reduce:transition-none motion-reduce:transform-none";

export const DETAIL_PAGE_CONTENT_ENTER_CLASS = MOTION_PAGE_ENTER_CLASS;
export const CARD_INTERACTIVE_SURFACE_CLASS =
  "transition-[border-color,box-shadow,transform] duration-200 ease-out motion-reduce:transition-colors motion-reduce:hover:translate-y-0";
/** Shared media-card chrome: tokenized surface + glacier hover glow. */
export const CARD_SURFACE_CLASS =
  "group relative overflow-hidden rounded-xl border border-border bg-card hover:-translate-y-1 hover:border-primary/50 hover:shadow-xl hover:shadow-primary/20";
export const CARD_MEDIA_HOVER_CLASS =
  "transition-transform duration-200 ease-out group-hover:scale-105 motion-reduce:transition-none motion-reduce:group-hover:scale-100";
export const CARD_OVERLAY_REVEAL_CLASS = MOTION_MEDIA_OVERLAY_CLASS;
export const CARD_ACTION_REVEAL_CLASS =
  "transition-[background-color,opacity,transform] duration-200 ease-out motion-reduce:transition-colors motion-reduce:scale-100";

// Shared content fade transitions
export const CONTENT_FADE_TRANSITION_MS = MOTION_DURATION_STANDARD_MS;
export const CONTENT_FADE_ENTER_CLASS = MOTION_SECTION_ENTER_CLASS;
export const CONTENT_FADE_EXIT_CLASS =
  "animate-out fade-out-0 fill-mode-both duration-200 ease-in motion-reduce:animate-none motion-reduce:opacity-100";

// Watch-room playback and synchronization behavior.
export const WATCH_ROOM_SEEK_STEP_SEC = 10;
export const WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC = 1.5;
export const WATCH_ROOM_SYNC_ANNOUNCE_DEBOUNCE_MS = 1200;
