import { prefersNativeHLS } from "@/lib/playback";

/** hls.js's light build: its own chunk, loaded only by a page that streams HLS. */
export function loadHlsLight() {
  return import("hls.js/light");
}

/**
 * Starts fetching hls.js ahead of the player that needs it. The player loads
 * it only once its stream is known (after the user, the technical details
 * and the server settings), so on a cold cache the ~360 KB chunk used to
 * download after all of them instead of alongside. A browser that plays HLS
 * natively (iPhone Safari) never needs it. A failure here is the player's to
 * report when it imports the chunk itself.
 */
export function preloadHlsLight() {
  if (prefersNativeHLS) return;
  void (async () => {
    try {
      await loadHlsLight();
    } catch {
      // The player retries the import and reports its own failure.
    }
  })();
}
