import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  extractGeneratedBlock,
  GENERATED_TARGETS,
} from "../../../scripts/generate-theme";
import { THEME_COLORS } from "@/lib/theme";
import { THEME_TOKENS, type ThemeName } from "@/lib/theme-tokens";
import { oklchToHex, parseOklchTokens } from "../helpers/color";

// The theme sync points (styles.css tokens, boot.css pre-hydration paint, the
// index.html anti-flash script) are GENERATED from src/lib/theme-tokens.ts by
// scripts/generate-theme.ts. This test fails when a generated block is edited
// by hand or the module changes without rerunning `bun run generate:theme`,
// and when a token's OKLCH value and hex disagree.
// Files are resolved from the vitest cwd (the web/ project root).
const read = (relPath: string) =>
  readFileSync(resolve(process.cwd(), relPath), "utf8");

describe("generated theme blocks match src/lib/theme-tokens.ts", () => {
  it.each(GENERATED_TARGETS.map((target) => [target.relPath, target] as const))(
    "%s is up to date (bun run generate:theme)",
    (relPath, target) => {
      expect(extractGeneratedBlock(read(relPath), target)).toBe(
        target.render(),
      );
    },
  );
});

describe("theme-tokens.ts is internally consistent", () => {
  const themes: ThemeName[] = ["light", "dark"];

  it.each(themes)("%s OKLCH values round-trip to their hex", (theme) => {
    for (const [name, token] of Object.entries(THEME_TOKENS[theme])) {
      if (!token.hex) continue; // alpha tokens carry no hex
      const parsed = parseOklchTokens(`${name}: ${token.value};`)[name];
      expect(parsed, `unparseable OKLCH for ${name} (${theme})`).toBeDefined();
      expect(oklchToHex(parsed), `${name} (${theme})`).toBe(token.hex);
    }
  });

  it("THEME_COLORS derives from the canvas tokens", () => {
    expect(THEME_COLORS.dark).toBe(THEME_TOKENS.dark["--background"].hex);
    expect(THEME_COLORS.light).toBe(THEME_TOKENS.light["--background"].hex);
  });
});
