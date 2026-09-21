import { describe, expect, it } from "vitest";
import {
  formatDate,
  formatRuntimeMinutes,
  formatSpokenRuntimeMinutes,
  formatSpokenTime,
  formatTimecode,
  formatTimeLeft,
  formatTrackDuration,
  nounForCount,
  parseCatalogDate,
} from "@/lib/format";

// format.ts keeps its own month names; mirror them here so a test expectation
// is built the same way the formatter builds its output.
const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

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

describe("formatTimeLeft", () => {
  it("splits the remainder into hours and minutes", () => {
    expect(formatTimeLeft(1890, 7560)).toEqual({
      text: "1 hr 35 min left",
      spoken: "1 hour 35 minutes left",
    });
    expect(formatTimeLeft(0, 8107)).toEqual({
      text: "2 hr 16 min left",
      spoken: "2 hours 16 minutes left",
    });
  });

  it("drops zero-valued fields", () => {
    expect(formatTimeLeft(0, 3600)).toEqual({
      text: "1 hr left",
      spoken: "1 hour left",
    });
    expect(formatTimeLeft(0, 7200)).toEqual({
      text: "2 hr left",
      spoken: "2 hours left",
    });
    expect(formatTimeLeft(0, 750)).toEqual({
      text: "13 min left",
      spoken: "13 minutes left",
    });
  });

  it("rounds up to the next whole minute", () => {
    expect(formatTimeLeft(0, 3601).text).toBe("1 hr 1 min left");
    expect(formatTimeLeft(0, 61)).toEqual({
      text: "2 min left",
      spoken: "2 minutes left",
    });
  });

  // Rounding the seconds first keeps a near-minute remainder out of the
  // seconds field, where it would read "60 sec left".
  it("only reaches seconds inside the last minute", () => {
    expect(formatTimeLeft(0, 59.5)).toEqual({
      text: "1 min left",
      spoken: "1 minute left",
    });
    expect(formatTimeLeft(0, 45)).toEqual({
      text: "45 sec left",
      spoken: "45 seconds left",
    });
    expect(formatTimeLeft(0, 1)).toEqual({
      text: "1 sec left",
      spoken: "1 second left",
    });
  });

  it("never reports less than one second", () => {
    expect(formatTimeLeft(7560, 7560).text).toBe("1 sec left");
    expect(formatTimeLeft(9000, 7560).text).toBe("1 sec left");
    expect(formatTimeLeft(0, Number.NaN).text).toBe("1 sec left");
    expect(formatTimeLeft(0, Number.POSITIVE_INFINITY).text).toBe("1 sec left");
  });
});

describe("formatDate", () => {
  it("keeps a date-only catalog date on its own day", () => {
    // Stored dates are date-only. Parsed as UTC midnight and read back locally
    // they land a day early anywhere west of UTC, which showed a show that
    // first aired 2024-03-01 as "February 29, 2024".
    expect(formatDate("2024-03-01")).toBe("March 1, 2024");
    expect(formatDate("2024-01-01")).toBe("January 1, 2024");
    expect(formatDate("2024-12-31")).toBe("December 31, 2024");
  });

  it("still formats a timestamp that carries a time", () => {
    // A timestamp is zone-aware, so the calendar day depends on the runner's
    // offset. Derive the expected day from the same instant rather than
    // loosening the assertion to a month range.
    const timestamp = "2024-03-01T18:30:00Z";
    const d = new Date(timestamp);
    const expected = `${MONTHS[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;

    expect(formatDate(timestamp)).toBe(expected);
  });
});

describe("parseCatalogDate", () => {
  it("reads a date-only value as a local calendar date", () => {
    // The year, not just the day, is at stake: "2024-01-01" parsed at UTC
    // midnight reports 2023 anywhere west of UTC, so a show's air range
    // started a year early.
    const d = parseCatalogDate("2024-01-01");

    expect(d.getFullYear()).toBe(2024);
    expect(d.getMonth()).toBe(0);
    expect(d.getDate()).toBe(1);
  });

  it("leaves a value carrying a time to the normal parser", () => {
    const timestamp = "2024-03-01T18:30:00Z";

    expect(parseCatalogDate(timestamp).getTime()).toBe(
      new Date(timestamp).getTime(),
    );
  });
});

describe("nounForCount", () => {
  const ALBUM = { singular: "album", plural: "albums" };

  it("says the singular for exactly one", () => {
    expect(nounForCount(1, ALBUM)).toBe("album");
  });

  it("says the plural for none and for many", () => {
    // Zero is the case the library headers hit first: "0 albums", not
    // "0 album".
    expect(nounForCount(0, ALBUM)).toBe("albums");
    expect(nounForCount(2, ALBUM)).toBe("albums");
  });

  it("uses the pair's own plural rather than appending an s", () => {
    expect(nounForCount(3, { singular: "person", plural: "people" })).toBe(
      "people",
    );
  });
});
