import { expect, test, type Page } from "@playwright/test";
import { loginPageViaApi } from "../e2e-auth";
import { trackBrowserIssues } from "../e2e-browser-issues";
import { expectScreenshot, settle, THEMES, useTheme } from "./visual-helpers";

// Whole pages on the mock API server (mock-api-server.ts). The gallery proves
// a primitive did not move; these prove the pages built from primitives did
// not either, shell included.

const PHONE = { width: 390, height: 844 };

// The mock server stamps some rows with the time it started.
const FROZEN_NOW = new Date("2026-01-15T12:00:00Z");

async function openPage(page: Page, path: string) {
  await page.clock.setFixedTime(FROZEN_NOW);
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await settle(page);
}

for (const theme of THEMES) {
  test.describe(`pages, ${theme}`, () => {
    test.beforeEach(async ({ page }) => {
      await useTheme(page, theme);
    });

    test("login", async ({ page }) => {
      await openPage(page, "/login");
      await expectScreenshot(page, `login-${theme}.png`);
    });

    test.describe("signed in", () => {
      test.beforeEach(async ({ page }) => {
        await loginPageViaApi(page);
      });

      test("movie library", async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openPage(page, "/movies");
        await expectScreenshot(page, `movies-${theme}.png`);
        tracker.assertClean();
      });

      test("movie library on a phone, with the navigation sheet", async ({
        page,
      }) => {
        await page.setViewportSize(PHONE);
        await openPage(page, "/movies");
        await expectScreenshot(page, `movies-phone-${theme}.png`);

        await page.getByRole("button", { name: "Toggle Sidebar" }).click();
        await expect(
          page.getByRole("navigation", { name: "Main navigation" }),
        ).toBeVisible();
        await expectScreenshot(page, `nav-sheet-phone-${theme}.png`);
      });

      test("general settings", async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openPage(page, "/settings");
        await expectScreenshot(page, `settings-general-${theme}.png`, {
          fullPage: true,
        });
        tracker.assertClean();
      });

      test("playback settings, with a select open", async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openPage(page, "/settings/playback");
        await expectScreenshot(page, `settings-playback-${theme}.png`, {
          fullPage: true,
        });

        await page.getByRole("combobox", { name: "Profile" }).click();
        await expect(page.getByRole("listbox")).toBeVisible();
        await expectScreenshot(page, `settings-playback-select-${theme}.png`);
        tracker.assertClean();
      });

      test("users settings, with the Add User dialog", async ({ page }) => {
        const tracker = trackBrowserIssues(page);
        await openPage(page, "/settings/users");
        await expectScreenshot(page, `settings-users-${theme}.png`);

        await page.getByRole("button", { name: "Add User" }).click();
        await expect(page.getByRole("dialog", { name: "Add User" })).toBeVisible();
        await expectScreenshot(page, `settings-users-dialog-${theme}.png`);
        tracker.assertClean();
      });
    });
  });
}
