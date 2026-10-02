import { usePrefersCoarsePointer } from "@/hooks/use-coarse-pointer";

/**
 * Keyboard-shortcut hints for accessible names and sr-only keyboard maps.
 * A touch-first device has no keyboard to press them on, so they are dropped
 * there; the shortcuts themselves stay bound for an attached keyboard.
 */
export function useShortcutHints() {
  const showShortcutHints = !usePrefersCoarsePointer();
  const withShortcut = (label: string, keys: string) =>
    showShortcutHints ? `${label} (${keys})` : label;

  return { showShortcutHints, withShortcut };
}
