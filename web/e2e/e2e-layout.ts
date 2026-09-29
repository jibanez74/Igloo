import { expect, type Locator, type Page } from "@playwright/test";

export const VIEWPORTS = {
  phone: { width: 390, height: 844 },
  tablet: { width: 768, height: 1024 },
  desktop: { width: 1440, height: 900 },
} as const;

/** The three layouts, labelled, for specs that audit every breakpoint. */
export const BREAKPOINTS = Object.entries(VIEWPORTS).map(([label, size]) => ({
  label,
  size,
}));

export async function expectNoHorizontalOverflow(
  locator: Locator,
  label: string,
) {
  const bounds = await locator.evaluate(element => {
    const tolerance = 1;
    const rect = element.getBoundingClientRect();
    const clientWidth = document.documentElement.clientWidth;

    return {
      clientWidth,
      fits: rect.left >= -tolerance && rect.right <= clientWidth + tolerance,
      left: rect.left,
      right: rect.right,
      width: rect.width,
    };
  });

  expect(bounds, `${label} should fit within the viewport`).toMatchObject({
    fits: true,
  });
}

export async function expectPageHasNoHorizontalScroll(page: Page) {
  const dimensions = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));

  expect(
    dimensions.scrollWidth,
    `page should not scroll horizontally: ${JSON.stringify(dimensions)}`,
  ).toBeLessThanOrEqual(dimensions.clientWidth + 1);
}

/**
 * The page does not scroll sideways and no rendered element pokes past the
 * viewport's right edge, even one clipped by an `overflow: hidden` ancestor.
 */
export async function expectNoOverflowingElements(page: Page) {
  await expectPageHasNoHorizontalScroll(page);

  const offenders = await page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>("body *"))
      .filter(element => {
        const rect = element.getBoundingClientRect();
        return rect.width > 0 && rect.right > window.innerWidth + 1;
      })
      .slice(0, 5)
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        className: element.className.toString(),
        right: element.getBoundingClientRect().right,
      })),
  );

  expect(offenders).toEqual([]);
}

/** Presses Tab until `target` has focus, failing after `maxPresses`. */
export async function tabTo(page: Page, target: Locator, maxPresses = 40) {
  for (let presses = 0; presses < maxPresses; presses++) {
    await page.keyboard.press("Tab");
    if (await target.evaluate(element => element === document.activeElement)) {
      return;
    }
  }
  throw new Error(`Tab did not reach the target within ${maxPresses} presses`);
}

/** The card (an article) whose link is named exactly `name`. */
export function cardFor(page: Page, name: string) {
  return page.getByRole("article").filter({
    has: page.getByRole("link", { name, exact: true }),
  });
}
