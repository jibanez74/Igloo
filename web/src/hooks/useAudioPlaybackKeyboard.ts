import { useEffect, useEffectEvent } from "react";
import type { RefObject } from "react";
import { AUDIO_SEEK_STEP_SECONDS, AUDIO_VOLUME_STEP } from "@/lib/constants";

// Controls whose native keyboard interaction must win over the global
// playback shortcuts.
const INTERACTIVE_SELECTOR =
  'button, a[href], select, summary, [role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"], [role="option"], [role="tab"], [role="radio"], [role="checkbox"], [role="switch"], [role="slider"], [role="spinbutton"]';

// Overlays that own their keyboard interaction entirely. The player's own
// fullscreen dialog is exempted via the data-audio-player marker.
const FOREIGN_OVERLAY_SELECTOR =
  '[role="dialog"]:not([data-audio-player]), [role="alertdialog"], [role="menu"], [role="listbox"]';

const PLAYER_CHROME_SELECTOR = "[data-audio-player]";

const ARROW_KEYS = new Set(["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"]);

type AudioPlaybackKeyboardOptions = {
  audioRef: RefObject<HTMLAudioElement | null>;

  // False while there is nothing to control or another player owns the
  // keyboard (the video player suspends these shortcuts).
  enabled: boolean;

  onTogglePlay: () => void;
  onSeekBy: (offsetSeconds: number) => void;
  onNextTrack: () => void;
  onPreviousTrack: () => void;
  onRestart: () => void;
};

// App-wide playback shortcuts for the audio player, mirroring the video
// player's key map (see useVideoPlaybackKeyboard): Space/K toggle, J/L and
// ←/→ seek, ↑/↓ volume, N/P next/previous, R/Home/0 restart, M mute.
//
// The audio player is always mounted, so unlike the video player these listen
// globally and step aside for anything that owns its own keys: text inputs,
// foreign overlays, and focused controls outside the player's chrome. Inside
// the chrome the keys always control playback (Enter still activates the
// focused button).
export function useAudioPlaybackKeyboard({
  audioRef,
  enabled,
  onTogglePlay,
  onSeekBy,
  onNextTrack,
  onPreviousTrack,
  onRestart,
}: AudioPlaybackKeyboardOptions) {
  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    const audio = audioRef.current;

    if (!enabled || !audio) {
      return;
    }

    const target = event.target instanceof HTMLElement ? event.target : null;

    // The player's own sliders (seek, volume) stay eligible for the global
    // shortcuts — they never take text input — but their range navigation keys
    // belong to the native slider. Anything else that takes typing suppresses
    // shortcuts.
    const isPlayerRangeInput =
      target instanceof HTMLInputElement &&
      target.type === "range" &&
      target.closest(PLAYER_CHROME_SELECTOR) !== null;

    if (
      target &&
      !isPlayerRangeInput &&
      (target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable)
    ) {
      return;
    }

    if (
      isPlayerRangeInput &&
      (ARROW_KEYS.has(event.key) || event.key === "Home")
    ) {
      return;
    }

    if (event.ctrlKey || event.metaKey || event.altKey) {
      return;
    }

    if (target) {
      if (target.closest(FOREIGN_OVERLAY_SELECTOR)) {
        return;
      }

      const interactive = target.closest(INTERACTIVE_SELECTOR);
      if (interactive && !interactive.closest(PLAYER_CHROME_SELECTOR)) {
        // Outside the player, Space must activate the focused control and
        // navigation keys belong to widgets like tabs and radios.
        if (
          event.key === " " ||
          ARROW_KEYS.has(event.key) ||
          event.key === "Home"
        ) {
          return;
        }
      }
    }

    switch (event.key) {
      case " ":
      case "k":
      case "K":
        event.preventDefault();
        onTogglePlay();
        break;
      case "ArrowLeft":
      case "j":
      case "J":
        event.preventDefault();
        onSeekBy(-AUDIO_SEEK_STEP_SECONDS);
        break;
      case "ArrowRight":
      case "l":
      case "L":
        event.preventDefault();
        onSeekBy(AUDIO_SEEK_STEP_SECONDS);
        break;
      case "ArrowUp":
        event.preventDefault();
        // Unmute like the video player does, so the key always has an audible
        // effect rather than silently moving a muted volume.
        audio.muted = false;
        audio.volume = Math.min(1, audio.volume + AUDIO_VOLUME_STEP);
        break;
      case "ArrowDown":
        event.preventDefault();
        audio.muted = false;
        audio.volume = Math.max(0, audio.volume - AUDIO_VOLUME_STEP);
        break;
      case "n":
      case "N":
      case "MediaTrackNext":
        event.preventDefault();
        onNextTrack();
        break;
      case "p":
      case "P":
      case "MediaTrackPrevious":
        event.preventDefault();
        onPreviousTrack();
        break;
      case "r":
      case "R":
      case "Home":
      case "0":
        event.preventDefault();
        onRestart();
        break;
      case "m":
      case "M":
        event.preventDefault();
        audio.muted = !audio.muted;
        break;
    }
  });

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);

    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);
}
