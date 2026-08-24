// Client-side theme preference (light/dark), persisted to localStorage.
// The igloo light/dark palettes live in styles.css; this module just decides
// which one is active by toggling the `dark` class on <html>.
//
// The token values come from src/lib/theme-tokens.ts (the single token
// source); the inline anti-flash script in index.html duplicates the storage
// key and canvas hexes but is generated from the same module by
// `bun run generate:theme` (see docs/design-system.md §2.4).

import { MOTION_DURATION_STANDARD_MS } from "./constants";
import { getPrefersReducedMotion } from "./motion";
import { THEME_STORAGE_KEY, tokenHex } from "./theme-tokens";

export type Theme = "light" | "dark";

export { THEME_STORAGE_KEY };

/** Enables the theme cross-fade while present on <html>; see styles.css. */
export const THEME_SWITCH_CLASS = "theme-switching";

// Matches the generated <meta name="theme-color"> values (the canvas tokens).
export const THEME_COLORS: Record<Theme, string> = {
  dark: tokenHex("dark", "--background"),
  light: tokenHex("light", "--background"),
};

/** Reads the stored theme, defaulting to "dark" when absent or invalid. */
export function getStoredTheme(): Theme {
  try {
    return localStorage.getItem(THEME_STORAGE_KEY) === "light"
      ? "light"
      : "dark";
  } catch {
    return "dark";
  }
}

let activeTheme = getStoredTheme();

/** Returns the theme currently applied to this page session. */
export function getActiveTheme(): Theme {
  return activeTheme;
}

type ThemeListener = () => void;

const listeners = new Set<ThemeListener>();

/** Subscribes to theme changes; returns an unsubscribe function. */
export function subscribeTheme(listener: ThemeListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Applies the theme to the DOM: toggles the `dark` class and theme-color meta. */
export function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle("dark", theme === "dark");
  const meta = document.querySelector('meta[name="theme-color"]');
  meta?.setAttribute("content", THEME_COLORS[theme]);
  activeTheme = theme;
  for (const listener of listeners) {
    listener();
  }
}

let themeSwitchTimeoutId: number | undefined;

// Only user-initiated toggles animate: boot goes through applyTheme() directly
// (AppBoot + the index.html anti-flash script) and must switch instantly.
function beginThemeSwitchTransition(): void {
  if (getPrefersReducedMotion()) {
    return;
  }

  document.documentElement.classList.add(THEME_SWITCH_CLASS);
  window.clearTimeout(themeSwitchTimeoutId);
  themeSwitchTimeoutId = window.setTimeout(() => {
    document.documentElement.classList.remove(THEME_SWITCH_CLASS);
    themeSwitchTimeoutId = undefined;
  }, MOTION_DURATION_STANDARD_MS);
}

/** Persists the theme to localStorage and applies it with a brief cross-fade. */
export function setTheme(theme: Theme): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // Ignore storage failures (private mode, quota); still apply for this session.
  }
  beginThemeSwitchTransition();
  applyTheme(theme);
}
