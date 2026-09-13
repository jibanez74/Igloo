import type { QueryClient } from "@tanstack/react-query";
import {
  CONTINUE_WATCHING_KEY,
  MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS,
  SHOW_SEASON_EPISODES_KEY,
} from "@/lib/constants";
import { mediaWatchProgressQueryKey } from "@/lib/query-opts";
import type { PlaybackMediaRef } from "@/types/playback";

type PlaybackLocation = {
  routeId: string;
  pathname: string;
};

type PlaybackExitSyncOptions = {
  pausePlayback: () => void;
  flushProgress: () => Promise<void>;
  refreshWatchQueries: () => Promise<unknown>;
  onSaveError: () => void;
};

// Both watch-related caches must refresh on exit: the media's own
// watch-progress entry (staleTime 30s) feeds the Resume dialog when the play
// page is reopened right away, and the list that shows progress alongside it
// (continue-watching on Home for movies, the season's episode rows for TV)
// must not keep the pre-playback position.
export function refreshWatchQueries(
  queryClient: QueryClient,
  media: PlaybackMediaRef,
) {
  const listKey =
    media.kind === "movie"
      ? [CONTINUE_WATCHING_KEY]
      : [SHOW_SEASON_EPISODES_KEY];

  return Promise.all([
    queryClient.invalidateQueries({
      queryKey: listKey,
      refetchType: "all",
    }),
    queryClient.invalidateQueries({
      queryKey: mediaWatchProgressQueryKey(media),
    }),
  ]);
}

export function staysOnCurrentPlayback(
  current: PlaybackLocation,
  next: PlaybackLocation,
) {
  return current.routeId === next.routeId && current.pathname === next.pathname;
}

export async function synchronizePlaybackExit({
  pausePlayback,
  flushProgress,
  refreshWatchQueries,
  onSaveError,
}: PlaybackExitSyncOptions) {
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
