import { useEffect } from "react";
import { HLS_SESSION_KEEPALIVE_INTERVAL_MS } from "@/lib/constants";

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
  useEffect(() => {
    if (!enabled || !streamUrl) return;

    const interval = window.setInterval(() => {
      void fetch(streamUrl, { credentials: "include" }).catch(() => {
        // Best-effort keepalive; playback errors surface through the player.
      });
    }, HLS_SESSION_KEEPALIVE_INTERVAL_MS);

    return () => {
      window.clearInterval(interval);
    };
  }, [enabled, streamUrl]);
}
