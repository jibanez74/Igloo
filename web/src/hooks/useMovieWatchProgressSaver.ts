import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";
import { persistMovieWatchProgress } from "@/lib/movie-playback";
import { MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS } from "@/lib/constants";
import { showActionFailed } from "@/lib/toast-helpers";

type MovieWatchProgressSaverOptions = {
  movieId: number;
  playing: boolean;
  currentTimeRef: RefObject<number>;
  durationRef: RefObject<number>;
};

export function useMovieWatchProgressSaver({
  movieId,
  playing,
  currentTimeRef,
  durationRef,
}: MovieWatchProgressSaverOptions) {
  const pendingSaveRef = useRef<Promise<void>>(Promise.resolve());

  const queueProgressSave = useCallback(
    (progressSec: number, durationSec: number) => {
      const save = pendingSaveRef.current.then(() =>
        persistMovieWatchProgress(movieId, progressSec, durationSec),
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
          durationRef.current,
        );
      } catch {
        // Silent background save failure; pause/end handlers surface failures when needed.
      }
    }, MOVIE_WATCH_PROGRESS_SAVE_INTERVAL_MS);
    return () => {
      window.clearInterval(interval);
    };
  }, [playing, currentTimeRef, durationRef, queueProgressSave]);

  useEffect(() => {
    const handlePageHide = () => {
      void persistMovieWatchProgress(
        movieId,
        currentTimeRef.current,
        durationRef.current,
        { keepalive: true },
      );
    };
    window.addEventListener("pagehide", handlePageHide);
    return () => {
      window.removeEventListener("pagehide", handlePageHide);
    };
  }, [movieId, currentTimeRef, durationRef]);

  const handlePauseSave = async () => {
    try {
      await queueProgressSave(
        currentTimeRef.current,
        durationRef.current,
      );
    } catch {
      // Best effort on pause; avoid interrupting playback UI with repeated toasts.
    }
  };

  const handleEndedSave = async () => {
    try {
      await queueProgressSave(
        durationRef.current,
        durationRef.current,
      );
    } catch {
      showActionFailed(
        "save watch progress",
        "Unable to mark this movie as watched.",
      );
    }
  };

  const flushProgress = () =>
    queueProgressSave(currentTimeRef.current, durationRef.current);

  return { handlePauseSave, handleEndedSave, flushProgress };
}
