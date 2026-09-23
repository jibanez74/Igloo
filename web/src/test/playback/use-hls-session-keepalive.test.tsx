import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHlsSessionKeepalive } from "@/hooks/useHlsSessionKeepalive";
import { HLS_SESSION_KEEPALIVE_INTERVAL_MS } from "@/lib/constants";

const streamUrl =
  "/api/movies/7/hls/remux/playlist.m3u8?playback_session=uuid&start=0";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useHlsSessionKeepalive", () => {
  it("refetches the manifest on the keepalive interval with credentials", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("#EXTM3U"));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));

    expect(fetchMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      streamUrl,
      expect.objectContaining({
        credentials: "include",
        signal: expect.any(AbortSignal),
      }),
    );

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("stops in a non-rendered error state and restarts after retry", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("#EXTM3U"));
    vi.stubGlobal("fetch", fetchMock);

    const { unmount, rerender } = renderHook(
      (props: { enabled: boolean }) =>
        useHlsSessionKeepalive({ enabled: props.enabled, streamUrl }),
      { initialProps: { enabled: false } },
    );

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);
    expect(fetchMock).not.toHaveBeenCalled();

    rerender({ enabled: true });
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(fetchMock).toHaveBeenCalledOnce();

    rerender({ enabled: false });
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);
    expect(fetchMock).toHaveBeenCalledOnce();

    rerender({ enabled: true });
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(fetchMock).toHaveBeenCalledTimes(2);

    unmount();
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("releases the response body", async () => {
    const response = new Response("#EXTM3U");
    const cancel = vi.spyOn(response.body!, "cancel");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);

    expect(cancel).toHaveBeenCalledOnce();
  });

  // A ping still in flight at unmount used to land after the page's stop
  // request and recreate the session it had just stopped.
  it("aborts a ping still in flight when it is disabled", async () => {
    let signal: AbortSignal | undefined;
    const fetchMock = vi.fn((_url: string, init: RequestInit) => {
      signal = init.signal ?? undefined;
      return new Promise<Response>(() => {});
    });
    vi.stubGlobal("fetch", fetchMock);

    const { unmount } = renderHook(() =>
      useHlsSessionKeepalive({ enabled: true, streamUrl }),
    );
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(signal?.aborted).toBe(false);

    unmount();

    expect(signal?.aborted).toBe(true);
  });

  it("does not stack a ping behind one the server is still holding", async () => {
    const fetchMock = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 3);

    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("survives fetch rejections", async () => {
    const fetchMock = vi.fn().mockRejectedValue(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
