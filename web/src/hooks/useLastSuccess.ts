import { useState } from "react";

/**
 * A query's envelope as a surface already in use should see it: the latest
 * success stands in for a failure that comes after it.
 *
 * The fetch helpers never throw, so React Query caches a failed request's
 * `{ error: true }` envelope as data, and a background refetch that fails (a
 * window focus or a reconnect while the server restarts) replaces good data
 * with it. A player deriving "not found" or "playback failed" from that
 * envelope tore down the film being watched. Before any success the envelope
 * passes through unchanged, so a first load still reports its failure.
 */
export function useLastSuccess<T extends { error: boolean }>(
  data: T | undefined,
): T | undefined {
  const [lastSuccess, setLastSuccess] = useState<T | undefined>(undefined);
  if (data?.error === false && data !== lastSuccess) {
    setLastSuccess(data);
  }
  return data?.error === true && lastSuccess ? lastSuccess : data;
}
