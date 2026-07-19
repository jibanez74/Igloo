import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  contrastRatio,
  extractRuleBody,
  parseOklchTokens,
  type Oklch,
} from "../helpers/color";

// Guards both igloo themes against accessibility regressions: every key
// text/surface pair must meet WCAG AA (and AAA for body text). The token values
// live in styles.css and are parsed here so the test tracks the real palette.
// Resolved from the vitest cwd (the web/ project root).
const cssPath = resolve(process.cwd(), "src/assets/styles.css");
const css = readFileSync(cssPath, "utf8");

const themes: Record<string, Record<string, Oklch>> = {
  ":root (light)": parseOklchTokens(extractRuleBody(css, ":root")),
  ".dark": parseOklchTokens(extractRuleBody(css, ".dark")),
};

// [foreground token, background token, min ratio]
const pairs: Array<[string, string, number]> = [
  ["--foreground", "--background", 7], // body text — AAA
  ["--card-foreground", "--card", 4.5],
  ["--popover-foreground", "--popover", 4.5],
  ["--muted-foreground", "--background", 4.5],
  ["--primary-foreground", "--primary", 4.5],
  ["--aurora-foreground", "--aurora", 4.5],
  ["--secondary-foreground", "--secondary", 4.5],
  ["--accent-foreground", "--accent", 4.5],
  ["--destructive-foreground", "--destructive", 4.5],
  ["--success-foreground", "--success", 4.5],
  ["--success", "--background", 4.5], // text-success on the canvas (§4.8)
  ["--success", "--card", 4.5], // text-success on raised surfaces
  ["--accent-teal-foreground", "--accent-teal", 4.5],
  ["--sidebar-foreground", "--sidebar", 7], // sidebar body text — AAA
  ["--sidebar-primary-foreground", "--sidebar-primary", 4.5],
  ["--sidebar-accent-foreground", "--sidebar-accent", 4.5],
];

describe.each(Object.entries(themes))("contrast — %s", (_name, tokens) => {
  it.each(pairs)("%s on %s >= %f:1", (fg, bg, min) => {
    const fgColor = tokens[fg];
    const bgColor = tokens[bg];
    expect(fgColor, `missing token ${fg}`).toBeDefined();
    expect(bgColor, `missing token ${bg}`).toBeDefined();
    const ratio = contrastRatio(fgColor, bgColor);
    expect(ratio).toBeGreaterThanOrEqual(min);
  });
});
