import { useEffect, useRef, useState } from "react";
import { HLS_CAPACITY_RETRY_MAX_ATTEMPTS } from "@/lib/constants";
type HlsCapacityRetryOptions = {
  streamWindowKey: string;
  onRetry: () => void;
  onMaxAttempts: (message: string) => void;
};

/**
 * Retries HLS manifest loads that fail with 503 (server at transcode
 * capacity), honoring the server's Retry-After delay. Attempts are budgeted
 * per stream window so a persistently saturated server surfaces a playback
 * error instead of retrying forever.
 *
 * `waitingForCapacity` stays true across the retry request, not just during
 * the Retry-After delay: the server parks a queued manifest request for up to
 * 15s before answering, and clearing the flag when the retry fires would hide
 * the explanation behind a bare spinner for most of the wait. It is the
 * consumer's job to call `notifyManifestLoaded` once a manifest arrives.
 */
export function useHlsCapacityRetry({
  streamWindowKey,
  onRetry,
  onMaxAttempts,
}: HlsCapacityRetryOptions) {
  const attemptsRef = useRef(0);
  const timerRef = useRef<number | null>(null);
  const trackedKeyRef = useRef("");
  const [waitingForCapacity, setWaitingForCapacity] = useState(false);

  useEffect(() => {
    if (trackedKeyRef.current === streamWindowKey) return;
    trackedKeyRef.current = streamWindowKey;
    attemptsRef.current = 0;
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    // Not derivable from props: the flag is set by hls.js 503 responses. The reset
    // also cancels a live timer, which render-phase adjustment cannot do.
    // react-doctor-disable-next-line react-doctor/no-adjust-state-on-prop-change
    setWaitingForCapacity(false);
  }, [streamWindowKey]);

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
      }
    };
  }, []);

  const handleCapacityBusy = (retryAfterSec: number) => {
    if (timerRef.current !== null) return;

    if (attemptsRef.current >= HLS_CAPACITY_RETRY_MAX_ATTEMPTS) {
      setWaitingForCapacity(false);
      onMaxAttempts(
        "The server is running at its transcoding limit. Try again in a few minutes.",
      );
      return;
    }

    attemptsRef.current += 1;
    setWaitingForCapacity(true);
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      onRetry();
    }, retryAfterSec * 1000);
  };

  // A manifest arrived, so the stream is no longer queued behind capacity.
  // The attempt budget deliberately survives: it is scoped to the stream
  // window, and a session that got in can still be refused on a later reload.
  const notifyManifestLoaded = () => {
    setWaitingForCapacity(false);
  };

  return { waitingForCapacity, handleCapacityBusy, notifyManifestLoaded };
}
