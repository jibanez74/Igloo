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

  describe("reports whether the write persisted", () => {
    it("returns true when localStorage accepts the write", () => {
      expect(setDevicePlaybackPreferences(USER, { downloadMbps: 15 })).toBe(
        true,
      );
    });

    it("returns false but still applies the value when storage throws", () => {
      const setItem = vi
        .spyOn(Storage.prototype, "setItem")
        .mockImplementation(() => {
          throw new Error("quota exceeded");
        });

      expect(setDevicePlaybackPreferences(USER, { downloadMbps: 15 })).toBe(
        false,
      );
      expect(getDevicePlaybackPreferences(USER).downloadMbps).toBe(15);

      setItem.mockRestore();
    });

    // Nothing reached storage, so a later patch has to merge against the
    // session snapshot -- re-reading storage would drop the earlier value.
    it("keeps session-only values when a later patch is applied", () => {
      const setItem = vi
        .spyOn(Storage.prototype, "setItem")
        .mockImplementation(() => {
          throw new Error("quota exceeded");
        });

      setDevicePlaybackPreferences(USER, { downloadMbps: 15 });
      setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "eng" });
      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        downloadMbps: 15,
        preferredAudioLanguage: "eng",
      });

      setItem.mockRestore();
    });
  });

  // A second tab writes to the same key. Without this the module would merge
  // patches against a stale snapshot and write the other tab's field away.
  describe("synchronizes across tabs", () => {
    function otherTabWrites(
      userId: number,
      value: Partial<Record<string, unknown>>,
    ) {
      const key = storageKeyForUser(userId);
      const newValue = JSON.stringify(value);
      localStorage.setItem(key, newValue);
      window.dispatchEvent(new StorageEvent("storage", { key, newValue }));
    }

    it("picks up another tab's write and notifies subscribers", () => {
      const listener = vi.fn();
      const unsubscribe = subscribeDevicePlaybackPreferences(listener);

      otherTabWrites(USER, { preferredAudioLanguage: "fra" });

      expect(listener).toHaveBeenCalledTimes(1);
      expect(getDevicePlaybackPreferences(USER).preferredAudioLanguage).toBe(
        "fra",
      );

      unsubscribe();
    });

    it("preserves the other tab's field when merging a later patch", () => {
      const unsubscribe = subscribeDevicePlaybackPreferences(() => {});
      // This tab reads first, so it holds a snapshot with no profile.
      expect(getDevicePlaybackPreferences(USER).preferredProfile).toBeNull();

      otherTabWrites(USER, { preferredProfile: "1080p_8mbps" });
      setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "eng" });

      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        preferredProfile: "1080p_8mbps",
        preferredAudioLanguage: "eng",
      });

      unsubscribe();
    });

    it("merges against storage even before the event is delivered", () => {
      const unsubscribe = subscribeDevicePlaybackPreferences(() => {});
      expect(getDevicePlaybackPreferences(USER).preferredProfile).toBeNull();

      // No StorageEvent: the browser delivers it asynchronously, so the write
      // path cannot rely on having seen it.
      localStorage.setItem(
        storageKeyForUser(USER),
        JSON.stringify({ preferredProfile: "1080p_8mbps" }),
      );
      setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "eng" });

      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        preferredProfile: "1080p_8mbps",
        preferredAudioLanguage: "eng",
      });

      unsubscribe();
    });

    it("drops every snapshot when another tab clears storage", () => {
      const unsubscribe = subscribeDevicePlaybackPreferences(() => {});
      setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "eng" });

      localStorage.clear();
      window.dispatchEvent(new StorageEvent("storage", { key: null }));

      expect(getDevicePlaybackPreferences(USER)).toEqual(
        DEFAULT_DEVICE_PLAYBACK_PREFERENCES,
      );

      unsubscribe();
    });

    it("ignores writes to unrelated keys", () => {
      const listener = vi.fn();
      const unsubscribe = subscribeDevicePlaybackPreferences(listener);

      window.dispatchEvent(
        new StorageEvent("storage", { key: "igloo-theme", newValue: "light" }),
      );

      expect(listener).not.toHaveBeenCalled();

      unsubscribe();
    });

    it("stops listening once the last subscriber unsubscribes", () => {
      const listener = vi.fn();
      subscribeDevicePlaybackPreferences(listener)();

      otherTabWrites(USER, { preferredAudioLanguage: "fra" });

      expect(listener).not.toHaveBeenCalled();
    });

    // With no listener attached nothing invalidates the cache, so a snapshot
    // held from before can only be stale -- and non-subscribing readers such as
    // the movie play loader would serve it.
    it("re-parses storage for a read after the last subscriber leaves", () => {
      const unsubscribe = subscribeDevicePlaybackPreferences(() => {});
      expect(getDevicePlaybackPreferences(USER).preferredAudioLanguage).toBe(
        null,
      );
      unsubscribe();

      // No StorageEvent: with the listener gone, nothing would deliver it.
      localStorage.setItem(
        storageKeyForUser(USER),
        JSON.stringify({ preferredAudioLanguage: "fra" }),
      );

      expect(getDevicePlaybackPreferences(USER).preferredAudioLanguage).toBe(
        "fra",
      );
    });

    // The exception: a value that never reached storage lives only in the
    // snapshot, so re-reading would return what storage never took.
    it("keeps a session-only value across the last unsubscribe", () => {
      const setItem = vi
        .spyOn(Storage.prototype, "setItem")
        .mockImplementation(() => {
          throw new Error("quota exceeded");
        });

      const unsubscribe = subscribeDevicePlaybackPreferences(() => {});
      setDevicePlaybackPreferences(USER, { downloadMbps: 15 });
      unsubscribe();

      expect(getDevicePlaybackPreferences(USER).downloadMbps).toBe(15);
      setDevicePlaybackPreferences(USER, { preferredAudioLanguage: "eng" });
      expect(getDevicePlaybackPreferences(USER)).toMatchObject({
        downloadMbps: 15,
        preferredAudioLanguage: "eng",
      });

      setItem.mockRestore();
    });
  });
});
