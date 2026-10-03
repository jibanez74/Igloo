import { useState } from "react";
import type { PrerollTrailer } from "@/types";

type PrerollQueueOptions = {
  /** The trailer queue; undefined while the request is still in flight. */
  trailers: PrerollTrailer[] | undefined;
  /** True when the queue request failed or timed out: the movie starts. */
  loadFailed: boolean;
};

/**
 * The pre-roll state machine: which trailer plays, and when the movie
 * starts. Every exit (the last trailer ending, a skip past it, "Start movie",
 * an empty or failed queue) lands on `done`; a trailer that fails to embed
 * is skipped silently like any other advance.
 */
export function usePrerollQueue({ trailers, loadFailed }: PrerollQueueOptions) {
  const [index, setIndex] = useState(0);
  const [skippedAll, setSkippedAll] = useState(false);

  const loading = trailers === undefined && !loadFailed;
  const total = trailers?.length ?? 0;
  const exhausted = !loading && index >= total;
  const done = skippedAll || loadFailed || exhausted;
  const current = done ? null : (trailers?.[index] ?? null);

  const advance = () => setIndex(previous => previous + 1);
  const skipAll = () => setSkippedAll(true);

  return {
    loading,
    done,
    current,
    position: index + 1,
    total,
    isLast: !loading && index === total - 1,
    skip: advance,
    skipAll,
    handleTrailerEnded: advance,
    handleTrailerError: advance,
  };
}
