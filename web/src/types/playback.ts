import type { STREAM_MODE_IDS } from "@/lib/constants";

export type StreamModeId = (typeof STREAM_MODE_IDS)[number];

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

export type MoviePlaybackStatus =
  | { kind: "ready" }
  | { kind: "notFound" }
  | { kind: "loading"; message: string }
  | { kind: "modeUnavailable"; modeLabel: string }
  | { kind: "error"; message: string };
