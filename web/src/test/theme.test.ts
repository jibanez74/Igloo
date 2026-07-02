import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  applyTheme,
  getStoredTheme,
  setTheme,
  THEME_COLORS,
  THEME_STORAGE_KEY,
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
});
