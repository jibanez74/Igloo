import { useState } from "react";

/**
 * A query's envelope as a surface already in use should see it: the latest
 * success for the same `key` stands in for a failure that comes after it.
 *
 * The fetch helpers never throw, so React Query caches a failed request's
 * `{ error: true }` envelope as data, and a background refetch that fails (a
 * window focus or a reconnect while the server restarts) replaces good data
 * with it. A player deriving "not found" or "playback failed" from that
 * envelope tore down the film being watched. Before any success for the key
 * the envelope passes through unchanged, so a first load still reports its
 * failure: the route components outlive a change of item, and the next
 * episode must never be shown as the one that just ended.
 */
export function useLastSuccess<T extends { error: boolean }>(
  data: T | undefined,
  key: string | number,
): T | undefined {
  const [lastSuccess, setLastSuccess] = useState<{
    key: string | number;
    data: T;
  } | null>(null);
  if (
    data?.error === false &&
    (lastSuccess?.data !== data || lastSuccess.key !== key)
  ) {
    setLastSuccess({ key, data });
  }
  return data?.error === true && lastSuccess?.key === key
    ? lastSuccess.data
    : data;
}
