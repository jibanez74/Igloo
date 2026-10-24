import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSettledHlsRebase } from "@/hooks/useSettledHlsRebase";
import { HLS_SEEK_SETTLE_MS } from "@/lib/constants";
import { nextHlsSeekInputOrder } from "@/lib/video-playback";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

function renderSettledRebase(
  resetKey = "movie:1:720p:0",
  newerElementSeekPending?: (requestOrder: number) => boolean,
) {
  const onRebase = vi.fn();
  const rendered = renderHook(
    ({ key, handler }) =>
      useSettledHlsRebase({
        resetKey: key,
        onRebase: handler,
        newerElementSeekPending,
      }),
    { initialProps: { key: resetKey, handler: onRebase } },
  );
  return { ...rendered, onRebase };
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

// The seek bar calls seek() on every pointer move, so a rebase that went out
// at once rebased (and started a transcode) mid-drag, and on every ~10 s of
// travel back past the new session's start.
describe("useSettledHlsRebase", () => {
  it("rebases once the controls have been quiet for the settle window", async () => {
    const { result, onRebase } = renderSettledRebase();

    act(() => {
      result.current.requestRebase(4000);
    });
    await advance(HLS_SEEK_SETTLE_MS - 1);
    expect(onRebase).not.toHaveBeenCalled();
    expect(result.current.isRebasePending()).toBe(true);

    await advance(1);
    expect(onRebase).toHaveBeenCalledExactlyOnceWith(4000);
    expect(result.current.isRebasePending()).toBe(false);
  });

  it("rebases a run of requests once, to where the run ended", async () => {
    const { result, onRebase } = renderSettledRebase();

    for (const target of [3980, 3970, 3960, 3950]) {
      act(() => {
        result.current.requestRebase(target);
      });
      await advance(HLS_SEEK_SETTLE_MS - 1);
    }
    await advance(1);

    expect(onRebase).toHaveBeenCalledExactlyOnceWith(3950);
  });

  it("drops a pending rebase the controls came back from", async () => {
    const { result, onRebase } = renderSettledRebase();

    act(() => {
      result.current.requestRebase(4000);
      result.current.cancelRebase();
    });
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).not.toHaveBeenCalled();
    expect(result.current.isRebasePending()).toBe(false);
  });

  it("drops a pending rebase when the stream window changes", async () => {
    const { result, rerender, onRebase } = renderSettledRebase();

    act(() => {
      result.current.requestRebase(4000);
    });
    rerender({ key: "movie:1:1080p:0", handler: onRebase });
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).not.toHaveBeenCalled();
    expect(result.current.isRebasePending()).toBe(false);
  });

  it("drops a pending rebase on unmount", async () => {
    const { result, unmount, onRebase } = renderSettledRebase();

    act(() => {
      result.current.requestRebase(4000);
    });
    unmount();
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).not.toHaveBeenCalled();
  });

  it("calls the handler current when the timer fires", async () => {
    const { result, rerender, onRebase } = renderSettledRebase();
    const laterHandler = vi.fn();

    act(() => {
      result.current.requestRebase(4000);
    });
    rerender({ key: "movie:1:720p:0", handler: laterHandler });
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).not.toHaveBeenCalled();
    expect(laterHandler).toHaveBeenCalledExactlyOnceWith(4000);
  });
});

// The page's controls and the element's own seeks settle in overlapping
// windows. The first timer to fire used to decide: a far click followed by a
// picture-in-picture scrub rebased to the click, and a drag's settled report
// cancelled a far click made after it.
describe("useSettledHlsRebase latest input", () => {
  it("yields to an element seek the viewer made after the request", async () => {
    let elementOrder: number | null = null;
    const { result, onRebase } = renderSettledRebase(
      "movie:1:720p:0",
      (requestOrder) => (elementOrder ?? 0) > requestOrder,
    );

    act(() => {
      result.current.requestRebase(4000);
    });
    elementOrder = nextHlsSeekInputOrder();
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).not.toHaveBeenCalled();
    expect(result.current.isRebasePending()).toBe(false);
  });

  it("rebases over an element seek the viewer made before the request", async () => {
    const elementOrder = nextHlsSeekInputOrder();
    const { result, onRebase } = renderSettledRebase(
      "movie:1:720p:0",
      (requestOrder) => elementOrder > requestOrder,
    );

    act(() => {
      result.current.requestRebase(4000);
    });
    await advance(HLS_SEEK_SETTLE_MS);

    expect(onRebase).toHaveBeenCalledExactlyOnceWith(4000);
  });

  it("tells an element report whether a newer request is waiting", () => {
    const { result } = renderSettledRebase();
    const before = nextHlsSeekInputOrder();

    expect(result.current.isRebasePendingAfter(before)).toBe(false);
    act(() => {
      result.current.requestRebase(4000);
    });
    const after = nextHlsSeekInputOrder();

    expect(result.current.isRebasePendingAfter(before)).toBe(true);
    expect(result.current.isRebasePendingAfter(after)).toBe(false);
    act(() => {
      result.current.cancelRebase();
    });
    expect(result.current.isRebasePendingAfter(before)).toBe(false);
  });
});
