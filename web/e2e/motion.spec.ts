import { expect, test } from "@playwright/test";

import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { loginPageViaApi } from "./e2e-auth";

test.describe("Reduced motion", () => {
  test("Home keeps its content visible without section entrance animations", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await loginPageViaApi(page);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1280, height: 900 });

    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(["Home"]);
    await expectPageHasNoHorizontalScroll(page);

    for (const name of [
      // The billboard: the mock's first Continue Watching item.
      "Frost Harbor",
      "Recently Added Movies",
      "Recently Added Shows",
      "Recently Added Albums",
      "Now Playing in Theaters",
    ]) {
      const section = page.getByRole("region", { name, exact: true });
      await expect(section).toBeVisible();
      await expect(section).toHaveCSS("animation-name", "none");
      await expect(section).toHaveCSS("opacity", "1");
      await expect(section).toHaveCSS("translate", "none");
    }

    browserIssues.assertClean();
  });
});
