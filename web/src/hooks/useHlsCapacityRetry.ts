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
      setWaitingForCapacity(false);
      onRetry();
    }, retryAfterSec * 1000);
  };

  return { waitingForCapacity, handleCapacityBusy };
}
