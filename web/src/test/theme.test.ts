import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MOTION_DURATION_STANDARD_MS } from "@/lib/constants";
import {
  applyTheme,
  getStoredTheme,
  setTheme,
  THEME_COLORS,
  THEME_STORAGE_KEY,
  THEME_SWITCH_CLASS,
} from "@/lib/theme";

describe("theme preference", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = "";
    document.head.innerHTML =
      '<meta name="theme-color" content="#000000" />';
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("defaults to dark when nothing is stored", () => {
    expect(getStoredTheme()).toBe("dark");
  });

  it("defaults to dark for an invalid stored value", () => {
    localStorage.setItem(THEME_STORAGE_KEY, "purple");
    expect(getStoredTheme()).toBe("dark");
  });

  it("reads a stored light preference", () => {
    localStorage.setItem(THEME_STORAGE_KEY, "light");
    expect(getStoredTheme()).toBe("light");
  });

  it("applyTheme toggles the dark class and theme-color meta", () => {
    applyTheme("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(
      document
        .querySelector('meta[name="theme-color"]')
        ?.getAttribute("content"),
    ).toBe(THEME_COLORS.light);

    applyTheme("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(
      document
        .querySelector('meta[name="theme-color"]')
        ?.getAttribute("content"),
    ).toBe(THEME_COLORS.dark);
  });

  it("setTheme persists and applies", () => {
    setTheme("light");
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
    expect(getStoredTheme()).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  describe("theme switch cross-fade", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
      vi.unstubAllGlobals();
    });

    const hasSwitchClass = () =>
      document.documentElement.classList.contains(THEME_SWITCH_CLASS);

    it("setTheme adds the switch class and removes it after the fade", () => {
      setTheme("light");
      expect(hasSwitchClass()).toBe(true);

      vi.advanceTimersByTime(MOTION_DURATION_STANDARD_MS);
      expect(hasSwitchClass()).toBe(false);
    });

    it("a rapid second toggle restarts the removal timer", () => {
      setTheme("light");
      vi.advanceTimersByTime(MOTION_DURATION_STANDARD_MS / 2);

      setTheme("dark");
      vi.advanceTimersByTime(MOTION_DURATION_STANDARD_MS / 2);
      expect(hasSwitchClass()).toBe(true);

      vi.advanceTimersByTime(MOTION_DURATION_STANDARD_MS / 2);
      expect(hasSwitchClass()).toBe(false);
    });

    it("skips the fade under prefers-reduced-motion but still applies", () => {
      vi.stubGlobal(
        "matchMedia",
        vi.fn().mockImplementation((query: string) => ({
          matches: query === "(prefers-reduced-motion: reduce)",
          media: query,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
        })),
      );

      setTheme("light");
      expect(hasSwitchClass()).toBe(false);
      expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
      expect(document.documentElement.classList.contains("dark")).toBe(false);
    });

    it("applyTheme alone never animates (boot path)", () => {
      applyTheme("light");
      expect(hasSwitchClass()).toBe(false);
    });
  });
});
