import { useEffect } from "react";
import { HLS_SESSION_KEEPALIVE_INTERVAL_MS } from "@/lib/constants";
import { releaseResponseBody } from "@/lib/video-playback";

type HlsSessionKeepaliveOptions = {
  enabled: boolean;
  streamUrl: string;
};

/**
 * Keeps the server-side HLS session alive while the ready player is rendered.
 *
 * hls.js stops fetching once its buffer is full, and a paused tab fetches
 * nothing at all, so without this the session's short idle TTL would evict a
 * still-open player. Refetching the manifest refreshes the TTL through the
 * same path as regular playback traffic, and if the session was already
 * evicted (e.g. after OS sleep) the request transparently recreates it at the
 * same start offset.
 */
export function useHlsSessionKeepalive({
  enabled,
  streamUrl,
}: HlsSessionKeepaliveOptions) {
  // Not data fetching: the response is discarded. The ping exists only for its
  // server-side effect of refreshing the HLS session TTL.
  // react-doctor-disable-next-line react-doctor/no-fetch-in-effect
  useEffect(() => {
    if (!enabled || !streamUrl) return;

    // Aborted on cleanup. A ping still in flight when the player unmounts
    // used to reach the server after the page's stop request and, since a
    // manifest request recreates a missing session, start an orphan that
    // transcoded until its idle TTL ran out.
    const controller = new AbortController();
    let inFlight = false;

    const ping = async () => {
      // A manifest request can wait on the server for tens of seconds, so a
      // slow ping must not stack a second one behind it.
      if (inFlight) return;
      inFlight = true;
      try {
        const response = await fetch(streamUrl, {
          credentials: "include",
          signal: controller.signal,
        });
        // Only the request matters. Releasing the body frees the connection
        // instead of leaving the playlist unread.
        await releaseResponseBody(response);
      } catch {
        // Best-effort keepalive; playback errors surface through the player.
      } finally {
        inFlight = false;
      }
    };

    const interval = window.setInterval(() => {
      void ping();
    }, HLS_SESSION_KEEPALIVE_INTERVAL_MS);

    return () => {
      window.clearInterval(interval);
      controller.abort();
    };
  }, [enabled, streamUrl]);
}
