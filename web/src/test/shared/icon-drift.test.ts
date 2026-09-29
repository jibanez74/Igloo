import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  ICON_TEXT_TARGETS,
  packIco,
  renderFaviconSvg,
  renderManifest,
} from "../../../scripts/generate-icons";
import { BRAND_MARK_PATH } from "@/lib/brand-mark";
import { tokenHex } from "@/lib/theme-tokens";
import { pngDimensions, readWebFile, resolveWeb } from "../helpers/public-files";

// public/favicon.svg and public/manifest.webmanifest are GENERATED from
// src/lib/theme-tokens.ts, src/lib/brand-mark.ts and src/lib/constants.ts by
// scripts/generate-icons.ts.
// This test fails when either file is edited by hand or a source changes
// without rerunning `bun run generate:icons`. The PNG and ICO rasters are
// screenshots whose bytes vary by Chromium version, so only the ICO container
// structure is checked here (the PNG sizes are covered by web-manifest.test.ts).

const PNG_SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

describe("generated icon files match their sources", () => {
  it.each(ICON_TEXT_TARGETS.map(target => [target.relPath, target] as const))(
    "%s is up to date (bun run generate:icons)",
    (relPath, target) => {
      expect(readWebFile(relPath)).toBe(target.render());
    },
  );
});

describe("favicon.svg", () => {
  const svg = renderFaviconSvg();

  it("uses only the dark canvas and glacier primary tokens", () => {
    const brand = [tokenHex("dark", "--background"), tokenHex("dark", "--primary")];
    const colors = [...new Set(svg.match(/#[0-9A-Fa-f]{6}/g))];
    expect(colors.length).toBeGreaterThan(0);
    for (const color of colors) {
      expect(brand, `${color} is not a brand token`).toContain(color);
    }
  });

  it("draws the shared brand glyph in the glacier primary", () => {
    expect(svg).toContain(`d="${BRAND_MARK_PATH}"`);
    expect(svg).toContain(`fill="${tokenHex("dark", "--primary")}" fill-rule="evenodd"`);
    expect(svg).toContain('aria-label="Igloo"');
  });
});

describe("manifest.webmanifest", () => {
  it("paints the splash and title bar with the dark canvas token", () => {
    const canvas = tokenHex("dark", "--background");
    expect(JSON.parse(renderManifest())).toMatchObject({
      background_color: canvas,
      theme_color: canvas,
    });
  });
});

describe("favicon.ico", () => {
  it("wraps 16 and 32 px PNG members", () => {
    const ico = readFileSync(resolveWeb("public/favicon.ico"));
    expect(ico.readUInt16LE(0)).toBe(0);
    expect(ico.readUInt16LE(2)).toBe(1);
    const count = ico.readUInt16LE(4);
    expect(count).toBe(2);

    const sizes: number[] = [];
    for (let index = 0; index < count; index++) {
      const at = 6 + 16 * index;
      const size = ico.readUInt8(at);
      const length = ico.readUInt32LE(at + 8);
      const offset = ico.readUInt32LE(at + 12);
      sizes.push(size);
      expect(offset + length).toBeLessThanOrEqual(ico.length);
      expect(ico.subarray(offset, offset + 8).equals(PNG_SIGNATURE)).toBe(true);
      expect(pngDimensions(ico, offset)).toEqual({ width: size, height: size });
    }
    expect(sizes).toEqual([16, 32]);
  });

  it("packs members back to back after the directory", () => {
    const first = new Uint8Array([1, 2, 3]);
    const second = new Uint8Array([4, 5, 6, 7, 8]);
    const ico = packIco([
      { size: 16, png: first },
      { size: 256, png: second },
    ]);

    expect(ico.length).toBe(6 + 16 * 2 + first.length + second.length);
    expect(ico.readUInt8(6)).toBe(16);
    expect(ico.readUInt32LE(6 + 8)).toBe(first.length);
    expect(ico.readUInt32LE(6 + 12)).toBe(38);
    // 256 px is stored as 0 in the one-byte edge fields.
    expect(ico.readUInt8(22)).toBe(0);
    expect(ico.readUInt32LE(22 + 12)).toBe(38 + first.length);
    expect([...ico.subarray(38)]).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
    expect(() => packIco([{ size: 512, png: first }])).toThrow(/1–256/);
  });
});
