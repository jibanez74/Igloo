import type { STREAM_MODE_IDS } from "@/lib/constants";

export type StreamModeId = (typeof STREAM_MODE_IDS)[number];

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

export type MoviePlaybackStatus =
  | { kind: "ready" }
  | { kind: "notFound" }
  | { kind: "loading"; message: string }
  | { kind: "modeUnavailable"; modeLabel: string }
  | { kind: "error"; message: string };
