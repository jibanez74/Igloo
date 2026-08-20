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

/** Merges a patch into this user's preferences and persists them. */
export function setDevicePlaybackPreferences(
  userId: number,
  patch: Partial<DevicePlaybackPreferences>,
): void {
  const next = sanitize({ ...getDevicePlaybackPreferences(userId), ...patch });
  snapshots.set(userId, next);

  const storage = browserLocalStorage();
  try {
    storage?.setItem(storageKeyForUser(userId), JSON.stringify(next));
  } catch {
    // Ignore storage failures (private mode, quota); the value still applies
    // for this session, matching the theme module's behaviour.
  }

  notify();
}

/** Subscribes to preference changes; returns an unsubscribe function. */
export function subscribeDevicePlaybackPreferences(
  listener: () => void,
): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Drops cached snapshots so the next read re-parses storage. Test-only. */
export function resetDevicePlaybackPreferencesCache(): void {
  snapshots.clear();
}
