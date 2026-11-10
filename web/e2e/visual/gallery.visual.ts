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

const SECTIONS = ["buttons", "badges", "fields", "surfaces", "navigation"];

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
    test("primitives", async ({ page }) => {
      const tracker = trackBrowserIssues(page);
      await openGallery(page, theme);

      for (const section of SECTIONS) {
        await expectScreenshot(
          page.locator(`[data-gallery="${section}"]`),
          `${section}-${theme}.png`,
        );
      }

      tracker.assertClean();
    });

    test("keyboard focus and hover", async ({ page }) => {
      await openGallery(page, theme);
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
