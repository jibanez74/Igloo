import type { DataTag, QueryClient, QueryKey } from "@tanstack/react-query";
import type { MutationSession } from "@/hooks/useSessionMutation";

/** What a query held before an optimistic write, for restoreOptimistic. */
export type OptimisticSnapshot<TData> = { previous: TData | undefined };

/**
 * An optimistic mutation's onMutate for a whole query entry: stops the
 * entry's fetch in flight (it would land over the write), then replaces the
 * entry with `next(previous)` (left alone when that is undefined: there was
 * nothing to edit). Resolves with the entry as it was, or with
 * undefined, having written nothing, when the session ended while the fetch
 * was being cancelled: the cache then belongs to whoever signs in next.
 *
 * For an entry that several mutations edit at once (a season's rows, the
 * liked ids), restore the one row instead: putting back the whole snapshot
 * would undo the others.
 */
export async function writeOptimistically<TData, TError>(
  queryClient: QueryClient,
  queryKey: DataTag<QueryKey, TData, TError>,
  session: MutationSession,
  next: (previous: TData | undefined) => TData | undefined,
): Promise<OptimisticSnapshot<TData> | undefined> {
  await queryClient.cancelQueries({ queryKey });
  if (session.ended()) return undefined;
  const previous = queryClient.getQueryData(queryKey);
  const written = next(previous);
  if (written !== undefined) queryClient.setQueryData(queryKey, written);
  return { previous };
}

/**
 * Undoes writeOptimistically after a refusal or a failure. With nothing cached
 * before the write, the real state is unknown, so the entry refetches rather
 * than keeping the optimistic guess.
 */
export function restoreOptimistic<TData, TError>(
  queryClient: QueryClient,
  queryKey: DataTag<QueryKey, TData, TError>,
  snapshot: OptimisticSnapshot<TData> | undefined,
) {
  if (snapshot === undefined) return;
  if (snapshot.previous === undefined) {
    void queryClient.invalidateQueries({ queryKey });
    return;
  }
  queryClient.setQueryData(queryKey, snapshot.previous);
}
