import { useEffect, useRef } from "react";
import { HLS_SEEK_SETTLE_MS } from "@/lib/constants";

type SettledHlsRebaseOptions = {
  /** The stream window; a change drops a pending rebase it already answered. */
  resetKey: string;
  onRebase: (targetTimeSec: number) => void;
};

// Holds a rebase the page's own controls asked for until they have been quiet
// for HLS_SEEK_SETTLE_MS, the same window VideoPlayer waits before reporting
// seeks the element made. The seek bar calls seek() on every pointer move, and
// a rebase changes the session start at once, so a drag used to rebase — and
// start a transcode — on any step that crossed the 120 s window, and on every
// ~10 s of travel back past the new session's start. Now a drag, a run of key
// presses, or a far click costs one rebase at its final target.
export function useSettledHlsRebase({
  resetKey,
  onRebase,
}: SettledHlsRebaseOptions) {
  const timerRef = useRef<number | null>(null);
  const targetRef = useRef(0);
  // The timer fires after later renders, so it calls the current handler
  // rather than the one the seek closed over.
  const onRebaseRef = useRef(onRebase);
  useEffect(() => {
    onRebaseRef.current = onRebase;
  }, [onRebase]);

  const cancelRebase = () => {
    if (timerRef.current === null) return;
    window.clearTimeout(timerRef.current);
    timerRef.current = null;
  };

  const requestRebase = (targetTimeSec: number) => {
    targetRef.current = targetTimeSec;
    cancelRebase();
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      onRebaseRef.current(targetRef.current);
    }, HLS_SEEK_SETTLE_MS);
  };

  const isRebasePending = () => timerRef.current !== null;

  // A new stream window (another quality, audio track, or start) or an
  // unmount must not inherit a rebase meant for the previous one.
  useEffect(() => {
    return () => {
      if (timerRef.current === null) return;
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    };
  }, [resetKey]);

  return { requestRebase, cancelRebase, isRebasePending };
}
