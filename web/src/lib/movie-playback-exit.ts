import { MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS } from "@/lib/constants";

type PlaybackLocation = {
  routeId: string;
  pathname: string;
};

type MoviePlaybackExitSyncOptions = {
  pausePlayback: () => void;
  flushProgress: () => Promise<void>;
  refreshContinueWatching: () => Promise<unknown>;
  onSaveError: () => void;
};

export function staysOnCurrentMoviePlayback(
  current: PlaybackLocation,
  next: PlaybackLocation,
) {
  return current.routeId === next.routeId && current.pathname === next.pathname;
}

export async function synchronizeMoviePlaybackExit({
  pausePlayback,
  flushProgress,
  refreshContinueWatching,
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
      await refreshContinueWatching();
    } catch {
      // Navigation should not be trapped if refreshing the list fails.
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
