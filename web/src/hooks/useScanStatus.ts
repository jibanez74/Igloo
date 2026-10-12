import { useEffect, useSyncExternalStore } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { movieScanStatusQueryOpts, musicScanStatusQueryOpts, showScanStatusQueryOpts } from "@/lib/query-opts";
import type { ScanStatusQueryOpts } from "@/lib/query-opts";
import { MOVIES_STATS_KEY, MUSIC_STATS_KEY, SHOWS_STATS_KEY } from "@/lib/constants";
import { invalidateLibraryQueries, type Library } from "@/lib/library-cache";
import type { MovieScanStatus, MusicScanStatus, ShowScanStatus } from "@/types/settings";

const subscribeVisibility = (onChange: () => void) => {
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
};
const isVisible = () => document.visibilityState !== "hidden";

type ScanStatusLike = Pick<MovieScanStatus, "state" | "phase" | "run_id" | "imported" | "updated" | "deleted">;

type ScanStatusOptions = {
  enabled?: boolean;
  watchIdle?: boolean;
};

type ScanStatusConfig<T extends ScanStatusLike & Record<string, unknown>> = {
  queryOptions: () => ScanStatusQueryOpts<T>;
  // statsKey is the library statistics query committed work refreshes; a
  // library without statistics leaves it unset.
  statsKey?: string;
  library: Library;
};

// Polls a scan report every two seconds while it runs and every ten seconds
// otherwise, pausing in hidden tabs. Without watchIdle the first request still
// goes out, so a scan started elsewhere is discovered on any route; polling
// then continues only while it runs. Committed work refreshes the library
// statistics; a new run, phase or terminal state refreshes the library lists.
function useScanStatus<T extends ScanStatusLike & Record<string, unknown>>(
  config: ScanStatusConfig<T>,
  { enabled = true, watchIdle = true }: ScanStatusOptions,
) {
  const visible = useSyncExternalStore(subscribeVisibility, isVisible);
  const queryClient = useQueryClient();
  const query = useQuery({
    ...config.queryOptions(),
    enabled: query =>
      enabled && visible && (watchIdle || query.state.status === "pending" || query.state.data?.state === "running"),
    refetchOnMount: "always",
    refetchInterval: query => visible
      ? query.state.data?.state === "running" ? 2_000 : 10_000
      : false,
    refetchIntervalInBackground: false,
  });
  const status = query.data;
  const committed = status ? status.imported + status.updated + status.deleted : 0;
  const statsKey = config.statsKey;

  useEffect(() => {
    if (committed > 0 && statsKey) {
      void queryClient.invalidateQueries({ queryKey: [statsKey] });
    }
  }, [queryClient, statsKey, committed]);

  const phase = status?.phase;
  const state = status?.state;
  const runId = status?.run_id;
  const library = config.library;
  useEffect(() => {
    if (runId) invalidateLibraryQueries(queryClient, library);
  }, [queryClient, library, phase, state, runId]);

  return query;
}

const MOVIE_SCAN: ScanStatusConfig<MovieScanStatus> = {
  queryOptions: movieScanStatusQueryOpts,
  statsKey: MOVIES_STATS_KEY,
  library: "movies",
};

const MUSIC_SCAN: ScanStatusConfig<MusicScanStatus> = {
  queryOptions: musicScanStatusQueryOpts,
  statsKey: MUSIC_STATS_KEY,
  library: "music",
};

const SHOW_SCAN: ScanStatusConfig<ShowScanStatus> = {
  queryOptions: showScanStatusQueryOpts,
  statsKey: SHOWS_STATS_KEY,
  library: "shows",
};

export function useMovieScanStatus(options: ScanStatusOptions = {}) {
  return useScanStatus(MOVIE_SCAN, options);
}

export function useMusicScanStatus(options: ScanStatusOptions = {}) {
  return useScanStatus(MUSIC_SCAN, options);
}

export function useShowScanStatus(options: ScanStatusOptions = {}) {
  return useScanStatus(SHOW_SCAN, options);
}
