import { useEffect, useEffectEvent } from "react";
import {
  HLS_KEEPALIVE_QUERY_PARAM,
  HLS_SESSION_KEEPALIVE_INTERVAL_MS,
} from "@/lib/constants";
import { releaseResponseBody } from "@/lib/video-playback";

type HlsSessionKeepaliveOptions = {
  enabled: boolean;
  streamUrl: string;
  /**
   * Makes every ping refresh-only. The server then never recreates an evicted
   * session and answers 404 instead, which is reported once per stream URL so
   * the caller can rebase at its playhead. Without it a ping recreates a
   * missing session at the stream URL's start, which suits a watch room: its
   * stream always starts at 0 and the room sync restores the position.
   */
  onSessionLost?: () => void;
};

/**
 * Keeps the server-side HLS session alive while the ready player is rendered.
 *
 * hls.js stops fetching once its buffer is full, and a paused tab fetches
 * nothing at all, so without this the session's short idle TTL would evict a
 * still-open player. Refetching the manifest refreshes the TTL through the
 * same path as regular playback traffic. A session already evicted (after OS
 * sleep, say) must not simply be recreated at the window start for a personal
 * stream: the playhead may be far past it, and a transcode would then encode
 * from the window start while the player waits. Those pings are refresh-only
 * and report the loss instead.
 */
export function useHlsSessionKeepalive({
  enabled,
  streamUrl,
  onSessionLost,
}: HlsSessionKeepaliveOptions) {
  const refreshOnly = onSessionLost !== undefined;
  const reportSessionLost = useEffectEvent(() => {
    onSessionLost?.();
  });

  // Not data fetching: the response is discarded. The ping exists only for its
  // server-side effect of refreshing the HLS session TTL.
  // react-doctor-disable-next-line react-doctor/no-fetch-in-effect
  useEffect(() => {
    if (!enabled || !streamUrl) return;

    const pingUrl = refreshOnly
      ? `${streamUrl}${streamUrl.includes("?") ? "&" : "?"}${HLS_KEEPALIVE_QUERY_PARAM}=1`
      : streamUrl;
    // Aborted on cleanup. A ping still in flight when the player unmounts
    // used to reach the server after the page's stop request and, since a
    // manifest request recreates a missing session, start an orphan that
    // transcoded until its idle TTL ran out.
    const controller = new AbortController();
    let inFlight = false;
    let lostReported = false;

    const ping = async () => {
      // A manifest request can wait on the server for tens of seconds, so a
      // slow ping must not stack a second one behind it.
      if (inFlight) return;
      inFlight = true;
      try {
        const response = await fetch(pingUrl, {
          credentials: "include",
          signal: controller.signal,
        });
        // Only the request matters. Releasing the body frees the connection
        // instead of leaving the playlist unread.
        await releaseResponseBody(response);
        const sessionLost = refreshOnly && response.status === 404;
        if (sessionLost && !lostReported) {
          lostReported = true;
          reportSessionLost();
        }
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
  }, [enabled, streamUrl, refreshOnly]);
}
