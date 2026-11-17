import { useEffect, useEffectEvent } from "react";
import type { RefObject } from "react";
import { shouldIgnorePlayerShortcut } from "@/lib/player-shortcuts";

type VideoPlaybackKeyboardOptions = {
  containerRef: RefObject<HTMLElement | null>;
  /**
   * The element whose readiness gates the shortcuts and whose volume they
   * change. A surface without one (the YouTube trailer) passes
   * `playerReady` and the volume callbacks instead.
   */
  videoRef?: RefObject<HTMLVideoElement | null>;
  playerReady?: boolean;
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
const HAVE_METADATA =
  typeof HTMLMediaElement === "undefined" ? 1 : HTMLMediaElement.HAVE_METADATA;
const SEEK_KEYS: ReadonlySet<string> = new Set([
  "ArrowLeft",
  "ArrowRight",
  "j",
  "J",
  "l",
  "L",
  "Home",
  "0",
]);

// The element's own volume, for a surface that passes no volume callbacks.
function nudgeVolume(video: HTMLVideoElement, delta: number) {
  video.volume = Math.min(1, Math.max(0, video.volume + delta));
  video.muted = false;
}

function toggleMuted(video: HTMLVideoElement) {
  video.muted = !video.muted;
}

/**
 * Which shortcuts the element can take. A seek needs a timeline, so it waits
 * for metadata; everything else only needs an element that has not failed.
 * Play, pause, mute and fullscreen therefore keep working through a
 * mid-playback stall, a rebase or a recovery reload, exactly as the on-screen
 * buttons do.
 */
function acceptsShortcut(video: HTMLVideoElement | null, key: string) {
  if (!video || video.error) return false;
  return !SEEK_KEYS.has(key) || video.readyState >= HAVE_METADATA;
}

export function useVideoPlaybackKeyboard({
  containerRef,
  videoRef,
  playerReady = false,
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

    if (shouldIgnorePlayerShortcut(e)) return;
    if (!targetInsidePlayer && !targetIsPageBody) return;

    const video = videoRef?.current;
    const ready = videoRef ? acceptsShortcut(video ?? null, e.key) : playerReady;
    if (!ready) {
      if (e.key === "Escape" && fullscreenActive && onEscape) {
        onEscape();
      }
      return;
    }

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
        if (onAdjustVolume) {
          consume();
          onAdjustVolume(volumeStep);
        } else if (video) {
          consume();
          nudgeVolume(video, volumeStep);
        }
        break;
      }
      case "ArrowDown": {
        if (onAdjustVolume) {
          consume();
          onAdjustVolume(-volumeStep);
        } else if (video) {
          consume();
          nudgeVolume(video, -volumeStep);
        }
        break;
      }
      case "m":
      case "M": {
        if (onToggleMute) {
          consume();
          onToggleMute();
        } else if (video) {
          consume();
          toggleMuted(video);
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
