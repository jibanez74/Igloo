import { expect, test, type Page } from "@playwright/test";
import { comparePngs, decodePng } from "./png";

// The exact comparison behind expectScreenshot (visual-helpers.ts) is only as
// good as its PNG decoder, so both are checked against the browser itself.
// No baselines here: every picture is drawn and compared within the test.

const SCREENSHOT = { animations: "disabled", caret: "hide", scale: "css" } as const;

/**
 * Two surfaces separated by a 1px line whose colour lies between theirs: the
 * shape Playwright's own comparator can take for anti-aliasing and skip.
 */
const surfaces = (line: string) => `
  <style>
    body { margin: 0; background: rgb(11, 18, 32); }
    .upper { height: 40px; background: rgb(21, 35, 58); }
    .line { height: 1px; background: ${line}; }
    .lower { height: 40px; background: rgb(11, 18, 32); }
    .text { font: 16px sans-serif; color: rgb(240, 245, 250); padding: 8px; }
  </style>
  <div class="upper"><div class="text">Surface</div></div>
  <div class="line"></div>
  <div class="lower"></div>
`;

async function shoot(page: Page, html: string) {
  await page.setViewportSize({ width: 200, height: 81 });
  await page.setContent(html);
  return page.screenshot(SCREENSHOT);
}

/** The RGBA bytes the browser itself decodes from a PNG, row by row. */
async function browserPixels(page: Page, png: Buffer) {
  return page.evaluate(async source => {
    const image = new Image();
    image.src = source;
    await image.decode();

    const canvas = document.createElement("canvas");
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    const context = canvas.getContext("2d")!;
    context.drawImage(image, 0, 0);

    const { data, width, height } = context.getImageData(
      0,
      0,
      canvas.width,
      canvas.height,
    );
    return { width, height, rgba: Array.from(data) };
  }, `data:image/png;base64,${png.toString("base64")}`);
}

test.describe("exact screenshot comparison", () => {
  test("decodes a screenshot to the pixels the browser painted", async ({
    page,
  }) => {
    const png = await shoot(page, surfaces("rgb(30, 48, 78)"));
    const decoded = decodePng(png);
    const painted = await browserPixels(page, png);

    expect([decoded.width, decoded.height]).toEqual([
      painted.width,
      painted.height,
    ]);

    const rgba: number[] = [];
    for (let pixel = 0; pixel < decoded.width * decoded.height; pixel += 1) {
      const offset = pixel * decoded.channels;
      rgba.push(
        decoded.data[offset],
        decoded.data[offset + 1],
        decoded.data[offset + 2],
        decoded.channels === 4 ? decoded.data[offset + 3] : 255,
      );
    }
    expect(rgba).toEqual(painted.rgba);
  });

  test("calls two shots of the same page the same", async ({ page }) => {
    const first = await shoot(page, surfaces("rgb(30, 48, 78)"));
    const second = await shoot(page, surfaces("rgb(30, 48, 78)"));

    expect(comparePngs(first, second)).toEqual({ kind: "same" });
  });

  test("counts every pixel of a 1px line whose tint moved", async ({ page }) => {
    const before = await shoot(page, surfaces("rgb(30, 48, 78)"));
    const after = await shoot(page, surfaces("rgb(31, 49, 79)"));

    // The whole line, and nothing else: 200 pixels on row 40.
    expect(comparePngs(before, after)).toEqual({
      kind: "pixels",
      count: 200,
      box: { left: 0, top: 40, right: 199, bottom: 40 },
    });
  });

  test("reports a size change instead of comparing pixels", async ({ page }) => {
    const before = await shoot(page, surfaces("rgb(30, 48, 78)"));
    await page.setViewportSize({ width: 200, height: 90 });
    const after = await page.screenshot(SCREENSHOT);

    expect(comparePngs(before, after)).toEqual({
      kind: "size",
      expected: "200x81",
      actual: "200x90",
    });
  });

  test("refuses a file that is not a PNG", () => {
    expect(() => decodePng(Buffer.from("not a png"))).toThrow("Not a PNG file");
  });
});
