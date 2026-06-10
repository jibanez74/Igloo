import type { RefObject } from "react";
import type { STREAM_MODE_IDS } from "@/lib/constants";
import type { SubtitleType } from "./movies";
import type { PlaySearchParams } from "./route-search";

export type StreamModeId = (typeof STREAM_MODE_IDS)[number];

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

export type PlaybackModeOption = {
  id: StreamModeId;
};

export type MoviePlaybackStatus =
  | { kind: "ready" }
  | { kind: "notFound" }
  | { kind: "loading"; message: string }
  | { kind: "modeUnavailable"; modeLabel: string }
  | { kind: "error"; message: string };

export type MoviePlaybackStatusArgs = {
  movieNotFound: boolean;
  movieIsPending: boolean;
  hasMovie: boolean;
  requestedMode: StreamModeId;
  techPending: boolean;
  modeUnavailable: boolean;
  playbackError: string | null;
};

export type PlaybackTimingOptions = {
  isHlsPlayback: boolean;
  hlsStartSec: number;
  movieDurationSec?: number;
};

export type SubtitleTrackInfo = {
  url: string;
  label: string;
  srclang: string;
};

export type SubtitleTrackInfoOptions = {
  movieId: number;
  resolvedSubtitleTrack: number | null;
  techLoaded: boolean;
  subtitleStreams: SubtitleType[];
};

export type RebaseOptions = {
  isHlsPlayback: boolean;
  targetTimeSec: number;
  hlsStartSec: number;
  currentVideoTimeSec: number;
};

export type MoviePlaybackSyncTarget = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

export type UseMoviePlaybackDataArgs = {
  movieId: number;
  search: PlaySearchParams;
  streamReloadKey: number;
  playbackSessionId: string;
  onSyncSearch: (target: MoviePlaybackSyncTarget) => void;
};

export type HlsSessionRecoveryOptions = {
  streamWindowKey: string;
  onRecover: (currentTimeSec: number) => void;
  onMaxAttempts: (message: string) => void;
};

export type VideoPlayerProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  title: string;
  isFullscreen?: boolean;
  onError: (message: string) => void;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onTimeUpdate?: (time: number) => void;
  onDurationChange?: (duration: number) => void;
  onNativeError?: (code: number | null | undefined) => void;
  subtitleTrack?: SubtitleTrackInfo | null;
  startSec?: number;
  onStartApplied?: (time: number) => void;
  onSessionLost?: (currentTime: number) => void;
};
