import { useEffect, useEffectEvent } from "react";
import type { RefObject } from "react";

type VideoPlaybackKeyboardOptions = {
  containerRef: RefObject<HTMLElement | null>;
  videoRef: RefObject<HTMLVideoElement | null>;
  enabled?: boolean;
  fullscreenActive?: boolean;
  volumeStep?: number;
  onShowControls?: () => void;
  onTogglePlay?: () => void;
  onSeekBackward?: () => void;
  onSeekForward?: () => void;
  onSeekToStart?: () => void;
  onAdjustVolume?: (delta: number) => void;
  onToggleMute?: () => void;
  onToggleFullscreen?: () => void;
  onEscape?: () => void;
};

const DEFAULT_VOLUME_STEP = 0.05;

export function useVideoPlaybackKeyboard({
  containerRef,
  videoRef,
  enabled = true,
  fullscreenActive = false,
  volumeStep = DEFAULT_VOLUME_STEP,
  onShowControls,
  onTogglePlay,
  onSeekBackward,
  onSeekForward,
  onSeekToStart,
  onAdjustVolume,
  onToggleMute,
  onToggleFullscreen,
  onEscape,
}: VideoPlaybackKeyboardOptions) {
  const handleKey = useEffectEvent((e: KeyboardEvent) => {
    if (!enabled) return;

    const target = e.target as HTMLElement;
    const container = containerRef.current;
    const targetInsidePlayer = container?.contains(target) ?? false;
    const targetIsPageBody =
      target === document.body || target === document.documentElement;

    if (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.tagName === "SELECT" ||
      target.isContentEditable
    ) {
      return;
    }
    if (!targetInsidePlayer && !targetIsPageBody) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    if (fullscreenActive) {
      onShowControls?.();
    }

    const consume = () => {
      e.preventDefault();
      e.stopPropagation();
    };

    switch (e.key) {
      case " ":
      case "k":
      case "K":
        if (onTogglePlay) {
          consume();
          onTogglePlay();
        }
        break;
      case "ArrowLeft":
      case "j":
      case "J":
        if (onSeekBackward) {
          consume();
          onSeekBackward();
        }
        break;
      case "ArrowRight":
      case "l":
      case "L":
        if (onSeekForward) {
          consume();
          onSeekForward();
        }
        break;
      case "ArrowUp": {
        const video = videoRef.current;
        if (onAdjustVolume) {
          consume();
          onAdjustVolume(volumeStep);
        } else if (video) {
          consume();
          video.volume = Math.min(1, video.volume + volumeStep);
          video.muted = false;
        }
        break;
      }
      case "ArrowDown": {
        const video = videoRef.current;
        if (onAdjustVolume) {
          consume();
          onAdjustVolume(-volumeStep);
        } else if (video) {
          consume();
          video.volume = Math.max(0, video.volume - volumeStep);
          video.muted = false;
        }
        break;
      }
      case "m":
      case "M": {
        const video = videoRef.current;
        if (onToggleMute) {
          consume();
          onToggleMute();
        } else if (video) {
          consume();
          video.muted = !video.muted;
        }
        break;
      }
      case "f":
      case "F":
        if (onToggleFullscreen) {
          consume();
          onToggleFullscreen();
        }
        break;
      case "Home":
      case "0":
        if (onSeekToStart) {
          consume();
          onSeekToStart();
        }
        break;
      case "Escape":
        if (onEscape) {
          onEscape();
        }
        break;
    }
  });

  useEffect(() => {
    const listener = (e: KeyboardEvent) => handleKey(e);
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, []);
}
