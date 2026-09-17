// Per-device playback preferences, persisted to localStorage.
//
// These are deliberately NOT stored on the server: one account streams from
// devices with different screens, decoders and network links, so a preferred
// profile or download speed that is right on a TV is wrong on a phone. The key
// is scoped per user as well as per device, because a browser profile can be
// shared by several people and the language preferences are personal.
//
// Follows the module shape of src/lib/theme.ts: a module-level cached snapshot
// plus a listener set, consumed through useSyncExternalStore.

import {
  DOWNLOAD_SPEED_MAX_MBPS,
  LANGUAGE_CODE_PATTERN,
  STREAM_MODES,
  SUBTITLE_OFF_VALUE,
} from "./constants";
import type { DevicePlaybackPreferences } from "@/types";

// Mirrored as a literal by web/e2e/playback-settings.spec.ts, which cannot
// import from src; playback-preferences.test.ts pins the format.
const PLAYBACK_PREFERENCES_STORAGE_PREFIX = "igloo-playback-prefs:";

export const DEFAULT_DEVICE_PLAYBACK_PREFERENCES: DevicePlaybackPreferences = {
  preferredProfile: null,
  downloadMbps: null,
  preferredAudioLanguage: null,
  preferredSubtitleLanguage: null,
};

// The profiles a client may pick as a default, mirroring the server's catalog:
// transcode profiles only, so neither "direct" nor "remux" can be stored here.
// Module scope: runs once at import over a handful of constant entries.
// react-doctor-disable-next-line react-doctor/js-combine-iterations
const SELECTABLE_PROFILE_IDS: readonly string[] = STREAM_MODES.filter(
  mode => mode.type === "transcode",
).map(mode => mode.id);

export function storageKeyForUser(userId: number): string {
  return `${PLAYBACK_PREFERENCES_STORAGE_PREFIX}${userId}`;
}

function browserLocalStorage(): Storage | null {
  try {
    if (typeof window === "undefined") return null;
    return window.localStorage;
  } catch {
    return null;
  }
}

function sanitizeProfile(value: unknown): string | null {
  return typeof value === "string" && SELECTABLE_PROFILE_IDS.includes(value)
    ? value
    : null;
}

function sanitizeDownloadMbps(value: unknown): number | null {
  return typeof value === "number" &&
    Number.isFinite(value) &&
    value > 0 &&
    value < DOWNLOAD_SPEED_MAX_MBPS
    ? value
    : null;
}

function sanitizeAudioLanguage(value: unknown): string | null {
  return typeof value === "string" &&
    value !== SUBTITLE_OFF_VALUE &&
    LANGUAGE_CODE_PATTERN.test(value)
    ? value
    : null;
}

function sanitizeSubtitleLanguage(value: unknown): string | null {
  if (value === SUBTITLE_OFF_VALUE) return SUBTITLE_OFF_VALUE;
  return typeof value === "string" && LANGUAGE_CODE_PATTERN.test(value)
    ? value
    : null;
}

// localStorage is user-writable and the server no longer validates these
// values, so every field is coerced on the way in. Unknown keys are dropped by
// construction.
function sanitize(raw: unknown): DevicePlaybackPreferences {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) {
    return DEFAULT_DEVICE_PLAYBACK_PREFERENCES;
  }

  const value = raw as Record<string, unknown>;
  return {
    preferredProfile: sanitizeProfile(value.preferredProfile),
    downloadMbps: sanitizeDownloadMbps(value.downloadMbps),
    preferredAudioLanguage: sanitizeAudioLanguage(value.preferredAudioLanguage),
    preferredSubtitleLanguage: sanitizeSubtitleLanguage(
      value.preferredSubtitleLanguage,
    ),
  };
}

function read(userId: number): DevicePlaybackPreferences {
  const storage = browserLocalStorage();
  if (!storage) return DEFAULT_DEVICE_PLAYBACK_PREFERENCES;

  try {
    const stored = storage.getItem(storageKeyForUser(userId));
    if (stored === null) return DEFAULT_DEVICE_PLAYBACK_PREFERENCES;
    return sanitize(JSON.parse(stored));
  } catch {
    return DEFAULT_DEVICE_PLAYBACK_PREFERENCES;
  }
}

