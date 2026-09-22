import { describe, expect, it } from "vitest";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";

describe("isApiFailure", () => {
  it("accepts a failure envelope", () => {
    expect(isApiFailure({ error: true, message: "Nope." })).toBe(true);
  });

  it("rejects a success envelope, even one carrying a message", () => {
    expect(isApiFailure({ error: false, data: {} })).toBe(false);
    expect(isApiFailure({ error: false, message: "Saved." })).toBe(false);
  });

  // A failed request never produces an envelope at all.
  it("rejects what is not an envelope", () => {
    expect(isApiFailure(undefined)).toBe(false);
    expect(isApiFailure(null)).toBe(false);
    expect(isApiFailure("error")).toBe(false);
    expect(isApiFailure({ error: true })).toBe(false);
    expect(isApiFailure({ error: true, message: 500 })).toBe(false);
  });
});

describe("apiErrorMessage", () => {
  it("prefers the server's own message", () => {
    expect(
      apiErrorMessage({ error: true, message: "Library is rescanning." }, "…"),
    ).toBe("Library is rescanning.");
  });

  it("falls back when the request never reached the server", () => {
    expect(apiErrorMessage(undefined, "Check your connection.")).toBe(
      "Check your connection.",
    );
  });

  it("falls back for a success envelope, which has no error to report", () => {
    expect(apiErrorMessage({ error: false, data: {} }, "Fallback.")).toBe(
      "Fallback.",
    );
  });
});
