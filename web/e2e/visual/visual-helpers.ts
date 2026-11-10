import { expect, type Page } from "@playwright/test";

export const THEMES = ["dark", "light"] as const;
export type Theme = (typeof THEMES)[number];

/** Chooses the theme before the app's first paint, as a stored choice would. */
export async function useTheme(page: Page, theme: Theme) {
  await page.addInitScript(
    value => localStorage.setItem("igloo-theme", value),
    theme,
  );
}

/**
 * Waits until nothing that changes pixels is still arriving: the boot splash
 * is gone, Inter has loaded (it is `font-display: swap`), and the network is
 * quiet.
 */
export async function settle(page: Page) {
  await expect(page.locator("#initial-splash")).toHaveCount(0);
  await page.waitForLoadState("networkidle");
  await page.evaluate(() => document.fonts.ready);
}
