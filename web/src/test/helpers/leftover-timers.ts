/**
 * Timers a test file leaves pending.
 *
 * A test can end while a timer it started is still waiting: the router's
 * pending-UI timer for a loader that never settles, the theme switch's
 * cross-fade. Vitest tears the jsdom globals down when the file ends, but the
 * worker can live a little longer, and a timer that fires in between touches
 * `window` or `document` that are no longer there. That fails the whole run
 * with an unhandled error although every test passed.
 *
 * `trackTimers` wraps `setTimeout` on the given scope and returns
 * `clearLeftovers`, which cancels every timer that has not fired yet. The
 * test setup calls it after each file.
 */
type TimerScope = Pick<typeof globalThis, "setTimeout" | "clearTimeout">;

export function trackTimers(scope: TimerScope = globalThis) {
  const pending = new Set<ReturnType<typeof setTimeout>>();
  const original = scope.setTimeout;

  const tracked = (
    handler: (...args: unknown[]) => void,
    delay?: number,
    ...args: unknown[]
  ) => {
    const id = original(
      () => {
        pending.delete(id);
        handler(...args);
      },
      delay,
    );
    pending.add(id);
    return id;
  };
  scope.setTimeout = Object.assign(tracked, original) as typeof setTimeout;

  return {
    /** How many timers are still waiting. */
    pendingCount: () => pending.size,
    /** Cancels every timer that has not fired; returns how many there were. */
    clearLeftovers() {
      const count = pending.size;
      for (const id of pending) scope.clearTimeout(id);
      pending.clear();
      return count;
    },
    /** Puts the original `setTimeout` back. */
    restore() {
      scope.setTimeout = original;
    },
  };
}
