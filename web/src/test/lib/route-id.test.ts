import { describe, expect, it } from "vitest";
import { parseRouteId } from "@/lib/route-id";

describe("parseRouteId", () => {
  it("accepts a plain positive integer", () => {
    expect(parseRouteId("1")).toBe(1);
    expect(parseRouteId("401")).toBe(401);
  });

  it("rejects a value with trailing characters", () => {
    // parseInt would return 12 for each of these and load a real record for a
    // malformed URL.
    expect(parseRouteId("12abc")).toBeNull();
    expect(parseRouteId("12 ")).toBeNull();
    expect(parseRouteId("12.5")).toBeNull();
    expect(parseRouteId("12/../3")).toBeNull();
  });

  it("rejects zero, negatives, leading zeros, and non-numerics", () => {
    expect(parseRouteId("0")).toBeNull();
    expect(parseRouteId("-3")).toBeNull();
    expect(parseRouteId("007")).toBeNull();
    expect(parseRouteId("")).toBeNull();
    expect(parseRouteId("abc")).toBeNull();
  });

  it("rejects a value beyond the safe integer range", () => {
    expect(parseRouteId("9007199254740993")).toBeNull();
  });
});
