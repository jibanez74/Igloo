import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";
import {
  createPlaybackSessionId,
  persistMovieWatchProgress,
} from "@/lib/movie-playback";
import {
  MOVIE_WATCH_PROGRESS_KEEPALIVE_DEDUPE_MS,
  MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS,
} from "@/lib/constants";
import { showActionFailed } from "@/lib/toast-helpers";

type MovieWatchProgressSaverOptions = {
  movieId: number;
  playing: boolean;
  currentTimeRef: RefObject<number>;
  durationRef: RefObject<number>;
  /**
   * Duration from the movie's technical details, used when the video element
   * has not reported a duration yet (HLS streams report the session-local
   * duration late). Without it, exit saves before metadata loads are dropped.
   */
  fallbackDurationSec?: number;
};

export function useMovieWatchProgressSaver({
  movieId,
  playing,
  currentTimeRef,
  durationRef,
  fallbackDurationSec,
}: MovieWatchProgressSaverOptions) {
  // Null until the first save; the chain starts from a resolved promise then.
  const pendingSaveRef = useRef<Promise<void> | null>(null);
  const fallbackDurationRef = useRef(fallbackDurationSec ?? 0);
  const saveSessionIdRef = useRef<string | null>(null);
  const saveSequenceRef = useRef(0);

  if (saveSessionIdRef.current === null) {
    saveSessionIdRef.current = createPlaybackSessionId();
  }

  useEffect(() => {
    fallbackDurationRef.current = fallbackDurationSec ?? 0;
  }, [fallbackDurationSec]);

  const effectiveDurationSec = useCallback(() => {
    if (durationRef.current > 0) return durationRef.current;
    return fallbackDurationRef.current;
  }, [durationRef]);

  const queueProgressSave = useCallback(
    (progressSec: number, durationSec: number) => {
      const saveSequence = ++saveSequenceRef.current;
      const save = (pendingSaveRef.current ?? Promise.resolve()).then(() =>
        persistMovieWatchProgress(
          movieId,
          progressSec,
          durationSec,
          saveSessionIdRef.current!,
          saveSequence,
        ),
      );
      pendingSaveRef.current = save.catch(() => {});
      return save;
    },
    [movieId],
  );

  useEffect(() => {
    if (!playing) return;
    const interval = window.setInterval(async () => {
      try {
        await queueProgressSave(
          currentTimeRef.current,
          effectiveDurationSec(),
        );
      } catch {
        // Silent background save failure; pause/end handlers surface failures when needed.
      }
    }, MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS);
    return () => {
      window.clearInterval(interval);
    };
  }, [playing, currentTimeRef, effectiveDurationSec, queueProgressSave]);

  useEffect(() => {
    // On a real page close, visibilitychange (hidden) usually fires and then
    // pagehide follows; the dedupe window keeps that from double-saving while
    // still covering browsers/platforms that only deliver one of the two.
    let lastKeepalive: { progressSec: number; atMs: number } | null = null;

    const flushKeepalive = () => {
      const progressSec = currentTimeRef.current;
      const isDuplicate =
        lastKeepalive !== null &&
        Math.abs(lastKeepalive.progressSec - progressSec) < 1 &&
        Date.now() - lastKeepalive.atMs <
          MOVIE_WATCH_PROGRESS_KEEPALIVE_DEDUPE_MS;
      if (isDuplicate) return;

      const atMs = Date.now();
      const durationSec = effectiveDurationSec();
      const saveSequence = ++saveSequenceRef.current;
      lastKeepalive = { progressSec, atMs };
      void persistMovieWatchProgress(
        movieId,
        progressSec,
        durationSec,
        saveSessionIdRef.current!,
        saveSequence,
        { keepalive: true },
      );
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState !== "hidden") return;
      flushKeepalive();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("pagehide", flushKeepalive);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("pagehide", flushKeepalive);
    };
  }, [currentTimeRef, effectiveDurationSec, movieId]);

  const handlePauseSave = async () => {
    try {
      await queueProgressSave(
        currentTimeRef.current,
        effectiveDurationSec(),
      );
    } catch {
      // Best effort on pause; avoid interrupting playback UI with repeated toasts.
    }
  };

  const handleEndedSave = async () => {
    const durationSec = effectiveDurationSec();
    try {
      await queueProgressSave(durationSec, durationSec);
    } catch {
      showActionFailed(
        "save watch progress",
        "Unable to mark this movie as watched.",
      );
    }
  };

  const flushProgress = () =>
    queueProgressSave(currentTimeRef.current, effectiveDurationSec());

  return { handlePauseSave, handleEndedSave, flushProgress };
}
