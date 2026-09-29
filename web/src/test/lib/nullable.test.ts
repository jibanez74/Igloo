import { describe, expect, it } from "vitest";
import { unwrapNormalizedString } from "@/lib/nullable";

describe("unwrapNormalizedString", () => {
  it("trims and lower-cases a valid wrapped or plain string", () => {
    expect(unwrapNormalizedString({ Valid: true, String: " High 10 " })).toBe(
      "high 10",
    );
    expect(unwrapNormalizedString("  SMPTE2084")).toBe("smpte2084");
  });

  it("is undefined for a missing or invalid value", () => {
    expect(unwrapNormalizedString({ Valid: false, String: "main" })).toBe(
      undefined,
    );
    expect(unwrapNormalizedString(null)).toBe(undefined);
    expect(unwrapNormalizedString(undefined)).toBe(undefined);
  });
});
