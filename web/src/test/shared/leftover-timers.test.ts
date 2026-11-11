import { describe, expect, it, vi } from "vitest";
import { trackTimers } from "@/test/helpers/leftover-timers";

const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// Its own scope, so the test does not touch the tracker the setup file put
// on the real globals.
function scope() {
  return {
    setTimeout: globalThis.setTimeout,
    clearTimeout: globalThis.clearTimeout,
  };
}

describe("trackTimers", () => {
  it("cancels the timers that have not fired, so they never run", async () => {
    const timers = scope();
    const tracker = trackTimers(timers);
    const late = vi.fn();

    timers.setTimeout(late, 20);
    timers.setTimeout(late, 30);
    expect(tracker.pendingCount()).toBe(2);

    expect(tracker.clearLeftovers()).toBe(2);
    expect(tracker.pendingCount()).toBe(0);
    await wait(60);
    expect(late).not.toHaveBeenCalled();
  });

  it("lets a timer fire with its arguments and then forgets it", async () => {
    const timers = scope();
    const tracker = trackTimers(timers);
    const handler = vi.fn();

    timers.setTimeout(handler, 5, "a", 2);
    await wait(40);

    expect(handler).toHaveBeenCalledExactlyOnceWith("a", 2);
    expect(tracker.pendingCount()).toBe(0);
    expect(tracker.clearLeftovers()).toBe(0);
  });

  it("still counts a timer the caller cleared itself, and clearing it again is harmless", async () => {
    const timers = scope();
    const tracker = trackTimers(timers);
    const handler = vi.fn();

    const id = timers.setTimeout(handler, 10);
    timers.clearTimeout(id);
    // Clearing an already-cleared timer is harmless, so the tracker keeps the
    // id until the file ends rather than wrapping clearTimeout too.
    expect(tracker.clearLeftovers()).toBe(1);
    await wait(30);
    expect(handler).not.toHaveBeenCalled();
  });

  it("puts the original setTimeout back", () => {
    const timers = scope();
    const original = timers.setTimeout;
    const tracker = trackTimers(timers);

    expect(timers.setTimeout).not.toBe(original);
    tracker.restore();
    expect(timers.setTimeout).toBe(original);
  });
});
