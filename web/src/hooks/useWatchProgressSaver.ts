import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";
import {
  createPlaybackSessionId,
  persistWatchProgress,
  shouldPersistWatchProgress,
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
  // `seq` names the save that made the report, so a failure clears only its
  // own report; `settled` is set once an ordinary save succeeds, or when a
  // keepalive is dispatched (nothing later can confirm it).
  const lastReportRef = useRef<{
    sec: number;
    seq: number;
    settled: boolean;
  } | null>(null);
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
    const report = lastReportRef.current;
    if (report === null) return playedRef.current;
    return Math.abs(progressSec - report.sec) >= 1;
  }, []);

  // A failed save leaves its position unreported, so the next chance retries,
  // unless a later save has reported a position since.
  const forgetFailedReport = useCallback((saveSequence: number) => {
    if (lastReportRef.current?.seq === saveSequence) {
      lastReportRef.current = null;
    }
  }, []);

  // A save that would not be sent (no duration yet, or a position below the
  // floor) reports nothing, so the same position is saved once it can be.
  const queueProgressSave = useCallback(
    (progressSec: number, durationSec: number) => {
      if (!shouldPersistWatchProgress(progressSec, durationSec)) {
        return Promise.resolve();
      }
      const saveSequence = ++saveSequenceRef.current;
      const previous = pendingSaveRef.current;
      lastReportRef.current = {
        sec: progressSec,
        seq: saveSequence,
        settled: false,
      };
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
          forgetFailedReport(saveSequence);
          throw error;
        }
        const report = lastReportRef.current;
        if (report?.seq === saveSequence) report.settled = true;
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

  // A position already reported may still be on its way: the exit refreshes
  // the watch queries once this resolves, so wait for the pending save, and
  // retry the position if that save failed.
  const saveIfNewProgress = useCallback(async () => {
    const progressSec = currentTimeRef.current;
    if (!hasNewProgress(progressSec)) {
      await pendingSaveRef.current;
      if (!hasNewProgress(progressSec)) return;
    }
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
    // An ordinary save still in flight does not count: unload can cancel it.
    const flushKeepalive = async () => {
      const progressSec = currentTimeRef.current;
      const report = lastReportRef.current;
      if (
        report === null
          ? !playedRef.current
          : report.settled && Math.abs(progressSec - report.sec) < 1
      ) {
        return;
      }
      const durationSec = effectiveDurationSec();
      if (!shouldPersistWatchProgress(progressSec, durationSec)) return;

      const saveSequence = ++saveSequenceRef.current;
      lastReportRef.current = {
        sec: progressSec,
        seq: saveSequence,
        settled: true,
      };
      const sent = await persistWatchProgress(
        { kind, id },
        progressSec,
        durationSec,
        saveSessionIdRef.current!,
        saveSequence,
        { keepalive: true },
      );
      if (!sent) forgetFailedReport(saveSequence);
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
