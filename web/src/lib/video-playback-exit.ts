import type { QueryClient } from "@tanstack/react-query";
import {
  CONTINUE_WATCHING_KEY,
  EPISODE_KEY,
  PLAYBACK_EXIT_SYNC_TIMEOUT_MS,
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

// The lists that show this media's progress alongside it, and so must not keep
// the pre-playback position. Home's continue-watching row carries both kinds;
// an episode also sits in its season's episode rows. Every caller that
// refreshes or clears watch progress reads the set from here - a reset that
// skipped continue-watching left an episode resumable on Home.
export function watchListQueryKeys(media: PlaybackMediaRef) {
  return media.kind === "movie"
    ? [[CONTINUE_WATCHING_KEY]]
    : [[CONTINUE_WATCHING_KEY], [SHOW_SEASON_EPISODES_KEY]];
}

// An episode's entry carries `next_episode`, the episode after it with this
// user's progress, which the up-next card resumes from. A change to one
// episode's progress therefore dates the entry of the episode before it,
// which the caller cannot name, so every cached episode is marked stale.
// Nothing refetches here: a play page refetches its entry when it mounts,
// and the episode playing now keeps the entry it is showing.
export function markNextEpisodesStale(
  queryClient: QueryClient,
  media: PlaybackMediaRef,
) {
  if (media.kind !== "episode") return Promise.resolve();
  return queryClient.invalidateQueries({
    queryKey: [EPISODE_KEY],
    refetchType: "none",
  });
}

// Every watch-related cache must refresh on exit: the media's own
// watch-progress entry (staleTime 30s) feeds the Resume dialog when the play
// page is reopened right away, plus the lists above. Exit refetches the lists
// even while inactive, since the page being returned to is not mounted yet; a
// toggle on a mounted page passes "active" so it only marks the rest stale.
export function refreshWatchQueries(
  queryClient: QueryClient,
  media: PlaybackMediaRef,
  { refetchType = "all" }: { refetchType?: "all" | "active" } = {},
) {
  return Promise.all([
    ...watchListQueryKeys(media).map(queryKey =>
      queryClient.invalidateQueries({ queryKey, refetchType }),
    ),
    queryClient.invalidateQueries({
      queryKey: mediaWatchProgressQueryKey(media),
    }),
    markNextEpisodesStale(queryClient, media),
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
    timeoutId = setTimeout(resolve, PLAYBACK_EXIT_SYNC_TIMEOUT_MS);
  });

  try {
    await Promise.race([synchronization, timeout]);
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId);
  }
}
