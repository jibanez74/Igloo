import { useEffect } from "react";
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
  useEffect(() => {
    if (!playing) return;
    const interval = window.setInterval(async () => {
      try {
        await persistMovieWatchProgress(
          movieId,
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
  }, [movieId, playing, currentTimeRef, durationRef]);

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

  // In-app navigation (e.g. the Back button) fires neither pagehide nor a
  // reliable pause event, so flush the latest position on unmount to avoid
  // dropping progress accrued since the last interval tick.
  useEffect(() => {
    return () => {
      // Intentionally read the live ref values at unmount time (not the values
      // captured when the effect ran) so we flush the most recent position.
      void persistMovieWatchProgress(
        movieId,
        // eslint-disable-next-line react-hooks/exhaustive-deps
        currentTimeRef.current,
        // eslint-disable-next-line react-hooks/exhaustive-deps
        durationRef.current,
      ).catch(() => {
        // Best-effort flush on unmount; nothing left to surface the error to.
      });
    };
  }, [movieId, currentTimeRef, durationRef]);

  const handlePauseSave = async () => {
    try {
      await persistMovieWatchProgress(
        movieId,
        currentTimeRef.current,
        durationRef.current,
      );
    } catch {
      // Best effort on pause; avoid interrupting playback UI with repeated toasts.
    }
  };

  const handleEndedSave = async () => {
    try {
      await persistMovieWatchProgress(
        movieId,
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

  return { handlePauseSave, handleEndedSave };
}
