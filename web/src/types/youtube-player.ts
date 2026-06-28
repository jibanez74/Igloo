import type { RefCallback } from "react";

export type UseYouTubePlayerOptions = {
  videoId: string | null;
  autoplay?: boolean;
  controls?: boolean;
  onReady?: () => void;
  onStateChange?: (state: YT.PlayerState) => void;
  onError?: (error: YT.PlayerError) => void;
  onEnd?: () => void;
};

export type UseYouTubePlayerReturn = {
  containerRef: RefCallback<HTMLDivElement>;
  isReady: boolean;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isMuted: boolean;
  error: string | null;
  play: () => void;
  pause: () => void;
  togglePlay: () => void;
  seekTo: (seconds: number) => void;
  seekForward: (seconds: number) => void;
  seekBackward: (seconds: number) => void;
  setVolume: (volume: number) => void;
  mute: () => void;
  unmute: () => void;
  toggleMute: () => void;
  retry: () => void;
};
