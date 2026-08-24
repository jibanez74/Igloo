import { describe, expect, it } from "vitest";
import {
  formatRuntimeMinutes,
  formatSpokenRuntimeMinutes,
  formatSpokenTime,
  formatTimecode,
  formatTrackDuration,
} from "@/lib/format";

describe("formatTrackDuration", () => {
  it("returns an empty string for missing or invalid durations", () => {
    expect(formatTrackDuration(0)).toBe("");
    expect(formatTrackDuration(-1)).toBe("");
    expect(formatTrackDuration(Number.NaN)).toBe("");
    expect(formatTrackDuration(Number.POSITIVE_INFINITY)).toBe("");
  });

  it("formats millisecond durations as m:ss", () => {
    expect(formatTrackDuration(225000)).toBe("3:45");
    expect(formatTrackDuration(61000)).toBe("1:01");
  });

  it("floors partial seconds", () => {
    expect(formatTrackDuration(200700)).toBe("3:20");
  });
});

describe("formatTimecode", () => {
  it("returns 0:00 for non-finite or negative input", () => {
    expect(formatTimecode(Number.NaN)).toBe("0:00");
    expect(formatTimecode(Number.POSITIVE_INFINITY)).toBe("0:00");
    expect(formatTimecode(-1)).toBe("0:00");
    expect(formatTimecode(-1, { forceHours: true })).toBe("0:00");
  });

  it("formats sub-hour values as m:ss", () => {
    expect(formatTimecode(0)).toBe("0:00");
    expect(formatTimecode(83)).toBe("1:23");
    expect(formatTimecode(3599)).toBe("59:59");
  });

  it("includes the hours field past one hour", () => {
    expect(formatTimecode(3600)).toBe("1:00:00");
    expect(formatTimecode(3923)).toBe("1:05:23");
    expect(formatTimecode(7500)).toBe("2:05:00");
  });

  it("pads sub-hour values to h:mm:ss when forceHours is set", () => {
    expect(formatTimecode(300, { forceHours: true })).toBe("0:05:00");
    expect(formatTimecode(12, { forceHours: true })).toBe("0:00:12");
    expect(formatTimecode(3923, { forceHours: true })).toBe("1:05:23");
  });

  it("floors fractional seconds", () => {
    expect(formatTimecode(89.9)).toBe("1:29");
  });
});

describe("formatSpokenTime", () => {
  it("returns 0 seconds for non-finite, negative, or zero input", () => {
    expect(formatSpokenTime(Number.NaN)).toBe("0 seconds");
    expect(formatSpokenTime(-5)).toBe("0 seconds");
    expect(formatSpokenTime(0)).toBe("0 seconds");
  });

  it("formats playback positions as words, dropping zero fields", () => {
    expect(formatSpokenTime(1)).toBe("1 second");
    expect(formatSpokenTime(330)).toBe("5 minutes 30 seconds");
    expect(formatSpokenTime(3600)).toBe("1 hour");
    expect(formatSpokenTime(3923)).toBe("1 hour 5 minutes 23 seconds");
  });
});

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
    expect(formatRuntimeMinutes(0.75)).toBeNull();
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
    expect(formatSpokenRuntimeMinutes(0.75)).toBeNull();
  });
});
