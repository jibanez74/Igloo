import { useSyncExternalStore } from "react";
import {
  getDevicePlaybackPreferences,
  subscribeDevicePlaybackPreferences,
} from "@/lib/playback-preferences";
import type { DevicePlaybackPreferences } from "@/types";

/** This device's playback preferences for the given user, kept in sync with localStorage. */
export function useDevicePlaybackPreferences(
  userId: number,
): DevicePlaybackPreferences {
  return useSyncExternalStore(
    subscribeDevicePlaybackPreferences,
    () => getDevicePlaybackPreferences(userId),
    () => getDevicePlaybackPreferences(userId),
  );
}
