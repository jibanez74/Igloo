import { describe, expect, it } from "vitest";
import {
  MOTION_DURATIONS_MS,
  SHADOW_SCALE,
  Z_INDEX_SCALE,
} from "@/lib/theme-tokens";
import { cn } from "@/lib/utils";

// cn() resolves conflicts between Tailwind classes. It has to be told about
// the scales the app adds to the theme (src/lib/theme-tokens.ts), or a variant
// that sets `shadow-floating` cannot be overridden and cannot override.
describe("cn with the app's theme scales", () => {
  it.each(Object.keys(SHADOW_SCALE))(
    "treats shadow-%s as a shadow size",
    name => {
      expect(cn("shadow-md", `shadow-${name}`)).toBe(`shadow-${name}`);
      expect(cn(`shadow-${name}`, "shadow-none")).toBe("shadow-none");
    },
  );

  it("keeps a shadow colour beside a scale shadow", () => {
    expect(cn("shadow-floating", "shadow-black/15")).toBe(
      "shadow-floating shadow-black/15",
    );
    expect(cn("shadow-primary/20", "shadow-glow")).toBe(
      "shadow-primary/20 shadow-glow",
    );
  });

  it.each(Object.keys(Z_INDEX_SCALE))("treats z-%s as a z-index", name => {
    expect(cn("z-50", `z-${name}`)).toBe(`z-${name}`);
    expect(cn(`z-${name}`, "z-10")).toBe("z-10");
  });

  it.each(Object.keys(MOTION_DURATIONS_MS))(
    "treats duration-%s as a transition duration",
    name => {
      expect(cn("duration-150", `duration-${name}`)).toBe(`duration-${name}`);
      expect(cn(`duration-${name}`, "duration-300")).toBe("duration-300");
    },
  );

  it("still resolves Tailwind's own scales", () => {
    expect(cn("p-2 p-4", "shadow-md shadow-xl", "z-10 z-50")).toBe(
      "p-4 shadow-xl z-50",
    );
  });
});
