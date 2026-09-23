import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHlsSessionRecovery } from "@/hooks/useHlsSessionRecovery";
import {
  HLS_SESSION_LOST_INCIDENT_WINDOW_MS,
  HLS_SESSION_LOST_MAX_ATTEMPTS,
  HLS_SESSION_LOST_MIN_INTERVAL_MS,
} from "@/lib/constants";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

function renderRecovery() {
  const onRecover = vi.fn();
  const onMaxAttempts = vi.fn();
  const rendered = renderHook(() =>
    useHlsSessionRecovery({ onRecover, onMaxAttempts }),
  );
  return { ...rendered, onRecover, onMaxAttempts };
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("useHlsSessionRecovery", () => {
  it("recovers at once on the first lost session", () => {
    const { result, onRecover } = renderRecovery();

    act(() => {
      result.current.handleSessionLost(42);
    });

    expect(onRecover).toHaveBeenCalledExactlyOnceWith(42);
    expect(result.current.recoveryAttempt).toBe(1);
  });

  // The player reports each lost session once, so a report that arrives too
  // soon is the next failure. Dropping it left the player stalled with
  // nothing scheduled.
  it("spaces a loss that follows too soon instead of dropping it", async () => {
    const { result, onRecover } = renderRecovery();

    act(() => {
      result.current.handleSessionLost(42);
    });
    await advance(500);
    act(() => {
      result.current.handleSessionLost(42);
    });
    expect(onRecover).toHaveBeenCalledOnce();

    await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS - 500 - 1);
    expect(onRecover).toHaveBeenCalledOnce();
    await advance(1);
    expect(onRecover).toHaveBeenCalledTimes(2);
  });

  // The budget used to reset on every stream-window change, and a recovery
  // changes the window itself, so a 404 that kept coming back never ran out.
  it("gives up after the attempt budget when losses keep coming", async () => {
    const { result, onRecover, onMaxAttempts } = renderRecovery();

    for (let attempt = 0; attempt < HLS_SESSION_LOST_MAX_ATTEMPTS; attempt++) {
      act(() => {
        result.current.handleSessionLost(42);
      });
      await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS);
    }
    expect(onRecover).toHaveBeenCalledTimes(HLS_SESSION_LOST_MAX_ATTEMPTS);
    expect(onMaxAttempts).not.toHaveBeenCalled();

    act(() => {
      result.current.handleSessionLost(42);
    });

    expect(onRecover).toHaveBeenCalledTimes(HLS_SESSION_LOST_MAX_ATTEMPTS);
    expect(onMaxAttempts).toHaveBeenCalledOnce();
  });

  it("starts a new incident with a full budget after a quiet period", async () => {
    const { result, onRecover, onMaxAttempts } = renderRecovery();

    for (let attempt = 0; attempt < HLS_SESSION_LOST_MAX_ATTEMPTS; attempt++) {
      act(() => {
        result.current.handleSessionLost(42);
      });
      await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS);
    }
    await advance(HLS_SESSION_LOST_INCIDENT_WINDOW_MS);

    act(() => {
      result.current.handleSessionLost(3_600);
    });

    expect(onRecover).toHaveBeenCalledTimes(HLS_SESSION_LOST_MAX_ATTEMPTS + 1);
    expect(onRecover).toHaveBeenLastCalledWith(3_600);
    expect(onMaxAttempts).not.toHaveBeenCalled();
    // Keeps counting across incidents so every attempt is announced.
    expect(result.current.recoveryAttempt).toBe(
      HLS_SESSION_LOST_MAX_ATTEMPTS + 1,
    );
  });

  it("restores the budget and drops a pending retry on reset", async () => {
    const { result, onRecover, onMaxAttempts } = renderRecovery();

    act(() => {
      result.current.handleSessionLost(42);
      result.current.handleSessionLost(42);
    });
    act(() => {
      result.current.resetRecovery();
    });
    await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS);
    expect(onRecover).toHaveBeenCalledOnce();

    for (let attempt = 0; attempt < HLS_SESSION_LOST_MAX_ATTEMPTS; attempt++) {
      act(() => {
        result.current.handleSessionLost(42);
      });
      await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS);
    }
    expect(onRecover).toHaveBeenCalledTimes(HLS_SESSION_LOST_MAX_ATTEMPTS + 1);
    expect(onMaxAttempts).not.toHaveBeenCalled();
  });

  it("cancels a pending retry on unmount", async () => {
    const { result, onRecover, unmount } = renderRecovery();

    act(() => {
      result.current.handleSessionLost(42);
      result.current.handleSessionLost(42);
    });
    unmount();
    await advance(HLS_SESSION_LOST_MIN_INTERVAL_MS);

    expect(onRecover).toHaveBeenCalledOnce();
  });
});
