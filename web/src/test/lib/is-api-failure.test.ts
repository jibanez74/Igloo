import { describe, expect, it } from "vitest";
import {
  apiErrorMessage,
  isApiFailure,
  tooManyAttemptsMessage,
} from "@/lib/is-api-failure";

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
    expect(apiErrorMessage(undefined, "Couldn’t load.")).toBe(
      "Couldn’t load.",
    );
  });

  it("falls back for a success envelope, which has no error to report", () => {
    expect(apiErrorMessage({ error: false, data: {} }, "Fallback.")).toBe(
      "Fallback.",
    );
  });

  it("falls back when the failure carries a blank message", () => {
    expect(apiErrorMessage({ error: true, message: "" }, "Fallback.")).toBe(
      "Fallback.",
    );
  });

  it("keeps the server's message for a client error", () => {
    expect(
      apiErrorMessage({ error: true, message: "Playlist is full.", status: 409 }, "Fallback."),
    ).toBe("Playlist is full.");
  });

  // apiRequest answers every 404 with its canned "404 - …" envelope.
  it("falls back for a not-found", () => {
    expect(
      apiErrorMessage(
        { error: true, message: "404 - The resource you requested was not found.", status: 404 },
        "Fallback.",
      ),
    ).toBe("Fallback.");
  });

  // A 5xx message is the server's internal error text, not copy for a person.
  it("falls back for a server error", () => {
    expect(
      apiErrorMessage(
        { error: true, message: "failed to add movies to playlist", status: 500 },
        "Couldn’t load playlists.",
      ),
    ).toBe("Couldn’t load playlists.");
  });
});

describe("rate-limit copy", () => {
  it.each([
    [1, "Too many attempts. Wait 1 second."],
    [30, "Too many attempts. Wait 30 seconds."],
    [60, "Too many attempts. Wait 1 minute."],
    [61, "Too many attempts. Wait 2 minutes."],
    [240, "Too many attempts. Wait 4 minutes."],
    [300, "Too many attempts. Wait 5 minutes."],
    [undefined, "Too many attempts."],
    [0, "Too many attempts."],
  ])("words a %s-second wait", (retryAfterSec, copy) => {
    expect(tooManyAttemptsMessage(retryAfterSec)).toBe(copy);
  });

  // The server's lowercase constant prescribes a retry (design-system §3.4).
  it("replaces the server's message on a 429", () => {
    expect(
      apiErrorMessage(
        {
          error: true,
          message: "too many attempts, please try again later",
          status: 429,
          retryAfterSec: 240,
        },
        "Fallback.",
      ),
    ).toBe("Too many attempts. Wait 4 minutes.");
  });

  it("words a 429 with no known wait, such as a proxy's", () => {
    expect(
      apiErrorMessage(
        { error: true, message: "429 - The request could not be completed.", status: 429 },
        "Fallback.",
      ),
    ).toBe("Too many attempts.");
  });
});
