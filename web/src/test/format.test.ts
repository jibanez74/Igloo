import { describe, expect, it } from "vitest";
import { formatRuntimeMinutes } from "@/lib/format";

describe("formatRuntimeMinutes", () => {
  it("returns null for empty or non-positive runtimes", () => {
    expect(formatRuntimeMinutes(null)).toBeNull();
    expect(formatRuntimeMinutes(undefined)).toBeNull();
    expect(formatRuntimeMinutes(0)).toBeNull();
  });

  it("formats minute runtimes", () => {
    expect(formatRuntimeMinutes(45)).toBe("45m");
    expect(formatRuntimeMinutes(116)).toBe("1h 56m");
  });
});
