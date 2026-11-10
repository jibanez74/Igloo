import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "../e2e-browser-issues";
import {
  expectScreenshot,
  settle,
  THEMES,
  useTheme,
  type Theme,
} from "./visual-helpers";

// Baselines of every primitive in the dev-only gallery (/gallery). A change
// meant to be invisible must leave these untouched; a change meant to be seen
// updates the ones it moves, on purpose, in its own commit.

const SECTIONS = [
  "buttons",
  "badges",
  "fields",
  "controls",
  "surfaces",
  "navigation",
  "tokens",
];

// What each utility of the elevation, layer and duration scales
// (src/lib/theme-tokens.ts) must compute to. The floating shadow is the one
// value that differs by theme.
const FLOATING_SHADOW = {
  light: "rgba(0, 0, 0, 0.15) 0px 25px 50px -12px",
  dark: "rgba(0, 0, 0, 0.5) 0px 25px 50px -12px",
};
const TOKEN_STYLES = [
  ["z-chrome", "zIndex", "40"],
  ["z-modal", "zIndex", "50"],
  ["z-popover", "zIndex", "60"],
  ["duration-micro", "transitionDuration", "0.15s"],
  ["duration-standard", "transitionDuration", "0.2s"],
  ["duration-page", "transitionDuration", "0.3s"],
] as const;

// Overlay name → the role its open content exposes.
const OVERLAYS = [
  ["dialog", "dialog"],
  ["alert-dialog", "alertdialog"],
  ["select", "listbox"],
  ["dropdown", "menu"],
  ["popover", "dialog"],
] as const;

async function openGallery(page: Page, theme: Theme, query = "") {
  const apiRequests: string[] = [];
  page.on("request", request => {
    const { pathname } = new URL(request.url());
    if (pathname.startsWith("/api/")) apiRequests.push(pathname);
  });

  await useTheme(page, theme);
  await page.goto(`/gallery${query}`);
  await expect(page.locator("[data-gallery-root]")).toBeVisible();
  await settle(page);

  // The gallery is static: a request here means a primitive grew a
  // dependency on the app around it.
  expect(apiRequests).toEqual([]);
}

for (const theme of THEMES) {
  test.describe(`gallery, ${theme}`, () => {
    // Each section alone on the page (`?section=`). Where a section sits
    // changes a few of its anti-aliased pixels, so on the full gallery a
    // section that grew would redraw every baseline below it.
    for (const section of SECTIONS) {
      test(`${section} section`, async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openGallery(page, theme, `?section=${section}`);

        const sections = page.locator("[data-gallery]");
        await expect(sections).toHaveCount(1);
        await expectScreenshot(sections, `${section}-${theme}.png`);

        tracker.assertClean();
      });
    }

    test("the full gallery shows every section", async ({ page }) => {
      await openGallery(page, theme);

      await expect(page.locator("[data-gallery]")).toHaveCount(
        SECTIONS.length,
      );
    });

    test("the token utilities compute to their scale values", async ({
      page,
    }) => {
      await openGallery(page, theme, "?section=tokens");
      const computed = (token: string, property: string) =>
        page
          .locator(`[data-token="${token}"]`)
          .evaluate(
            (element, name) =>
              getComputedStyle(element).getPropertyValue(name),
            property,
          );

      expect(await computed("shadow-floating", "box-shadow")).toContain(
        FLOATING_SHADOW[theme],
      );

      // Tailwind's shadow-lg geometry, in the theme's primary at 20%.
      const glow = await computed("shadow-glow", "box-shadow");
      expect(glow).toMatch(/\/ 0\.2\) 0px 10px 15px -3px/);
      expect(glow).toMatch(/\/ 0\.2\) 0px 4px 6px -4px$/);

      for (const [token, property, value] of TOKEN_STYLES) {
        const cssName = property.replace(/[A-Z]/g, c => `-${c.toLowerCase()}`);
        expect(await computed(token, cssName), token).toBe(value);
      }
    });

    test("keyboard focus and hover", async ({ page }) => {
      await openGallery(page, theme, "?section=buttons");
      const buttons = page.locator('[data-gallery="buttons"]');

      // A real Tab, so the ring is the keyboard one (:focus-visible). The
      // buttons section comes first in the gallery; the assertion below says
      // so if that ever changes.
      await page.keyboard.press("Tab");
      await expect(buttons.getByRole("button").first()).toBeFocused();
      await expectScreenshot(buttons, `buttons-focus-${theme}.png`);

      await buttons
        .getByRole("button", { name: "Dialog secondary" })
        .hover();
      await expectScreenshot(buttons, `buttons-hover-${theme}.png`);
    });

    for (const [name, role] of OVERLAYS) {
      test(`${name} open`, async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openGallery(page, theme, `?open=${name}`);

        await expect(page.getByRole(role)).toBeVisible();
        await expectScreenshot(page, `overlay-${name}-${theme}.png`);

        tracker.assertClean();
      });
    }
  });
}
