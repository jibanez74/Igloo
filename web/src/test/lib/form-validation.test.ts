import { describe, expect, it } from "vitest";
import {
  PLAYLIST_DESCRIPTION_MAX_LENGTH,
  PLAYLIST_NAME_MAX_LENGTH,
  USER_PASSWORD_MAX_BYTES,
  USER_PASSWORD_MIN_LENGTH,
} from "@/lib/constants";
import {
  isValidEmail,
  passwordRuleError,
  playlistFieldsError,
} from "@/lib/form-validation";

describe("passwordRuleError", () => {
  it("accepts a password inside both limits", () => {
    expect(passwordRuleError("x".repeat(USER_PASSWORD_MIN_LENGTH))).toBeNull();
  });

  it("counts the minimum in code points, so an emoji counts once", () => {
    const short = "😀".repeat(USER_PASSWORD_MIN_LENGTH - 1);
    expect(passwordRuleError(short)).toBe(
      `Password must be at least ${USER_PASSWORD_MIN_LENGTH} characters.`,
    );
  });

  it("caps the password in UTF-8 bytes", () => {
    // Each emoji is 4 bytes, so this clears the minimum but not the maximum.
    const long = "😀".repeat(USER_PASSWORD_MAX_BYTES / 4 + 1);
    expect(passwordRuleError(long)).toBe(
      `Password must be at most ${USER_PASSWORD_MAX_BYTES} UTF-8 bytes.`,
    );
  });

  it("words the message around the field label", () => {
    expect(passwordRuleError("short", "New password")).toBe(
      `New password must be at least ${USER_PASSWORD_MIN_LENGTH} characters.`,
    );
  });
});

describe("isValidEmail", () => {
  it("accepts an address with one @ and no whitespace", () => {
    expect(isValidEmail("a@b.c")).toBe(true);
    expect(isValidEmail("a@b")).toBe(true);
  });

  it("rejects whitespace, a missing @, and empty input", () => {
    expect(isValidEmail("a b@c")).toBe(false);
    expect(isValidEmail("ab.c")).toBe(false);
    expect(isValidEmail("")).toBe(false);
  });
});

describe("playlistFieldsError", () => {
  it("accepts a named playlist with or without a description", () => {
    expect(playlistFieldsError("Road Trip", "")).toBeNull();
    expect(playlistFieldsError("Road Trip", "Songs for the drive")).toBeNull();
  });

  it("requires a name that is more than whitespace", () => {
    expect(playlistFieldsError("   ", "")).toBe("Playlist name is required");
  });

  it("limits the trimmed name and description", () => {
    expect(
      playlistFieldsError(` ${"n".repeat(PLAYLIST_NAME_MAX_LENGTH)} `, ""),
    ).toBeNull();
    expect(
      playlistFieldsError("n".repeat(PLAYLIST_NAME_MAX_LENGTH + 1), ""),
    ).toBe(
      `Playlist name is too long (max ${PLAYLIST_NAME_MAX_LENGTH} characters)`,
    );
    expect(
      playlistFieldsError(
        "Road Trip",
        "d".repeat(PLAYLIST_DESCRIPTION_MAX_LENGTH + 1),
      ),
    ).toBe(
      `Playlist description is too long (max ${PLAYLIST_DESCRIPTION_MAX_LENGTH} characters)`,
    );
  });
});
