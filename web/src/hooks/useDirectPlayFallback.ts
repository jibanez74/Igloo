import { useEffect, useEffectEvent, useRef } from "react";
import type { RefObject } from "react";
import { DIRECT_PLAY_STALL_TIMEOUT_MS } from "@/lib/constants";
import {
  shouldDirectPlayFallback,
  type DirectPlayFallbackTrigger,
} from "@/lib/movie-playback";
import type { StreamModeId } from "@/types";

type DirectPlayFallbackOptions = {
  streamWindowKey: string;
  videoRef: RefObject<HTMLVideoElement | null>;
  isHlsPlayback: boolean;
  resolvedMode: StreamModeId;
  techLoaded: boolean;
  directAvailable: boolean;
  /** True while the player (and therefore the video element) is rendered. */
  playerMounted: boolean;
  onFallback: () => void;
};

/**
 * One-shot fallback from a failed direct play to remux (audit D-FB/D-FB2).
 * `handleNativeError` consumes the two decode-incompatibility MediaError
 * codes; a bounded stall guard covers the container failures that never raise
 * a MediaError at all (silent 0ms stall — audit §5.6). The attempt budget
 * resets per stream window, so a user who manually re-selects direct after a
 * fallback gets exactly one more attempt.
 */
export function useDirectPlayFallback({
  streamWindowKey,
  videoRef,
  isHlsPlayback,
  resolvedMode,
  techLoaded,
  directAvailable,
  playerMounted,
  onFallback,
}: DirectPlayFallbackOptions) {
  const attemptedRef = useRef(false);
  const trackedKeyRef = useRef("");

  useEffect(() => {
    if (trackedKeyRef.current !== streamWindowKey) {
      trackedKeyRef.current = streamWindowKey;
      attemptedRef.current = false;
    }
  }, [streamWindowKey]);

  const attemptFallback = (
    trigger: DirectPlayFallbackTrigger | null | undefined,
  ): boolean => {
    const shouldFallback = shouldDirectPlayFallback({
      trigger,
      isHlsPlayback,
      resolvedMode,
      techLoaded,
      directAvailable,
      alreadyAttempted: attemptedRef.current,
    });
    if (!shouldFallback) return false;

    attemptedRef.current = true;
    onFallback();
    return true;
  };

  /** Returns true when the error was consumed by a fallback navigation. */
  const handleNativeError = (code: number | null | undefined): boolean => {
    return attemptFallback(code);
  };

  const fireStallFallback = useEffectEvent(() => {
    attemptFallback("stall");
  });

  const stallGuardArmed =
    playerMounted &&
    !isHlsPlayback &&
    resolvedMode === "direct" &&
    techLoaded &&
    directAvailable;

  // The guard disarms on loadedmetadata rather than on playback progress:
  // a player left paused produces no timeupdate and must not trip it, while
  // the silent-stall container failures never reach metadata at all. Failures
  // after metadata (e.g. a truncated file) surface as MEDIA_ERR_DECODE and
  // take the error path instead.
  useEffect(() => {
    if (!stallGuardArmed) return;
    const video = videoRef.current;
    if (!video) return;
    // Metadata already decoded: this window has proven the bytes play.
    if (video.readyState >= HTMLMediaElement.HAVE_METADATA) return;

    const timer = window.setTimeout(() => {
      fireStallFallback();
    }, DIRECT_PLAY_STALL_TIMEOUT_MS);
    const disarm = () => window.clearTimeout(timer);

    video.addEventListener("loadedmetadata", disarm, { once: true });
    return () => {
      video.removeEventListener("loadedmetadata", disarm);
      window.clearTimeout(timer);
    };
  }, [stallGuardArmed, streamWindowKey, videoRef]);

  return { handleNativeError };
}
