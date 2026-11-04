import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";
import {
  createPlaybackSessionId,
  persistWatchProgress,
} from "@/lib/video-playback";
import { WATCH_PROGRESS_SAVE_INTERVAL_MS } from "@/lib/constants";
import { showActionFailed } from "@/lib/toast-helpers";
import type { PlaybackMediaRef } from "@/types/playback";

type WatchProgressSaverOptions = {
  media: PlaybackMediaRef;
  playing: boolean;
  currentTimeRef: RefObject<number>;
  durationRef: RefObject<number>;
  /**
   * Duration from the media's technical details, used when the video element
   * has not reported a duration yet (HLS streams report the session-local
   * duration late). Without it, exit saves before metadata loads are dropped.
   */
  fallbackDurationSec?: number;
  /**
   * False while nothing the viewer did should count as watching the media:
   * the trailer pre-roll runs before the movie, so no save of any kind (the
   * interval, pause/end, tab-hidden keepalive, or the exit flush) may run.
   */
  enabled?: boolean;
};

export function useWatchProgressSaver({
  media,
  playing,
  currentTimeRef,
  durationRef,
  fallbackDurationSec,
  enabled = true,
}: WatchProgressSaverOptions) {
  // Null until the first save; the chain starts from a resolved promise then.
  const pendingSaveRef = useRef<Promise<void> | null>(null);
  const fallbackDurationRef = useRef(fallbackDurationSec ?? 0);
  const saveSessionIdRef = useRef<string | null>(null);
  const saveSequenceRef = useRef(0);
  // Every page load is a new save session, and the server takes any write
  // from another session, so a page reporting a position it already reported
  // (a paused tab closed after the viewer went on watching on the TV) would
  // roll the newer progress back. Only a position that moved since the last
  // save is reported; before any save, only once the media has played.
  const lastReportedSecRef = useRef<number | null>(null);
  const playedRef = useRef(false);
  const { kind, id } = media;

  if (saveSessionIdRef.current === null) {
    saveSessionIdRef.current = createPlaybackSessionId();
  }

  useEffect(() => {
    fallbackDurationRef.current = fallbackDurationSec ?? 0;
  }, [fallbackDurationSec]);

  useEffect(() => {
    if (playing && enabled) playedRef.current = true;
  }, [playing, enabled]);

  const effectiveDurationSec = useCallback(() => {
    if (durationRef.current > 0) return durationRef.current;
    return fallbackDurationRef.current;
  }, [durationRef]);

  const hasNewProgress = useCallback((progressSec: number) => {
    const lastReported = lastReportedSecRef.current;
    if (lastReported === null) return playedRef.current;
    return Math.abs(progressSec - lastReported) >= 1;
  }, []);

  // A failed save leaves its position unreported, so the next chance retries.
  const forgetFailedReport = useCallback((progressSec: number) => {
    if (lastReportedSecRef.current === progressSec) {
      lastReportedSecRef.current = null;
    }
  }, []);

  const queueProgressSave = useCallback(
    (progressSec: number, durationSec: number) => {
      const saveSequence = ++saveSequenceRef.current;
      const previous = pendingSaveRef.current;
      lastReportedSecRef.current = progressSec;
      const save = (async () => {
        await previous;
        try {
          await persistWatchProgress(
            { kind, id },
            progressSec,
            durationSec,
            saveSessionIdRef.current!,
            saveSequence,
          );
        } catch (error) {
          forgetFailedReport(progressSec);
          throw error;
        }
      })();
      pendingSaveRef.current = (async () => {
        try {
          await save;
        } catch {
          // The chain only orders saves; each caller handles its own failure.
        }
      })();
      return save;
    },
    [kind, id, forgetFailedReport],
  );

  const saveIfNewProgress = useCallback(async () => {
    const progressSec = currentTimeRef.current;
    if (!hasNewProgress(progressSec)) return;
    await queueProgressSave(progressSec, effectiveDurationSec());
  }, [currentTimeRef, hasNewProgress, queueProgressSave, effectiveDurationSec]);

  useEffect(() => {
    if (!playing || !enabled) return;
    const interval = window.setInterval(async () => {
      try {
        await saveIfNewProgress();
      } catch {
        // Silent background save failure; pause/end handlers surface failures when needed.
      }
    }, WATCH_PROGRESS_SAVE_INTERVAL_MS);
    return () => {
      window.clearInterval(interval);
    };
  }, [playing, enabled, saveIfNewProgress]);

  useEffect(() => {
    if (!enabled) return;

    // On a real page close, visibilitychange (hidden) usually fires and then
    // pagehide follows; the second finds its position already reported, while
    // browsers/platforms that only deliver one of the two are still covered.
    const flushKeepalive = async () => {
      const progressSec = currentTimeRef.current;
      if (!hasNewProgress(progressSec)) return;

      const saveSequence = ++saveSequenceRef.current;
      lastReportedSecRef.current = progressSec;
      const sent = await persistWatchProgress(
        { kind, id },
        progressSec,
        effectiveDurationSec(),
        saveSessionIdRef.current!,
        saveSequence,
        { keepalive: true },
      );
      if (!sent) forgetFailedReport(progressSec);
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState !== "hidden") return;
      void flushKeepalive();
    };
    const handlePageHide = () => {
      void flushKeepalive();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("pagehide", handlePageHide);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("pagehide", handlePageHide);
    };
  }, [
    enabled,
    currentTimeRef,
    effectiveDurationSec,
    hasNewProgress,
    forgetFailedReport,
    kind,
    id,
  ]);

  const handlePauseSave = async () => {
    if (!enabled) return;
    try {
      await saveIfNewProgress();
    } catch {
      // Best effort on pause; avoid interrupting playback UI with repeated toasts.
    }
  };

  // Always sent: completion must not depend on an earlier save succeeding.
  const handleEndedSave = async () => {
    if (!enabled) return;
    const durationSec = effectiveDurationSec();
    try {
      await queueProgressSave(durationSec, durationSec);
    } catch {
      showActionFailed(
        "save watch progress",
        `Unable to mark this ${kind} as watched.`,
      );
    }
  };

  const flushProgress = () => {
    if (!enabled) return Promise.resolve();
    return saveIfNewProgress();
  };

  return { handlePauseSave, handleEndedSave, flushProgress };
}
