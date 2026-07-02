import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  THEME_COLORS,
  THEME_STORAGE_KEY,
  THEME_TEXT_COLORS,
  type Theme,
} from "@/lib/theme";
import { extractRuleBody, oklchToHex, parseOklchTokens } from "./color";

// The canvas/text hexes and the theme storage key are intentionally duplicated
// across styles.css (OKLCH tokens), boot.css (pre-hydration paint), the
// index.html anti-flash script, and src/lib/theme.ts. This test pins them all
// to the theme.ts constants so an edit in one place can't silently drift.
// Resolved from the vitest cwd (the web/ project root).
const read = (relPath: string) =>
  readFileSync(resolve(process.cwd(), relPath), "utf8");

const stylesCss = read("src/assets/styles.css");
const bootCss = read("src/assets/boot.css");
const indexHtml = read("index.html");

const themes: Array<[Theme, string]> = [
  ["light", ":root"],
  ["dark", ".dark"],
];

const bootSelectors: Record<Theme, string> = {
  light: "html",
  dark: "html.dark",
};

describe("theme constants stay in sync across files", () => {
  it.each(themes)(
    "styles.css %s canvas/text tokens match THEME_COLORS",
    (theme, selector) => {
      const tokens = parseOklchTokens(extractRuleBody(stylesCss, selector));
      expect(oklchToHex(tokens["--background"])).toBe(THEME_COLORS[theme]);
      expect(oklchToHex(tokens["--foreground"])).toBe(THEME_TEXT_COLORS[theme]);
    },
  );

  it.each(themes)(
    "boot.css %s html rule matches THEME_COLORS",
    (theme) => {
      const body = extractRuleBody(bootCss, bootSelectors[theme]);
      const background = /background:\s*(#[0-9a-fA-F]{6})/.exec(body)?.[1];
      const color = /color:\s*(#[0-9a-fA-F]{6})/.exec(body)?.[1];
      expect(background?.toUpperCase()).toBe(THEME_COLORS[theme]);
      expect(color?.toUpperCase()).toBe(THEME_TEXT_COLORS[theme]);
    },
  );

  it("index.html anti-flash script uses the storage key and theme-color hexes", () => {
    expect(indexHtml).toContain(`localStorage.getItem("${THEME_STORAGE_KEY}")`);
    expect(indexHtml).toContain(THEME_COLORS.light);
    expect(indexHtml).toContain(THEME_COLORS.dark);
  });

  it("index.html <meta name=\"theme-color\"> defaults to the dark canvas", () => {
    const meta = /<meta name="theme-color" content="(#[0-9a-fA-F]{6})"/.exec(
      indexHtml,
    )?.[1];
    expect(meta?.toUpperCase()).toBe(THEME_COLORS.dark);
  });

  it("boot.css glacier gradient tint matches the dark --primary token", () => {
    // Both body gradients tint with the glacier primary as rgba(56, 189, 248, a).
    expect(bootCss).toContain("rgba(56, 189, 248");
    const darkTokens = parseOklchTokens(extractRuleBody(stylesCss, ".dark"));
    expect(oklchToHex(darkTokens["--primary"])).toBe("#38BDF8");
  });
});
