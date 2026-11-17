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
 * natively (iPhone Safari) never needs it. A failure here is left to the
 * player's own import, which reports it.
 */
export function preloadHlsLight() {
  if (prefersNativeHLS) return;
  void (async () => {
    try {
      await loadHlsLight();
    } catch {
      // The player's own import reports the failure.
    }
  })();
}
