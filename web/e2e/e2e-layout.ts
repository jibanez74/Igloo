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

/** The layouts a responsive audit walks: the three VIEWPORTS and two in between. */
export const AUDIT_VIEWPORTS = [
  { name: "small phone", width: 360, height: 800 },
  { name: "phone", ...VIEWPORTS.phone },
  { name: "tablet portrait", ...VIEWPORTS.tablet },
  { name: "tablet landscape", width: 1024, height: 768 },
  { name: "desktop", ...VIEWPORTS.desktop },
] as const;

type ResponsiveAuditOptions = {
  /** Brings the page to the state under audit at the current viewport. */
  ready: () => Promise<void>;
  /** Accessible names that must each belong to a visible control. */
  requiredNames?: readonly string[];
  /** Fails on visible text smaller than this many CSS pixels. */
  minFontPx?: number;
  viewports?: readonly { name: string; width: number; height: number }[];
};

/**
 * At each viewport: the page does not scroll sideways, no visible element
 * sits past either edge, every visible control has an accessible name, and
 * every required name belongs to one. The name is worked out the way the
 * app's markup gives it (aria-labelledby, aria-label, a label, the text, a
 * placeholder or a title), not by the full accessible-name algorithm.
 */
export async function auditResponsiveControls(
  page: Page,
  { ready, requiredNames = [], minFontPx, viewports = AUDIT_VIEWPORTS }: ResponsiveAuditOptions,
) {
  for (const viewport of viewports) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await ready();
    await expectPageHasNoHorizontalScroll(page);

    const audit = await page.evaluate(
      ({ requiredNames, minFontPx }) => {
        const isVisible = (element: Element) => {
          const style = window.getComputedStyle(element);
          const box = element.getBoundingClientRect();
          return (
            style.visibility !== "hidden" &&
            style.display !== "none" &&
            box.width > 0 &&
            box.height > 0
          );
        };

        const accessibleName = (element: Element) => {
          const labelledBy = (element.getAttribute("aria-labelledby") ?? "")
            .split(/\s+/)
            .filter(Boolean)
            .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
            .filter(Boolean)
            .join(" ");
          if (labelledBy) return labelledBy;

          const ariaLabel = element.getAttribute("aria-label")?.trim();
          if (ariaLabel) return ariaLabel;

          if (element.id) {
            const label = document.querySelector(`label[for="${CSS.escape(element.id)}"]`);
            const labelText = label?.textContent?.trim();
            if (labelText) return labelText;
          }

          const wrappingLabel = element.closest("label")?.textContent?.trim();
          if (wrappingLabel) return wrappingLabel;

          const text = element.textContent?.trim();
          if (text) return text;

          return (
            element.getAttribute("placeholder")?.trim() ||
            element.getAttribute("title")?.trim() ||
            ""
          );
        };

        const describe = (element: Element) => ({
          tag: element.tagName.toLowerCase(),
          role: element.getAttribute("role"),
          text: element.textContent?.trim().slice(0, 60) ?? "",
        });

        const controls = Array.from(
          document.querySelectorAll(
            [
              "a[href]",
              "button",
              'input:not([type="hidden"])',
              "select",
              "textarea",
              '[role="button"]',
              '[role="tab"]',
              '[role="textbox"]',
              '[tabindex]:not([tabindex="-1"])',
            ].join(", "),
          ),
        ).filter(isVisible);
        const names = controls.map(accessibleName);

        const smallText =
          minFontPx === undefined
            ? []
            : Array.from(document.querySelectorAll("label, p, span, button, a, input"))
                .filter(isVisible)
                .map(element => ({
                  text:
                    element.textContent?.trim() || element.getAttribute("placeholder") || "",
                  fontSize: Number.parseFloat(getComputedStyle(element).fontSize),
                }))
                .filter(item => item.text && item.fontSize < minFontPx);

        return {
          missing: requiredNames.filter(name => !names.includes(name)),
          unlabeled: controls.filter((_, index) => !names[index]).map(describe),
          offscreen: Array.from(document.querySelectorAll("body, body *"))
            .filter(isVisible)
            .filter(element => {
              const box = element.getBoundingClientRect();
              return box.left < -1 || box.right > window.innerWidth + 1;
            })
            .map(describe),
          smallText,
        };
      },
      { requiredNames: [...requiredNames], minFontPx },
    );

    expect(audit.missing, `${viewport.name}: required controls missing`).toEqual([]);
    expect(audit.unlabeled, `${viewport.name}: controls without a name`).toEqual([]);
    expect(audit.offscreen, `${viewport.name}: elements past the viewport`).toEqual([]);
    expect(audit.smallText, `${viewport.name}: text under ${minFontPx}px`).toEqual([]);
  }
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

/**
 * `hero` starts exactly where the app header ends and is as wide as the page
 * area: a detail page's hero cancels the shell's padding at the top and at
 * both sides, so no strip of page background shows around it.
 */
export async function expectFlushUnderAppHeader(page: Page, hero: Locator) {
  // Retried: the page's entrance animation slides the hero into place, and a
  // viewport change relays the page out over a frame or two.
  await expect(async () => {
    const [header, main, box] = await Promise.all([
      page.getByRole("banner").boundingBox(),
      page.getByRole("main").boundingBox(),
      hero.boundingBox(),
    ]);
    if (!header || !main || !box) {
      throw new Error("the app header, main and the hero must all be rendered");
    }

    expect(box.y, "hero top meets the header's bottom edge").toBeCloseTo(
      header.y + header.height,
      0,
    );
    expect(box.x, "hero starts at the page area's left edge").toBeCloseTo(
      main.x,
      0,
    );
    expect(box.width, "hero spans the page area").toBeCloseTo(main.width, 0);
  }).toPass({ timeout: 5000 });
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
