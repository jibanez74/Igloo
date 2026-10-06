import { useState } from "react";
import { RESUME_AHEAD_THRESHOLD_SEC } from "@/lib/constants";
import { hasEligibleResumeProgress } from "@/lib/video-playback";

type ResumeDecision =
  | { status: "pending" }
  | { status: "show"; resumeTargetSec: number; startSec: number }
  | { status: "dismissed" };

type ResumeDecisionOptions = {
  /** Identity of the media being played; a change resets the decision. */
  mediaKey: string;
  start: number;
  /** An up-next hand-off that plays on its own, never interrupted. */
  autoplay: boolean;
  playing: boolean;
  watchProgressPending: boolean;
  savedProgressSec: number | null;
  savedDurationSec: number | null;
};

const PENDING: ResumeDecision = { status: "pending" };
const DISMISSED: ResumeDecision = { status: "dismissed" };

/**
 * Decides once per media item whether to offer resuming from saved progress.
 *
 * A page that opens mid-media (a restored tab, a reload, Back into the player,
 * a chapter link) offers it only when the saved position is clearly further
 * along than the URL's start, which is the last seek rather than the last
 * position; the alternative is then playing from that start, not starting over.
 *
 * The decision is a snapshot taken when the watch-progress query first
 * resolves and is latched afterwards, so background refetches (window focus,
 * stale-time expiry) and progress saved during the current playback can never
 * re-open the dialog mid-playback.
 */
export function useResumeDecision({
  mediaKey,
  start,
  autoplay,
  playing,
  watchProgressPending,
  savedProgressSec,
  savedDurationSec,
}: ResumeDecisionOptions) {
  const initialDecision = start > 0 && autoplay ? DISMISSED : PENDING;
  const [trackedMediaKey, setTrackedMediaKey] = useState(mediaKey);
  const [decision, setDecision] = useState(initialDecision);

  let effectiveDecision = decision;
  if (trackedMediaKey !== mediaKey) {
    setTrackedMediaKey(mediaKey);
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
      const eligible = hasEligibleResumeProgress(
        savedProgressSec,
        savedDurationSec,
      );
      const aheadOfStart =
        savedProgressSec !== null &&
        (start === 0 || savedProgressSec - start > RESUME_AHEAD_THRESHOLD_SEC);
      const resolved: ResumeDecision =
        eligible && aheadOfStart && savedProgressSec !== null
          ? { status: "show", resumeTargetSec: savedProgressSec, startSec: start }
          : DISMISSED;
      setDecision(resolved);
      effectiveDecision = resolved;
    }
  }

  const dismissResumeDecision = () => setDecision(DISMISSED);

  return {
    /** True until the saved-progress snapshot has been taken. */
    resumeDecisionPending: effectiveDecision.status === "pending",
    resumeDialogOpen: effectiveDecision.status === "show",
    resumeTargetSec:
      effectiveDecision.status === "show"
        ? effectiveDecision.resumeTargetSec
        : null,
    /** The URL's start the dialog offers instead of starting over, if any. */
    playFromSec:
      effectiveDecision.status === "show" && effectiveDecision.startSec > 0
        ? effectiveDecision.startSec
        : null,
    dismissResumeDecision,
  };
}
