import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { expect, test, type Locator, type Page } from "@playwright/test";
import { comparePngs, type PixelDifference } from "./png";

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

/** Shots taken before the exact comparison gives up on a mismatch. */
const EXACT_ATTEMPTS = 3;

type ScreenshotOptions = {
  /** For a page target: the whole scrollable page instead of the viewport. */
  fullPage?: boolean;
};

const describeDifference = (difference: PixelDifference) => {
  switch (difference.kind) {
    case "same":
      return "no difference";
    case "size":
      return `size ${difference.actual}, baseline ${difference.expected}`;
    case "pixels": {
      const { left, top, right, bottom } = difference.box;
      return `${difference.count} differing pixels, inside x ${left}-${right}, y ${top}-${bottom}`;
    }
  }
};

const isPage = (target: Page | Locator): target is Page => "goto" in target;

/**
 * The screenshot assertion every visual spec uses: Playwright's
 * `toHaveScreenshot`, then a comparison with no allowance at all.
 *
 * `toHaveScreenshot` owns the baseline file, waits for the picture to stop
 * changing and writes the diff images of a failure. Its comparator does not
 * count a differing pixel it takes for anti-aliasing, though, and a 1px
 * border between two surfaces can pass for one. So the same target is shot
 * once more and compared with the baseline pixel for pixel (png.ts).
 *
 * Soft throughout: one test reports every image that moved, which is also
 * what lets a redraw replace them all in one run.
 */
export async function expectScreenshot(
  target: Page | Locator,
  name: string,
  { fullPage = false }: ScreenshotOptions = {},
) {
  const info = test.info();

  if (isPage(target)) {
    await expect.soft(target).toHaveScreenshot(name, { fullPage });
  } else {
    await expect.soft(target).toHaveScreenshot(name);
  }

  // A run that does not compare pixels (playwright.visual.config.ts) has no
  // baseline it can trust; a missing one has already failed above.
  const baselinePath = info.snapshotPath(name, { kind: "screenshot" });
  if (info.project.ignoreSnapshots || !existsSync(baselinePath)) return;

  const shared = { animations: "disabled", caret: "hide", scale: "css" } as const;
  const shoot = () =>
    isPage(target)
      ? target.screenshot({ ...shared, fullPage })
      : target.screenshot(shared);
  const baseline = readFileSync(baselinePath);
  const page = isPage(target) ? target : target.page();

  // The same pixels are not always drawn twice: now and then a few
  // anti-aliased pixels on a rounded corner come out one shade apart on one
  // shot and not the next. A real change never matches the baseline, however
  // often it is shot, so a mismatch is retried before it counts.
  let actual = await shoot();
  let difference = comparePngs(baseline, actual);
  for (
    let attempt = 1;
    difference.kind !== "same" && attempt < EXACT_ATTEMPTS;
    attempt += 1
  ) {
    await page.evaluate(
      () => new Promise(resolve => requestAnimationFrame(resolve)),
    );
    actual = await shoot();
    difference = comparePngs(baseline, actual);
    if (difference.kind === "same") {
      info.annotations.push({
        type: "exact comparison retried",
        description: `${name} matched on attempt ${attempt + 1}`,
      });
    }
  }
  if (difference.kind === "same") return;

  // --update-snapshots rewrites only what Playwright's comparator calls
  // changed, so a difference it does not count is redrawn here.
  const { updateSnapshots } = info.config;
  if (updateSnapshots === "all" || updateSnapshots === "changed") {
    writeFileSync(baselinePath, actual);
    return;
  }

  await info.attach(`${name}, actual (exact comparison)`, {
    body: actual,
    contentType: "image/png",
  });
  expect
    .soft(
      describeDifference(difference),
      `${name} must match its baseline pixel for pixel`,
    )
    .toBe("no difference");
}
