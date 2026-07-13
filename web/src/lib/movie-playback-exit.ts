import type { QueryClient } from "@tanstack/react-query";
import {
  CONTINUE_WATCHING_KEY,
  MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS,
  MOVIE_WATCH_PROGRESS_KEY,
} from "@/lib/constants";

type PlaybackLocation = {
  routeId: string;
  pathname: string;
};

type MoviePlaybackExitSyncOptions = {
  pausePlayback: () => void;
  flushProgress: () => Promise<void>;
  refreshWatchQueries: () => Promise<unknown>;
  onSaveError: () => void;
};

// Both watch-related caches must refresh on exit: continue-watching feeds the
// home page, and the movie's watch-progress entry (staleTime 30s) feeds the
// Resume dialog when the play page is reopened right away.
export function refreshMovieWatchQueries(
  queryClient: QueryClient,
  movieId: number,
) {
  return Promise.all([
    queryClient.invalidateQueries({
      queryKey: [CONTINUE_WATCHING_KEY],
      refetchType: "all",
    }),
    queryClient.invalidateQueries({
      queryKey: [MOVIE_WATCH_PROGRESS_KEY, movieId],
    }),
  ]);
}

export function staysOnCurrentMoviePlayback(
  current: PlaybackLocation,
  next: PlaybackLocation,
) {
  return current.routeId === next.routeId && current.pathname === next.pathname;
}

export async function synchronizeMoviePlaybackExit({
  pausePlayback,
  flushProgress,
  refreshWatchQueries,
  onSaveError,
}: MoviePlaybackExitSyncOptions) {
  pausePlayback();

  const synchronization = (async () => {
    try {
      await flushProgress();
    } catch {
      onSaveError();
    }

    try {
      await refreshWatchQueries();
    } catch {
      // Navigation should not be trapped if refreshing the queries fails.
    }
  })();

  let timeoutId: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<void>((resolve) => {
    timeoutId = setTimeout(resolve, MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS);
  });

  try {
    await Promise.race([synchronization, timeout]);
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId);
  }
}
