import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHlsCapacityRetry } from "@/hooks/useHlsCapacityRetry";
import { HLS_CAPACITY_RETRY_MAX_ATTEMPTS } from "@/lib/constants";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

type HookProps = Parameters<typeof useHlsCapacityRetry>[0];

function renderRetry(overrides: Partial<HookProps> = {}) {
  const onRetry = vi.fn();
  const onMaxAttempts = vi.fn();
  const initialProps: HookProps = {
    streamWindowKey: "movie:1",
    onRetry,
    onMaxAttempts,
    ...overrides,
  };
  const rendered = renderHook(props => useHlsCapacityRetry(props), {
    initialProps,
  });
  return { ...rendered, onRetry, onMaxAttempts, initialProps };
}

describe("useHlsCapacityRetry", () => {
  it("waits for the Retry-After delay before retrying", async () => {
    const { result, onRetry } = renderRetry();

    act(() => {
      result.current.handleCapacityBusy(7);
    });

    expect(result.current.waitingForCapacity).toBe(true);
    expect(onRetry).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_999);
    });
    expect(onRetry).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(onRetry).toHaveBeenCalledOnce();
    expect(result.current.waitingForCapacity).toBe(false);
  });

  it("ignores duplicate busy signals while a retry is scheduled", async () => {
    const { result, onRetry } = renderRetry();

    act(() => {
      result.current.handleCapacityBusy(5);
      result.current.handleCapacityBusy(5);
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("reports max attempts once the budget is exhausted", async () => {
    const { result, onRetry, onMaxAttempts } = renderRetry();

    for (let i = 0; i < HLS_CAPACITY_RETRY_MAX_ATTEMPTS; i++) {
      act(() => {
        result.current.handleCapacityBusy(1);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1_000);
      });
    }
    expect(onRetry).toHaveBeenCalledTimes(HLS_CAPACITY_RETRY_MAX_ATTEMPTS);
    expect(onMaxAttempts).not.toHaveBeenCalled();

    act(() => {
      result.current.handleCapacityBusy(1);
    });

    expect(onMaxAttempts).toHaveBeenCalledOnce();
    expect(result.current.waitingForCapacity).toBe(false);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(onRetry).toHaveBeenCalledTimes(HLS_CAPACITY_RETRY_MAX_ATTEMPTS);
  });

  it("resets the attempt budget when the stream window changes", async () => {
    const { result, rerender, onRetry, onMaxAttempts, initialProps } =
      renderRetry();

    for (let i = 0; i < HLS_CAPACITY_RETRY_MAX_ATTEMPTS; i++) {
      act(() => {
        result.current.handleCapacityBusy(1);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1_000);
      });
    }

    rerender({ ...initialProps, streamWindowKey: "movie:1:start:600" });

    act(() => {
      result.current.handleCapacityBusy(1);
    });

    expect(onMaxAttempts).not.toHaveBeenCalled();
    expect(result.current.waitingForCapacity).toBe(true);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    expect(onRetry).toHaveBeenCalledTimes(HLS_CAPACITY_RETRY_MAX_ATTEMPTS + 1);
  });

  it("cancels a scheduled retry when the stream window changes", async () => {
    const { result, rerender, onRetry, initialProps } = renderRetry();

    act(() => {
      result.current.handleCapacityBusy(5);
    });
    expect(result.current.waitingForCapacity).toBe(true);

    rerender({ ...initialProps, streamWindowKey: "movie:2" });

    expect(result.current.waitingForCapacity).toBe(false);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(onRetry).not.toHaveBeenCalled();
  });
});
