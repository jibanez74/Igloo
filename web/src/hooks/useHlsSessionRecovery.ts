import { useEffect, useRef, useState } from "react";
import {
  HLS_SESSION_LOST_INCIDENT_WINDOW_MS,
  HLS_SESSION_LOST_MAX_ATTEMPTS,
  HLS_SESSION_LOST_MIN_INTERVAL_MS,
} from "@/lib/constants";
import { currentPlaybackTimestampMs } from "@/lib/video-playback";

type HlsSessionRecoveryOptions = {
  onRecover: (currentTimeSec: number) => void;
  onMaxAttempts: (message: string) => void;
};

// The budget belongs to an incident, not to a stream window. It used to reset
// whenever the window key changed, but a recovery changes that key itself
// (it rebases to a new start), so a 404 that kept coming back never used up
// the budget: each attempt rewound ten seconds and fired straight away,
// hundreds of manifest requests in a row. A loss more than
// HLS_SESSION_LOST_INCIDENT_WINDOW_MS after the previous attempt starts a new
// incident, so an idle eviction an hour into the film still gets a full
// budget. The budget and the pending retry belong to one media item: both
// play routes key the page on the media id, so a change of item is a remount
// and the unmount cleanup drops a retry that still held the old position.
export function useHlsSessionRecovery({
  onRecover,
  onMaxAttempts,
}: HlsSessionRecoveryOptions) {
  const attemptsRef = useRef(0);
  const lastAttemptAtRef = useRef(0);
  const retryTimerRef = useRef<number | null>(null);
  // Counts every attempt for the life of the page rather than per incident,
  // so the announcement key changes on every attempt, even the first of a
  // new incident; 0 means no recovery has run yet.
  const [recoveryAttempt, setRecoveryAttempt] = useState(0);

  useEffect(() => {
    return () => {
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current);
      }
    };
  }, []);

  const attempt = (currentTimeSec: number) => {
    attemptsRef.current += 1;
    lastAttemptAtRef.current = currentPlaybackTimestampMs();
    setRecoveryAttempt((previous) => previous + 1);
    onRecover(currentTimeSec);
  };

  const handleSessionLost = (currentTimeSec: number) => {
    // A retry is already scheduled for this incident.
    if (retryTimerRef.current !== null) return;

    const sinceLastAttemptMs =
      currentPlaybackTimestampMs() - lastAttemptAtRef.current;
    if (
      attemptsRef.current > 0 &&
      sinceLastAttemptMs >= HLS_SESSION_LOST_INCIDENT_WINDOW_MS
    ) {
      attemptsRef.current = 0;
    }

    if (attemptsRef.current >= HLS_SESSION_LOST_MAX_ATTEMPTS) {
      onMaxAttempts(
        "Playback session could not be recovered. Try reloading the page or choosing another quality.",
      );
      return;
    }

    // Spaced, not dropped: the player reports each lost session once, so a
    // report that arrives too soon is the next failure, not a duplicate.
    // Dropping it left the player stalled with nothing scheduled.
    const waitMs =
      attemptsRef.current > 0
        ? HLS_SESSION_LOST_MIN_INTERVAL_MS - sinceLastAttemptMs
        : 0;
    if (waitMs > 0) {
      retryTimerRef.current = window.setTimeout(() => {
        retryTimerRef.current = null;
        attempt(currentTimeSec);
      }, waitMs);
      return;
    }

    attempt(currentTimeSec);
  };

  // Wipes the budget when the viewer asks for a fresh start, such as the
  // Retry button on the error screen.
  const resetRecovery = () => {
    attemptsRef.current = 0;
    lastAttemptAtRef.current = 0;
    if (retryTimerRef.current !== null) {
      window.clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  };

  return { handleSessionLost, resetRecovery, recoveryAttempt };
}
