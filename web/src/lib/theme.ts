// Client-side theme preference (light/dark), persisted to localStorage.
// The igloo light/dark palettes live in styles.css; this module just decides
// which one is active by toggling the `dark` class on <html>.
//
// NOTE: THEME_STORAGE_KEY and the THEME_COLORS hexes are intentionally
// duplicated by the inline anti-flash script in index.html (which runs before
// any module loads). Keep the two in sync.

export type Theme = "light" | "dark";

export const THEME_STORAGE_KEY = "igloo-theme";

// Matches <meta name="theme-color"> values; canvas #0A1322 (dark) / #F2F7FC (light).
export const THEME_COLORS: Record<Theme, string> = {
  dark: "#0A1322",
  light: "#F2F7FC",
};

// Body text on the canvas; mirrors the --foreground tokens and boot.css `color`.
export const THEME_TEXT_COLORS: Record<Theme, string> = {
  dark: "#F8FAFC",
  light: "#0A1322",
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
  for (const listener of listeners) {
    listener();
  }
}

/** Persists the theme to localStorage and applies it immediately. */
export function setTheme(theme: Theme): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // Ignore storage failures (private mode, quota); still apply for this session.
  }
  applyTheme(theme);
}
