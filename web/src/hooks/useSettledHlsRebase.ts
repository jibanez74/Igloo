import { useEffect, useRef } from "react";
import { HLS_SEEK_SETTLE_MS } from "@/lib/constants";
import { clearTimeoutRef } from "@/lib/utils";
import { nextHlsSeekInputOrder } from "@/lib/video-playback";

type SettledHlsRebaseOptions = {
  /** The stream window; a change drops a pending rebase it already answered. */
  resetKey: string;
  onRebase: (targetTimeSec: number) => void;
  /**
   * Whether the element is still settling a seek the viewer made after the
   * request with this order (see nextHlsSeekInputOrder). The request then
   * yields, and the element's own report decides where playback goes.
   */
  newerElementSeekPending?: (requestOrder: number) => boolean;
};

// Holds a rebase the page's own controls asked for until they have been quiet
// for HLS_SEEK_SETTLE_MS, the same window VideoPlayer waits before reporting
// seeks the element made. The seek bar calls seek() on every pointer move, and
// a rebase changes the session start at once, so a drag used to rebase — and
// start a transcode — on any step that crossed the 120 s window, and on every
// ~10 s of travel back past the new session's start. Now a drag, a run of key
// presses, or a far click costs one rebase at its final target.
//
// The two settle windows overlap, so the latest input wins rather than the
// first timer to fire: a far click followed by a picture-in-picture scrub
// used to rebase to the click, and a drag's settled report cancelled a far
// click made after it.
export function useSettledHlsRebase({
  resetKey,
  onRebase,
  newerElementSeekPending,
}: SettledHlsRebaseOptions) {
  const timerRef = useRef<number | null>(null);
  const targetRef = useRef(0);
  const orderRef = useRef(0);
  // The timer fires after later renders, so it calls the current handlers
  // rather than the ones the seek closed over.
  const handlersRef = useRef({ onRebase, newerElementSeekPending });
  useEffect(() => {
    handlersRef.current = { onRebase, newerElementSeekPending };
  }, [onRebase, newerElementSeekPending]);

  const cancelRebase = () => {
    clearTimeoutRef(timerRef);
  };

  const requestRebase = (targetTimeSec: number) => {
    targetRef.current = targetTimeSec;
    orderRef.current = nextHlsSeekInputOrder();
    cancelRebase();
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      const handlers = handlersRef.current;
      const superseded =
        handlers.newerElementSeekPending?.(orderRef.current) ?? false;
      if (superseded) return;
      handlers.onRebase(targetRef.current);
    }, HLS_SEEK_SETTLE_MS);
  };

  const isRebasePending = () => timerRef.current !== null;

  /** Whether a pending rebase was requested after the input with this order. */
  const isRebasePendingAfter = (order: number) =>
    timerRef.current !== null && orderRef.current > order;

  // A new stream window (another quality, audio track, or start) or an
  // unmount must not inherit a rebase meant for the previous one.
  useEffect(() => {
    return () => {
      clearTimeoutRef(timerRef);
    };
  }, [resetKey]);

  return { requestRebase, cancelRebase, isRebasePending, isRebasePendingAfter };
}
