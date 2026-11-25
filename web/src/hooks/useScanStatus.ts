import { useEffect, useRef, useSyncExternalStore } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { scanStatusQueryOpts } from "@/lib/query-opts";
import { MOVIES_STATS_KEY, MUSIC_STATS_KEY, SHOWS_STATS_KEY } from "@/lib/constants";
import { invalidateLibraryQueries, type Library } from "@/lib/library-cache";
import type { MovieScanStatus, MusicScanStatus, ScanStatuses } from "@/types/settings";

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

// Polls every library's scan report, in one request, every two seconds while
// any of them runs and every ten seconds otherwise (or while the report keeps
// failing), pausing in hidden tabs. In the background, off the libraries page,
// idle reports are polled once a minute, so a scan started elsewhere or a run
// of watcher-detected changes is still noticed on any route. Each library's
// report then refreshes that library's queries on its own; see
// useLibraryScanRefresh.
export function useScanStatus({ enabled = true, background = false }: ScanStatusOptions = {}) {
  const visible = useSyncExternalStore(subscribeVisibility, isVisible);
  const query = useQuery({
    ...scanStatusQueryOpts(),
    enabled: enabled && visible,
    refetchOnMount: "always",
    refetchInterval: query => {
      if (!visible) return false;
      // A failing report (the server restarting, the session gone) keeps its
      // last "running" data; retrying that every two seconds for as long as
      // the failure lasts helps nobody, so it backs off to ten seconds, also
      // in the background, where a scan it shows running may finish.
      if (query.state.status === "error") return 10_000;
      if (anyRunning(query.state.data)) return 2_000;
      return background ? 60_000 : 10_000;
    },
    refetchIntervalInBackground: false,
  });
  useLibraryScanRefresh(query.data?.movies, "movies", MOVIES_STATS_KEY);
  useLibraryScanRefresh(query.data?.music, "music", MUSIC_STATS_KEY);
  useLibraryScanRefresh(query.data?.shows, "shows", SHOWS_STATS_KEY);
  useMusicLookupsRefresh(query.data?.music);
  return query;
}

// The music library's background lookups count as running too, so their
// progress line moves at the scan's pace.
function anyRunning(statuses: ScanStatuses | undefined) {
  if (statuses === undefined) return false;
  const scanning = Object.values(statuses).some(status => status.state === "running");
  return scanning || statuses.music.lookups.state === "running";
}

// While a lookup pass runs, the music lists refresh at most this often.
const LOOKUP_REFRESH_MS = 15_000;

// The music worker changes album covers after the scan run that started it
// has ended, so the scan's own refresh never sees those changes. The lists
// refresh when a pass ends, at most every LOOKUP_REFRESH_MS while one runs
// and changes the catalog, and when the revision moves between two reports
// of the same finished run with the worker idle in both: a pass that ran
// entirely between two polls.
function useMusicLookupsRefresh(status: MusicScanStatus | undefined) {
  const queryClient = useQueryClient();
  const previous = useRef<{ runId: string; scanState: string; lookups: string; revision: number; listedAt: number; listedRevision: number } | null>(null);
  const runId = status?.run_id ?? "";
  const scanState = status?.state ?? "idle";
  const lookups = status?.lookups.state ?? "idle";
  const revision = status?.catalog_revision ?? 0;
  useEffect(() => {
    if (status === undefined) return;
    const last = previous.current;
    const now = Date.now();
    const current = { runId, scanState, lookups, revision, listedAt: last?.listedAt ?? now, listedRevision: last?.listedRevision ?? revision };
    previous.current = current;
    if (last === null) return;
    const moved = revision !== last.revision;
    const ended = last.lookups !== "idle" && lookups === "idle";
    const due = lookups === "running" && revision !== last.listedRevision && now - last.listedAt >= LOOKUP_REFRESH_MS;
    const sameFinishedRun = runId === last.runId && scanState !== "running" && last.scanState !== "running";
    const passBetweenPolls = sameFinishedRun && last.lookups === "idle" && lookups === "idle" && moved;
    if (!ended && !due && !passBetweenPolls) return;
    current.listedAt = now;
    current.listedRevision = revision;
    invalidateLibraryQueries(queryClient, "music");
  }, [queryClient, status, runId, scanState, lookups, revision]);
}

// A new catalog revision refreshes the library's statistics; a new run, phase
// or terminal state of a library scan refreshes its lists. A run of
// watcher-detected changes is often one file and over within a poll, and
// several can finish between two polls, so the report seen may not be the run
// that changed anything. Such runs refresh the lists once one has ended and the
// revision, which counts every change across runs, moved since the last
// refresh. Each library keeps its own baselines.
function useLibraryScanRefresh(status: ScanStatusLike | undefined, library: Library, statsKey: string) {
  const queryClient = useQueryClient();
  const revision = status?.catalog_revision ?? 0;
  const phase = status?.phase;
  const state = status?.state;
  const runId = status?.run_id;
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
    if (revision > 0 && revision !== previous) {
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
}
