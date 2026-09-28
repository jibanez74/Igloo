import { expect, test, type Page } from "@playwright/test";

import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { loginPageViaApi } from "./e2e-auth";

async function expectDecorativeAnimationsStopped(page: Page) {
  const decorativeStates = await page.evaluate(() =>
    Array.from(document.querySelectorAll('[data-motion="decorative"]')).map(
      element => {
        const styles = window.getComputedStyle(element);
        const rect = element.getBoundingClientRect();
        const animationNames = styles.animationName
          .split(",")
          .map(name => name.trim());
        const animationDurations = styles.animationDuration
          .split(",")
          .map(duration => duration.trim());

        return {
          animationDurations,
          animationNames,
          visible:
            rect.width > 0 &&
            rect.height > 0 &&
            styles.display !== "none" &&
            styles.visibility !== "hidden",
        };
      },
    ),
  );

  // How many there are is ComingSoon's unit contract; there must be some, or
  // the check below proves nothing.
  expect(decorativeStates.length).toBeGreaterThan(0);
  expect(
    decorativeStates.filter(
      state =>
        state.visible &&
        state.animationNames.some((name, index) => {
          if (name === "none") {
            return false;
          }

          return state.animationDurations[index] !== "0s";
        }),
    ),
  ).toEqual([]);
}

test.describe("Reduced motion", () => {
  test("the ComingSoon page keeps its content visible and stops decorative loops", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await loginPageViaApi(page);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1280, height: 900 });

    await page.goto("/photos");
    await expect(page.getByRole("heading", { name: "Photos" })).toBeVisible();
    await expectPageHasNoHorizontalScroll(page);
    await expectDecorativeAnimationsStopped(page);

    browserIssues.assertClean();
  });
});
