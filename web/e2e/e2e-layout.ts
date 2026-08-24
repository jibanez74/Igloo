import { expect, type Locator, type Page } from "@playwright/test";

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
