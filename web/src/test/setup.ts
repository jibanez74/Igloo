import "@testing-library/jest-dom/vitest";
import { cleanup, configure } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// Route and dialog tests await React.lazy chunks whose first import pays a cold
// Vite transform, which on CI runners routinely outlasts testing-library's 1s
// default for findBy*/waitFor. Mirrors the testTimeout headroom set for the
// same reason in vite.config.ts.
configure({ asyncUtilTimeout: 5000 });

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

if (!window.matchMedia) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
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

if (!window.HTMLElement.prototype.scrollIntoView) {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
}

// jsdom's canPlayType answers "" for everything, which would make the
// direct-play probe refuse every file in unit tests. Mimic Chrome instead:
// "" for native HLS manifests (so supportsNativeHLS stays false and the
// player keeps its hls.js path) and "probably" otherwise (so the static
// direct-play rules stay decisive). Tests exercising the probe itself inject
// their own fake element via createCanPlayProbe.
window.HTMLMediaElement.prototype.canPlayType = (type: string) =>
  type.toLowerCase().includes("mpegurl") ? "" : "probably";

if (!window.ResizeObserver) {
  class ResizeObserverMock {
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
  }

  Object.defineProperty(window, "ResizeObserver", {
    writable: true,
    value: ResizeObserverMock,
  });
  Object.defineProperty(globalThis, "ResizeObserver", {
    writable: true,
    value: ResizeObserverMock,
  });
}
