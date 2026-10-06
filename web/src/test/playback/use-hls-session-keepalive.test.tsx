import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useHlsSessionKeepalive } from "@/hooks/useHlsSessionKeepalive";
import { HLS_SESSION_KEEPALIVE_INTERVAL_MS } from "@/lib/constants";

const streamUrl =
  "/api/movies/7/hls/remux/playlist.m3u8?playback_session=uuid&start=0";

const deferred404Response = () => {
  const response = new Response("gone", { status: 404 });
  let releaseCancellation!: () => void;
  const cancellation = new Promise<void>((resolve) => {
    releaseCancellation = resolve;
  });
  const cancel = vi.spyOn(response.body!, "cancel").mockReturnValue(cancellation);
  return { response, cancel, releaseCancellation };
};

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

    const { rerender } = renderHook(
      (props: { enabled: boolean }) =>
        useHlsSessionKeepalive({ enabled: props.enabled, streamUrl }),
      { initialProps: { enabled: true } },
    );
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(signal?.aborted).toBe(false);

    rerender({ enabled: false });

    expect(signal?.aborted).toBe(true);
  });

  it("does not stack a ping behind one the server is still holding", async () => {
    const fetchMock = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 3);

    expect(fetchMock).toHaveBeenCalledOnce();
  });

  // A personal stream must not have an evicted session recreated at the
  // window start: after a long sleep the playhead can be far past it.
  it("makes the ping refresh-only and reports a lost session once", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () => new Response("gone", { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);
    const onSessionLost = vi.fn();

    renderHook(() =>
      useHlsSessionKeepalive({ enabled: true, streamUrl, onSessionLost }),
    );
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);

    expect(fetchMock).toHaveBeenCalledWith(
      `${streamUrl}&keepalive=1`,
      expect.objectContaining({ credentials: "include" }),
    );
    expect(onSessionLost).toHaveBeenCalledOnce();

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(onSessionLost).toHaveBeenCalledOnce();
  });

  it("reports nothing while the refreshed session is alive", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("#EXTM3U")));
    const onSessionLost = vi.fn();

    renderHook(() =>
      useHlsSessionKeepalive({ enabled: true, streamUrl, onSessionLost }),
    );
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);

    expect(onSessionLost).not.toHaveBeenCalled();
  });

  it("ignores an obsolete 404 after replacing the stream URL and handler", async () => {
    const { response, cancel, releaseCancellation } = deferred404Response();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(response)
      .mockImplementation(async () => new Response("gone", { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);
    const onSessionLost = vi.fn();
    const replacementOnSessionLost = vi.fn();
    const replacementStreamUrl = streamUrl.replace("start=0", "start=60");
    const { rerender } = renderHook(
      (props: { streamUrl: string; onSessionLost: () => void }) =>
        useHlsSessionKeepalive({ enabled: true, ...props }),
      { initialProps: { streamUrl, onSessionLost } },
    );

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
    expect(cancel).toHaveBeenCalledOnce();
    expect(onSessionLost).not.toHaveBeenCalled();

    rerender({
      streamUrl: replacementStreamUrl,
      onSessionLost: replacementOnSessionLost,
    });
    releaseCancellation();
    await vi.advanceTimersByTimeAsync(0);

    expect(onSessionLost).not.toHaveBeenCalled();
    expect(replacementOnSessionLost).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock).toHaveBeenLastCalledWith(
      `${replacementStreamUrl}&keepalive=1`,
      expect.anything(),
    );
    expect(onSessionLost).not.toHaveBeenCalled();
    expect(replacementOnSessionLost).toHaveBeenCalledOnce();
  });

  it.each(["disabled", "unmounted"])(
    "ignores a 404 when %s while body cancellation is pending",
    async (cleanup) => {
      const { response, cancel, releaseCancellation } = deferred404Response();
      const fetchMock = vi.fn().mockResolvedValue(response);
      vi.stubGlobal("fetch", fetchMock);
      const onSessionLost = vi.fn();
      const { rerender, unmount } = renderHook(
        (props: { enabled: boolean }) =>
          useHlsSessionKeepalive({ ...props, streamUrl, onSessionLost }),
        { initialProps: { enabled: true } },
      );

      await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);
      expect(cancel).toHaveBeenCalledOnce();
      expect(onSessionLost).not.toHaveBeenCalled();

      if (cleanup === "disabled") {
        rerender({ enabled: false });
      } else {
        unmount();
      }
      releaseCancellation();
      await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);

      expect(fetchMock).toHaveBeenCalledOnce();
      expect(onSessionLost).not.toHaveBeenCalled();
    },
  );

  // A watch room's stream always starts at 0 and the room sync restores the
  // position, so its ping may recreate the session.
  it("keeps the plain manifest ping without a session-lost handler", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () => new Response("gone", { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));
    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS);

    expect(fetchMock).toHaveBeenCalledWith(streamUrl, expect.anything());
  });

  it("survives fetch rejections", async () => {
    const fetchMock = vi.fn().mockRejectedValue(new Error("offline"));
    vi.stubGlobal("fetch", fetchMock);

    renderHook(() => useHlsSessionKeepalive({ enabled: true, streamUrl }));

    await vi.advanceTimersByTimeAsync(HLS_SESSION_KEEPALIVE_INTERVAL_MS * 2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
