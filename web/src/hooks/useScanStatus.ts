import { useEffect, useRef, useSyncExternalStore } from "react";
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

type ScanStatusLike = Pick<
  MovieScanStatus,
  "state" | "phase" | "scope" | "run_id" | "catalog_revision"
>;

type ScanStatusOptions = {
  enabled?: boolean;
  background?: boolean;
};

type ScanStatusConfig<T extends ScanStatusLike & Record<string, unknown>> = {
  queryOptions: () => ScanStatusQueryOpts<T>;
  // statsKey is the library statistics query committed work refreshes; a
  // library without statistics leaves it unset.
  statsKey?: string;
  library: Library;
};

// Polls a scan report every two seconds while it runs and every ten seconds
// otherwise (or while the report keeps failing), pausing in hidden tabs. In the
// background, off the libraries page, an idle report is polled once a minute,
// so a scan started elsewhere or a run of watcher-detected changes is still
// noticed on any route. A new catalog revision refreshes the
// library statistics; a new run, phase or terminal state of a library scan
// refreshes the library lists. A run of watcher-detected changes is often one
// file and over within a poll, and several can finish between two polls, so
// the report seen may not be the run that changed anything. Such runs refresh
// the lists once one has ended and the revision, which counts every change
// across runs, moved since the last refresh.
function useScanStatus<T extends ScanStatusLike & Record<string, unknown>>(
  config: ScanStatusConfig<T>,
  { enabled = true, background = false }: ScanStatusOptions,
) {
  const visible = useSyncExternalStore(subscribeVisibility, isVisible);
  const queryClient = useQueryClient();
  const query = useQuery({
    ...config.queryOptions(),
    enabled: enabled && visible,
    refetchOnMount: "always",
    refetchInterval: query => {
      if (!visible) return false;
      // A failing report (the server restarting, the session gone) keeps its
      // last "running" data; retrying that every two seconds for as long as
      // the failure lasts helps nobody, so it backs off to ten seconds, also
      // in the background, where a scan it shows running may finish.
      if (query.state.status === "error") return 10_000;
      if (query.state.data?.state === "running") return 2_000;
      return background ? 60_000 : 10_000;
    },
    refetchIntervalInBackground: false,
  });
  const status = query.data;
  const revision = status?.catalog_revision ?? 0;
  const statsKey = config.statsKey;
  const phase = status?.phase;
  const state = status?.state;
  const runId = status?.run_id;
  const library = config.library;
  const watcherRun = status?.scope === "changes";
  const settledRevision = watcherRun && state !== "running" ? revision : undefined;

  // The first report sets the baseline. The server keeps the last run's id,
  // phase and revision after it ends, so refetching on that report would
  // replay every library query right after the pages loaded them. The client
  // cannot tell whether the run it describes ended before or after those
  // loads, so the first report marks the library stale without refetching:
  // what is on screen refetches at its next mount or window focus, and only
  // work reported after the baseline refetches at once. A list still loading
  // when that report lands keeps the answer it is given: its success clears
  // the mark (as it always did), a window of milliseconds at boot.
  const seenRevision = useRef<number | null>(null);
  useEffect(() => {
    if (status === undefined) return;
    const previous = seenRevision.current;
    seenRevision.current = revision;
    if (revision > 0 && revision !== previous && statsKey) {
      void queryClient.invalidateQueries(
        previous === null
          ? { queryKey: [statsKey], refetchType: "none" }
          : { queryKey: [statsKey] },
      );
    }
  }, [queryClient, statsKey, revision, status]);

  const seenRun = useRef<string | null>(null);
  const listedRevision = useRef(0);
  useEffect(() => {
    if (status === undefined) return;
    const run = `${runId ?? ""}|${phase ?? ""}|${state ?? ""}|${settledRevision ?? ""}`;
    const first = seenRun.current === null;
    if (run === seenRun.current) return;
    seenRun.current = run;
    if (first) {
      if (settledRevision !== undefined) listedRevision.current = settledRevision;
      if (runId) invalidateLibraryQueries(queryClient, library, { refetchType: "none" });
      return;
    }
    if (!runId) return;
    if (watcherRun) {
      if (settledRevision === undefined || settledRevision === listedRevision.current) return;
      listedRevision.current = settledRevision;
    }
    invalidateLibraryQueries(queryClient, library);
  }, [queryClient, library, phase, state, runId, watcherRun, settledRevision, status]);

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
