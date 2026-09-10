import { useEffect, useSyncExternalStore } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { movieScanStatusQueryOpts } from "@/lib/query-opts";
import { MOVIES_STATS_KEY } from "@/lib/constants";
import { invalidateMovieLibraryQueries } from "@/lib/movie-library-cache";

const subscribeVisibility = (onChange: () => void) => {
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
};
const isVisible = () => document.visibilityState !== "hidden";

export function useMovieScanStatus({ enabled = true, watchIdle = true }: {
  enabled?: boolean;
  watchIdle?: boolean;
} = {}) {
  const visible = useSyncExternalStore(subscribeVisibility, isVisible);
  const queryClient = useQueryClient();
  const query = useQuery({
    ...movieScanStatusQueryOpts(),
    enabled: query => enabled && visible && (watchIdle || query.state.data?.state === "running"),
    refetchOnMount: "always",
    refetchInterval: query => visible
      ? query.state.data?.state === "running" ? 2_000 : 10_000
      : false,
    refetchIntervalInBackground: false,
  });
  const status = query.data;
  const committed = status ? status.imported + status.updated + status.deleted : 0;

  useEffect(() => {
    if (committed > 0) {
      void queryClient.invalidateQueries({ queryKey: [MOVIES_STATS_KEY] });
    }
  }, [queryClient, committed]);

  const phase = status?.phase;
  const state = status?.state;
  const runId = status?.run_id;
  useEffect(() => {
    if (runId) invalidateMovieLibraryQueries(queryClient);
  }, [queryClient, phase, state, runId]);

  return query;
}
