import { useState } from "react";
import { hasEligibleMovieResumeProgress } from "@/lib/movie-playback";

type ResumeDecision =
  | { status: "pending" }
  | { status: "show"; resumeTargetSec: number }
  | { status: "dismissed" };

type MovieResumeDecisionOptions = {
  movieId: number;
  start: number;
  playing: boolean;
  watchProgressPending: boolean;
  savedProgressSec: number | null;
  savedDurationSec: number | null;
};

const PENDING: ResumeDecision = { status: "pending" };
const DISMISSED: ResumeDecision = { status: "dismissed" };

/**
 * Decides once per movie whether to offer resuming from saved progress.
 *
 * The decision is a snapshot taken when the watch-progress query first
 * resolves and is latched afterwards, so background refetches (window focus,
 * stale-time expiry) and progress saved during the current playback can never
 * re-open the dialog mid-movie.
 */
export function useMovieResumeDecision({
  movieId,
  start,
  playing,
  watchProgressPending,
  savedProgressSec,
  savedDurationSec,
}: MovieResumeDecisionOptions) {
  const initialDecision = start > 0 ? DISMISSED : PENDING;
  const [trackedMovieId, setTrackedMovieId] = useState(movieId);
  const [decision, setDecision] = useState(initialDecision);

  let effectiveDecision = decision;
  if (trackedMovieId !== movieId) {
    setTrackedMovieId(movieId);
    setDecision(initialDecision);
    effectiveDecision = initialDecision;
  }

  if (effectiveDecision.status === "pending") {
    if (playing) {
      // The user started playback before the progress query resolved; the
      // dialog must never interrupt playback that is already running.
      setDecision(DISMISSED);
      effectiveDecision = DISMISSED;
    } else if (!watchProgressPending) {
      const eligible = hasEligibleMovieResumeProgress(
        savedProgressSec,
        savedDurationSec,
      );
      const resolved: ResumeDecision =
        eligible && savedProgressSec !== null
          ? { status: "show", resumeTargetSec: savedProgressSec }
          : DISMISSED;
      setDecision(resolved);
      effectiveDecision = resolved;
    }
  }

  const dismissResumeDecision = () => setDecision(DISMISSED);

  return {
    resumeDialogOpen: effectiveDecision.status === "show",
    resumeTargetSec:
      effectiveDecision.status === "show"
        ? effectiveDecision.resumeTargetSec
        : null,
    dismissResumeDecision,
  };
}
