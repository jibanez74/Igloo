import { expect, test } from "@playwright/test";

import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";

test.describe("Side navigation", () => {
  test("collapses to icons, names them in tooltips, and stays collapsed after a reload", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);
    await loginPageViaApi(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/");

    const column = page.locator("[data-slot='sidebar-container']");
    const navigation = page.getByRole("navigation", { name: "Main navigation" });
    await expect(navigation.getByText("Library")).toBeVisible();
    expect((await column.boundingBox())?.width).toBe(256);

    const collapse = page.getByRole("button", { name: "Collapse sidebar" });
    await expect(collapse).toHaveAttribute("aria-expanded", "true");
    await collapse.click();

    const expand = page.getByRole("button", { name: "Expand sidebar" });
    await expect(expand).toHaveAttribute("aria-expanded", "false");
    await expect.poll(async () => (await column.boundingBox())?.width).toBe(48);
    await expect(navigation.getByText("Library")).toBeHidden();
    // The links keep their names with their labels gone.
    await expect(navigation.getByRole("link", { name: "TV Shows" })).toBeVisible();

    await navigation.getByRole("link", { name: "Movies" }).hover();
    await expect(page.getByRole("tooltip")).toHaveText("Movies");
    await page.mouse.move(640, 450);

    await page.reload();
    await expect(page.getByRole("button", { name: "Expand sidebar" })).toBeVisible();
    await expect.poll(async () => (await column.boundingBox())?.width).toBe(48);

    await page.getByRole("button", { name: "Expand sidebar" }).click();
    await expect.poll(async () => (await column.boundingBox())?.width).toBe(256);
    await expect(navigation.getByText("Library")).toBeVisible();

    browserIssues.assertClean();
  });

  test("the account block opens a menu that leads to Account settings", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);
    await loginPageViaApi(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/");

    const account = page.getByRole("button", { name: /^Account: / });
    await expect(account).toBeVisible();
    // Measured now: an open menu hides the rest of the page from role queries.
    const accountBox = await account.boundingBox();
    await account.click();

    const menu = page.getByRole("menu");
    await expect(menu.getByRole("menuitem")).toHaveText([
      "Account settings",
      /^Switch to (light|dark) theme$/,
      "Logout",
    ]);
    // It opens upward, inside the window. Polled: the menu's opening animation
    // scales and slides it for a moment, and a box measured mid-animation can
    // still overlap the account block.
    await expect
      .poll(async () => {
        const menuBox = await menu.boundingBox();
        return Boolean(
          menuBox &&
            accountBox &&
            menuBox.y >= 0 &&
            menuBox.y + menuBox.height <= accountBox.y,
        );
      })
      .toBe(true);

    await menu.getByRole("menuitem", { name: "Account settings" }).click();
    await expect(page).toHaveURL(/\/settings\/account$/);

    browserIssues.assertClean();
  });

  test("a phone opens the same navigation in a sheet", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);
    await loginPageViaApi(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");

    const open = page.getByRole("button", { name: "Open navigation" });
    const box = await open.boundingBox();
    expect(box && box.width >= 40 && box.height >= 40).toBe(true);
    await open.click();

    const sheet = page.getByRole("dialog", { name: "Navigation" });
    await expect(sheet.getByRole("link", { name: "Settings" })).toBeVisible();
    await expect(sheet.getByRole("button", { name: /^Account: / })).toBeVisible();
    // No tooltips in the sheet: the labels are on screen.
    await sheet.getByRole("link", { name: "Music" }).hover();
    await expect(page.getByRole("tooltip")).toHaveCount(0);

    browserIssues.assertClean();
  });
});
