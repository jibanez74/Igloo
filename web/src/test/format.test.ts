import { describe, expect, it } from "vitest";
import {
  formatRuntimeMinutes,
  formatSpokenRuntimeMinutes,
} from "@/lib/format";

describe("formatRuntimeMinutes", () => {
  it("returns null for empty, non-finite, or non-positive runtimes", () => {
    expect(formatRuntimeMinutes(null)).toBeNull();
    expect(formatRuntimeMinutes(undefined)).toBeNull();
    expect(formatRuntimeMinutes(Number.NaN)).toBeNull();
    expect(formatRuntimeMinutes(Number.POSITIVE_INFINITY)).toBeNull();
    expect(formatRuntimeMinutes(0)).toBeNull();
    expect(formatRuntimeMinutes(-1)).toBeNull();
  });

  it("formats minute runtimes", () => {
    expect(formatRuntimeMinutes(45)).toBe("45 min");
    expect(formatRuntimeMinutes(60)).toBe("1 hr");
    expect(formatRuntimeMinutes(116)).toBe("1 hr 56 min");
    expect(formatRuntimeMinutes(120)).toBe("2 hr");
  });

  it("floors fractional runtimes before formatting", () => {
    expect(formatRuntimeMinutes(116.75)).toBe("1 hr 56 min");
    expect(formatRuntimeMinutes(0.75)).toBe("");
  });
});

describe("formatSpokenRuntimeMinutes", () => {
  it("returns null for empty, non-finite, or non-positive runtimes", () => {
    expect(formatSpokenRuntimeMinutes(null)).toBeNull();
    expect(formatSpokenRuntimeMinutes(undefined)).toBeNull();
    expect(formatSpokenRuntimeMinutes(Number.NaN)).toBeNull();
    expect(formatSpokenRuntimeMinutes(Number.POSITIVE_INFINITY)).toBeNull();
    expect(formatSpokenRuntimeMinutes(0)).toBeNull();
    expect(formatSpokenRuntimeMinutes(-1)).toBeNull();
  });

  it("formats runtime words for screen readers", () => {
    expect(formatSpokenRuntimeMinutes(1)).toBe("1 minute");
    expect(formatSpokenRuntimeMinutes(45)).toBe("45 minutes");
    expect(formatSpokenRuntimeMinutes(60)).toBe("1 hour");
    expect(formatSpokenRuntimeMinutes(61)).toBe("1 hour 1 minute");
    expect(formatSpokenRuntimeMinutes(116)).toBe("1 hour 56 minutes");
    expect(formatSpokenRuntimeMinutes(120)).toBe("2 hours");
  });

  it("floors fractional runtimes before formatting words", () => {
    expect(formatSpokenRuntimeMinutes(116.75)).toBe("1 hour 56 minutes");
    expect(formatSpokenRuntimeMinutes(0.75)).toBe("");
  });
});
