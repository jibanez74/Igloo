import { useEffect, useRef } from "react";
import {
  HLS_SESSION_LOST_MAX_ATTEMPTS,
  HLS_SESSION_LOST_MIN_INTERVAL_MS,
} from "@/lib/constants";
import { currentPlaybackTimestampMs } from "@/lib/movie-playback";

type HlsSessionRecoveryOptions = {
  streamWindowKey: string;
  onRecover: (currentTimeSec: number) => void;
  onMaxAttempts: (message: string) => void;
};

export function useHlsSessionRecovery({
  streamWindowKey,
  onRecover,
  onMaxAttempts,
}: HlsSessionRecoveryOptions) {
  const attemptsRef = useRef(0);
  const lastAttemptAtRef = useRef(0);
  const trackedKeyRef = useRef("");

  useEffect(() => {
    if (trackedKeyRef.current !== streamWindowKey) {
      trackedKeyRef.current = streamWindowKey;
      attemptsRef.current = 0;
      lastAttemptAtRef.current = 0;
    }
  }, [streamWindowKey]);

  const handleSessionLost = (currentTimeSec: number) => {
    const now = currentPlaybackTimestampMs();
    if (attemptsRef.current >= HLS_SESSION_LOST_MAX_ATTEMPTS) {
      onMaxAttempts(
        "Playback session could not be recovered. Try reloading the page or choosing another quality.",
      );
      return;
    }
    const tooSoon =
      attemptsRef.current > 0 &&
      now - lastAttemptAtRef.current < HLS_SESSION_LOST_MIN_INTERVAL_MS;
    if (tooSoon) return;

    attemptsRef.current += 1;
    lastAttemptAtRef.current = now;
    onRecover(currentTimeSec);
  };

  return { handleSessionLost };
}