// useSyncExternalStore compares snapshots by reference, so a given user's
// snapshot object must stay identical until something writes. Re-parsing on
// every read would loop forever.
const snapshots = new Map<number, DevicePlaybackPreferences>();
const listeners = new Set<() => void>();

// Users whose last write never reached localStorage (private mode, quota).
// Their current values exist only in `snapshots`, so a later patch must merge
// against the snapshot rather than re-reading storage that never took them.
const unpersisted = new Set<number>();

/** Returns this user's stored preferences on this device, sanitized. */
export function getDevicePlaybackPreferences(
  userId: number,
): DevicePlaybackPreferences {
  const cached = snapshots.get(userId);
  if (cached) return cached;

  const loaded = read(userId);
  snapshots.set(userId, loaded);
  return loaded;
}

function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

function userIdFromStorageKey(key: string): number | null {
  if (!key.startsWith(PLAYBACK_PREFERENCES_STORAGE_PREFIX)) return null;

  const suffix = key.slice(PLAYBACK_PREFERENCES_STORAGE_PREFIX.length);
  if (!/^\d+$/.test(suffix)) return null;

  return Number(suffix);
}

// Another tab wrote to localStorage. Drop the affected snapshot rather than
// adopting event.newValue, so the next read re-parses and re-sanitizes through
// read() and there is only ever one parse path.
function handleStorageEvent(event: StorageEvent): void {
  // A null key means the other tab called localStorage.clear().
  if (event.key === null) {
    snapshots.clear();
    unpersisted.clear();
    notify();
    return;
  }

  const userId = userIdFromStorageKey(event.key);
  if (userId === null) return;

  snapshots.delete(userId);
  unpersisted.delete(userId);
  notify();
}

/**
 * Merges a patch into this user's preferences and persists them.
 *
 * Returns whether the write reached localStorage. A false result still applies
 * the value for this session, but the caller must not confirm it as a durable
 * save -- see docs/design-system.md §3.7.
 */
export function setDevicePlaybackPreferences(
  userId: number,
  patch: Partial<DevicePlaybackPreferences>,
): boolean {
  // Merge against storage, not the cached snapshot: another tab may have
  // written since this tab last read, and its `storage` event is delivered
  // asynchronously. Merging stale would write the whole object back and
  // silently clear the field the other tab changed.
  const base = unpersisted.has(userId)
    ? getDevicePlaybackPreferences(userId)
    : read(userId);
  const next = sanitize({ ...base, ...patch });
  snapshots.set(userId, next);

  let persisted = false;
  const storage = browserLocalStorage();
  if (storage) {
    try {
      storage.setItem(storageKeyForUser(userId), JSON.stringify(next));
      persisted = true;
    } catch {
      // Private mode or quota. The value still applies for this session; the
      // caller reports the session-only save.
    }
  }

  if (persisted) {
    unpersisted.delete(userId);
  } else {
    unpersisted.add(userId);
  }

  notify();
  return persisted;
}

/**
 * Subscribes to preference changes; returns an unsubscribe function.
 *
 * The `storage` listener is attached with the first subscriber and dropped with
 * the last, so tabs stay in sync only while something is rendering. Dropping it
 * also drops the cached snapshots, because nothing would invalidate them until
 * the next subscriber arrives.
 */
export function subscribeDevicePlaybackPreferences(
  listener: () => void,
): () => void {
  if (listeners.size === 0 && typeof window !== "undefined") {
    window.addEventListener("storage", handleStorageEvent);
  }

  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && typeof window !== "undefined") {
      window.removeEventListener("storage", handleStorageEvent);
      // A cached snapshot can now only go stale -- another tab's write is no
      // longer observed -- and non-subscribing readers such as the movie play
      // loader would serve it. Drop them so the next read re-parses. Users
      // whose last write never reached storage are kept: re-reading would
      // return values that storage never took.
      for (const cachedUserId of snapshots.keys()) {
        if (!unpersisted.has(cachedUserId)) {
          snapshots.delete(cachedUserId);
        }
      }
    }
  };
}

/** Drops cached snapshots so the next read re-parses storage. Test-only. */
export function resetDevicePlaybackPreferencesCache(): void {
  snapshots.clear();
  unpersisted.clear();
}
