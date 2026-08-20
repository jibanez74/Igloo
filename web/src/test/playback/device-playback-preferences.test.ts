import { beforeEach, describe, expect, it, vi } from "vitest";
import { SUBTITLE_OFF_VALUE } from "@/lib/constants";
import {
  DEFAULT_DEVICE_PLAYBACK_PREFERENCES,
  getDevicePlaybackPreferences,
  resetDevicePlaybackPreferencesCache,
  setDevicePlaybackPreferences,
  storageKeyForUser,
  subscribeDevicePlaybackPreferences,
} from "@/lib/playback-preferences";

const USER = 1;

function writeRaw(userId: number, raw: string) {
  localStorage.setItem(storageKeyForUser(userId), raw);
  resetDevicePlaybackPreferencesCache();
}

describe("device playback preferences", () => {
  beforeEach(() => {
    localStorage.clear();
    resetDevicePlaybackPreferencesCache();
  });

  // web/e2e/playback-settings.spec.ts mirrors this key format as a literal.
  it("scopes the storage key per user", () => {
    expect(storageKeyForUser(1)).toBe("igloo-playback-prefs:1");
    expect(storageKeyForUser(42)).toBe("igloo-playback-prefs:42");
  });

  it("defaults to no preferences when nothing is stored", () => {
    expect(getDevicePlaybackPreferences(USER)).toEqual(
      DEFAULT_DEVICE_PLAYBACK_PREFERENCES,
    );
  });

  it("round-trips a full set of preferences", () => {
    setDevicePlaybackPreferences(USER, {
      preferredProfile: "1080p_6mbps",
      downloadMbps: 50,
      preferredAudioLanguage: "spa",
      preferredSubtitleLanguage: SUBTITLE_OFF_VALUE,
    });
    resetDevicePlaybackPreferencesCache();

    expect(getDevicePlaybackPreferences(USER)).toEqual({
      preferredProfile: "1080p_6mbps",
      downloadMbps: 50,
      preferredAudioLanguage: "spa",
      preferredSubtitleLanguage: SUBTITLE_OFF_VALUE,
    });
  });

  it("merges a patch without clearing the other fields", () => {
    setDevicePlaybackPreferences(USER, { downloadMbps: 25 });
    setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "en" });

    expect(getDevicePlaybackPreferences(USER)).toMatchObject({
      downloadMbps: 25,
      preferredAudioLanguage: "en",
    });
  });

  it("keeps each user's preferences separate on the same device", () => {
    setDevicePlaybackPreferences(1, { preferredAudioLanguage: "spa" });
    setDevicePlaybackPreferences(2, { preferredAudioLanguage: "fra" });

    expect(getDevicePlaybackPreferences(1).preferredAudioLanguage).toBe("spa");
    expect(getDevicePlaybackPreferences(2).preferredAudioLanguage).toBe("fra");
  });

  it("notifies subscribers on write and stops after unsubscribe", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeDevicePlaybackPreferences(listener);

    setDevicePlaybackPreferences(USER, { downloadMbps: 10 });
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    setDevicePlaybackPreferences(USER, { downloadMbps: 20 });
    expect(listener).toHaveBeenCalledTimes(1);
  });

  // useSyncExternalStore compares snapshots by reference: a fresh object per
  // read would re-render forever.
  it("returns a stable snapshot until something writes", () => {
    const first = getDevicePlaybackPreferences(USER);
    expect(getDevicePlaybackPreferences(USER)).toBe(first);

    setDevicePlaybackPreferences(USER, { downloadMbps: 30 });
    expect(getDevicePlaybackPreferences(USER)).not.toBe(first);
  });

  describe("sanitizes untrusted storage", () => {
    it.each([
      ["not json at all", "{oh no"],
      ["a bare string", '"hello"'],
      ["an array", "[1,2,3]"],
      ["null", "null"],
    ])("falls back to defaults for %s", (_label, raw) => {
      writeRaw(USER, raw);
      expect(getDevicePlaybackPreferences(USER)).toEqual(
        DEFAULT_DEVICE_PLAYBACK_PREFERENCES,
      );
    });

    it.each([
      ["direct", "direct"],
      ["remux", "remux"],
      ["an unknown id", "4320p_99mbps"],
      ["a non-string", 7],
    ])("rejects %s as a preferred profile", (_label, value) => {
      writeRaw(USER, JSON.stringify({ preferredProfile: value }));
      expect(getDevicePlaybackPreferences(USER).preferredProfile).toBeNull();
    });

    it.each([
      ["zero", 0],
      ["negative", -5],
      ["the ceiling", 10_000],
      ["a string", "50"],
      ["NaN", null],
    ])("rejects %s as a download speed", (_label, value) => {
      writeRaw(USER, JSON.stringify({ downloadMbps: value }));
      expect(getDevicePlaybackPreferences(USER).downloadMbps).toBeNull();
    });

    it("keeps a download speed inside the accepted range", () => {
      writeRaw(USER, JSON.stringify({ downloadMbps: 9999 }));
      expect(getDevicePlaybackPreferences(USER).downloadMbps).toBe(9999);
    });

    it.each([
      ["off", SUBTITLE_OFF_VALUE],
      ["uppercase", "ENG"],
      ["too short", "e"],
      ["too long", "engl"],
    ])("rejects %s as an audio language", (_label, value) => {
      writeRaw(USER, JSON.stringify({ preferredAudioLanguage: value }));
      expect(
        getDevicePlaybackPreferences(USER).preferredAudioLanguage,
      ).toBeNull();
    });

    it("accepts off as a subtitle language but not as an audio language", () => {
      writeRaw(
        USER,
        JSON.stringify({
          preferredAudioLanguage: "eng",
          preferredSubtitleLanguage: SUBTITLE_OFF_VALUE,
        }),
      );
      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        preferredAudioLanguage: "eng",
        preferredSubtitleLanguage: SUBTITLE_OFF_VALUE,
      });
    });

    it("drops unknown keys", () => {
      writeRaw(
        USER,
        JSON.stringify({ downloadMbps: 40, somethingElse: "nope" }),
      );
      expect(getDevicePlaybackPreferences(USER)).toEqual({
        preferredProfile: null,
        downloadMbps: 40,
        preferredAudioLanguage: null,
        preferredSubtitleLanguage: null,
      });
    });

    it("re-sanitizes values written through the setter", () => {
      setDevicePlaybackPreferences(USER, {
        preferredProfile: "remux",
        downloadMbps: -1,
      });
      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        preferredProfile: null,
        downloadMbps: null,
      });
    });
  });

  it("still applies the value for this session when storage throws", () => {
    const setItem = vi
      .spyOn(Storage.prototype, "setItem")
      .mockImplementation(() => {
        throw new Error("quota exceeded");
      });

    setDevicePlaybackPreferences(USER, { downloadMbps: 15 });
    expect(getDevicePlaybackPreferences(USER).downloadMbps).toBe(15);

    setItem.mockRestore();
  });
});
