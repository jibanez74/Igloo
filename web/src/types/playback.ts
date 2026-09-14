import type { STREAM_MODE_IDS } from "@/lib/constants";
import type {
  AudioStreamType,
  ChapterType,
  SubtitleType,
  VideoStreamType,
} from "@/types/movies";
import type { NullableFloat64 } from "@/types/nullable";
import type { components } from "@/types/openapi.gen";

export type StreamModeId = (typeof STREAM_MODE_IDS)[number];

/** The library entities the video player can play. */
export type PlaybackMediaKind = "movie" | "episode";

/**
 * What the player addresses: a movie id or a TV episode id. Every playback
 * URL, query key, and saved-progress row is scoped by this pair, so a movie
 * and an episode that happen to share an id never collide.
 */
export type PlaybackMediaRef = {
  kind: PlaybackMediaKind;
  id: number;
};

/**
 * The probe columns the playback decisions read. Movie rows carry movie_id and
 * show-file rows carry file_id; neither is a playback input, so the player and
 * its helpers are typed on the common subset and accept both.
 */
export type PlaybackVideoStreamType = Omit<VideoStreamType, "movie_id">;
export type PlaybackAudioStreamType = Omit<AudioStreamType, "movie_id">;
export type PlaybackSubtitleType = Omit<SubtitleType, "movie_id">;
export type PlaybackChapterType = Omit<ChapterType, "movie_id">;

// `data` payload of the watch-progress routes, shared by movies and TV
// episodes: GET /api/movies/{id}/watch-progress and
// GET /api/shows/episodes/{id}/watch-progress.
export type WatchProgressType = components["schemas"]["WatchProgress"];

/** The file behind the media, as far as the player needs it. */
export type PlaybackTechnicalFile = {
  mime_type: string;
  duration: NullableFloat64;
};

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

/**
 * Playback preferences that belong to a device rather than an account, stored
 * in localStorage. See src/lib/playback-preferences.ts.
 */
export type DevicePlaybackPreferences = {
  preferredProfile: string | null;
  downloadMbps: number | null;
  preferredAudioLanguage: string | null;
  /** A language code, or SUBTITLE_OFF_VALUE to keep subtitles off. */
  preferredSubtitleLanguage: string | null;
};

export type PlaybackStatus =
  | { kind: "ready" }
  | { kind: "notFound" }
  | { kind: "loading"; message: string }
  | { kind: "modeUnavailable"; modeLabel: string }
  | { kind: "error"; message: string };

/**
 * What the player offers when the current media ends. The route resolves it
 * (for TV, the episode that follows) and owns the navigation; the player only
 * renders the card and runs the countdown.
 */
export type UpNextItem = {
  /** "S1 E4 · Episode name" */
  title: string;
  stillUrl: string | null;
  /** True when the next item continues from a saved position. */
  resume: boolean;
  onPlay: () => void;
};
