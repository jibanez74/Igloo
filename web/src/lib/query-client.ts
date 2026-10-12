import { QueryClient } from "@tanstack/react-query";
import type { DefaultOptions } from "@tanstack/react-query";
import { ApiFailureError, isApiFailure } from "@/lib/is-api-failure";

/**
 * TanStack Query cache tuning (v5: `gcTime` replaces former `cacheTime`).
 *
 * - staleTime: while fresh, the query will not refetch on mount/window focus.
 * - gcTime: how long inactive cached data is kept after the last observer unmounts.
 *
 * Library / user-editable data uses shorter stale times; TMDB-style catalogs longer.
 */
const MIN = 60_000;
/** 1 min — current user and per-user flags such as likes. */
export const STALE_1M = MIN;
export const STALE_LIST = 2 * MIN;
export const STALE_CATALOG = 5 * MIN;
export const STALE_THEATERS = 10 * MIN;
export const STALE_TECH = 10 * MIN;
export const STALE_30S = 30_000;

/** Every query's default; factories only override it to keep data longer (or not at all). */
const GC_DEFAULT = 10 * MIN;
export const GC_LONG = 30 * MIN;

/**
 * How long a failure stays fresh. apiRequest resolves a failed request as an
 * `{ error: true }` envelope instead of throwing, so React Query caches it
 * like data. A failure therefore gets this short window instead of the
 * query's own staleTime: long enough that the observers mounting together (a
 * loader then its page, every track row sharing the liked-ids query) do not
 * each refetch it, short enough that the next mount or window focus retries.
 */
const FAILURE_STALE_MS = 10_000;

/**
 * The staleTime for a query that resolves an API envelope: `ms` for a
 * success, at most FAILURE_STALE_MS for a failure.
 */
export function freshFor(ms: number) {
  return (query: { state: { data: unknown } }) =>
    isApiFailure(query.state.data) ? Math.min(ms, FAILURE_STALE_MS) : ms;
}

/**
 * Retry only sees thrown errors, and nearly every query caches its failure
 * envelope instead. The infinite lists throw ApiFailureError: a server or
 * network failure (the client stamps a network error 500) is tried twice
 * more, a 4xx never. Anything else that throws is not retried.
 */
function retryServerFailures(failureCount: number, error: unknown) {
  return (
    failureCount < 2 &&
    error instanceof ApiFailureError &&
    (error.failure.status ?? 500) >= 500
  );
}

/** The app's QueryClient. `overrides` layer per section on top of the defaults. */
export function createQueryClient(overrides: DefaultOptions = {}) {
  return new QueryClient({
    defaultOptions: {
      ...overrides,
      queries: {
        gcTime: GC_DEFAULT,
        retry: retryServerFailures,
        ...overrides.queries,
      },
    },
  });
}
