// jsdom shims that individual suites need to vary per test, unlike the global
// ones in src/test/setup.ts. `restoreMocks`/`unstubGlobals` in vite.config.ts
// undo spies and stubbed globals between tests, but `Object.defineProperty`
// writes are not tracked, so matchMedia callers must restore it themselves.

import { vi } from "vitest";

const realMatchMedia = window.matchMedia;

/**
 * Makes `(prefers-reduced-motion: reduce)` answer `prefersReducedMotion` and
 * every other query answer false. Pair with `restoreMatchMedia` in `afterEach`.
 */
export function setReducedMotionPreference(prefersReducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches:
        query === "(prefers-reduced-motion: reduce)" && prefersReducedMotion,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

/** Puts back the matchMedia installed by setup.ts. */
export function restoreMatchMedia() {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: realMatchMedia,
  });
}

/**
 * Stubs the media transport methods jsdom does not implement. Returns nothing:
 * `restoreMocks` puts the prototype methods back after each test.
 */
export function stubMediaElement() {
  vi.spyOn(window.HTMLMediaElement.prototype, "load").mockImplementation(
    () => {},
  );
  vi.spyOn(window.HTMLMediaElement.prototype, "play").mockImplementation(() =>
    Promise.resolve(),
  );
  vi.spyOn(window.HTMLMediaElement.prototype, "pause").mockImplementation(
    () => {},
  );
}
