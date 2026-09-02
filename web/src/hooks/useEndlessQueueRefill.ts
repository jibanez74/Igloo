import { useEffect, useEffectEvent, useRef } from "react";

type EndlessQueueRefillOptions<T> = {
  // Whether this queue mode is active and still has anything left to fetch.
  enabled: boolean;

  // Identifies the queue the refill belongs to. It is handed back to onAppend
  // so a batch that finishes after the user started a different queue can be
  // dropped by the state updater rather than spliced into the wrong queue.
  queueId: number;

  // How many tracks remain ahead of the current one. Pass a negative number
  // when the current track is not in the queue.
  tracksAhead: number;

  // Start fetching once fewer than this many tracks remain ahead.
  lookahead: number;

  // Fetch one batch. Resolve with an empty array for "nothing to add"; a
  // rejection is swallowed and the user keeps the queue they already have.
  fetchBatch: () => Promise<T[]>;

  onAppend: (batch: T[], queueId: number) => void;
};

// Shared driver for the endless queues (library shuffle, play all): watch how
// much runway is left ahead of the current track and top the queue up before it
// runs out.
//
// There is deliberately no effect cleanup. A batch that has already been
// fetched is paid for, and tearing the effect down (which happens on every
// track advance, since tracksAhead is a dependency) used to discard it and then
// bail out of the retry because the in-flight guard was still set — burning a
// request and narrowing the lookahead margin. Whether a finished batch is still
// wanted is a question only the state updater can answer, so it is asked there
// via queueId instead.
export function useEndlessQueueRefill<T>({
  enabled,
  queueId,
  tracksAhead,
  lookahead,
  fetchBatch,
  onAppend,
}: EndlessQueueRefillOptions<T>) {
  // The queue a fetch is currently in flight for, or null when idle. Keyed by
  // queue rather than a plain boolean so a refill for a queue the user has
  // since replaced does not lock the new one out: that batch will be discarded
  // on arrival anyway, and the new queue may be short enough to need topping up
  // immediately. A ref rather than state — flipping it must not re-render.
  const inFlightQueueIdRef = useRef<number | null>(null);

  const runRefill = useEffectEvent(async (activeQueueId: number) => {
    inFlightQueueIdRef.current = activeQueueId;

    let batch: T[] = [];
    try {
      batch = await fetchBatch();
    } catch {
      // Swallowed on purpose - playback continues with the current queue.
    }

    // Only the fetch that is still the in-flight one may clear the flag; a
    // straggler from a replaced queue must not unlock a live fetch. Cleared
    // before the append so the re-render it causes sees an idle hook.
    if (inFlightQueueIdRef.current === activeQueueId) {
      inFlightQueueIdRef.current = null;
    }

    if (batch.length > 0) {
      onAppend(batch, activeQueueId);
    }
  });

  useEffect(() => {
    const shouldFetch =
      enabled &&
      tracksAhead >= 0 &&
      tracksAhead < lookahead &&
      inFlightQueueIdRef.current !== queueId;

    if (!shouldFetch) {
      return;
    }

    void runRefill(queueId);
  }, [enabled, queueId, tracksAhead, lookahead]);
}
