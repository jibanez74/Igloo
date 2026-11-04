import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useWatchProgressSaver } from "@/hooks/useWatchProgressSaver";
import { episodeMediaRef, movieMediaRef } from "@/lib/media-ref";

const updateMediaWatchProgress = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  updateMediaWatchProgress,
}));

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function requestBody(fetchMock: ReturnType<typeof vi.fn>) {
  const options = fetchMock.mock.calls[0]?.[1] as RequestInit;
  return JSON.parse(options.body as string) as Record<string, unknown>;
}

const successfulUpdate = {
  error: false as const,
  data: { watched: false },
};

describe("watch progress saver", () => {
  it("saves nothing while disabled, not even on exit or tab hide", async () => {
    vi.useFakeTimers();
    try {
      const fetchMock = vi.fn().mockResolvedValue(
        new Response(JSON.stringify(successfulUpdate), { status: 200 }),
      );
      vi.stubGlobal("fetch", fetchMock);
      const currentTimeRef = { current: 300 };
      const durationRef = { current: 1000 };
      const { result } = renderHook(() =>
        useWatchProgressSaver({
          media: movieMediaRef(7),
          playing: true,
          currentTimeRef,
          durationRef,
          enabled: false,
        }),
      );

      await act(async () => {
        vi.advanceTimersByTime(60_000);
      });
      await act(async () => {
        await result.current.handlePauseSave();
        await result.current.handleEndedSave();
        await result.current.flushProgress();
      });
      vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
      await act(async () => {
        document.dispatchEvent(new Event("visibilitychange"));
        window.dispatchEvent(new Event("pagehide"));
      });

      expect(updateMediaWatchProgress).not.toHaveBeenCalled();
      expect(fetchMock).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("queues the exit snapshot after an in-flight save", async () => {
    const firstSave = deferred<typeof successfulUpdate>();
    updateMediaWatchProgress
      .mockReturnValueOnce(firstSave.promise)
      .mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());

    currentTimeRef.current = 450;
    let exitSave!: Promise<void>;
    act(() => {
      exitSave = result.current.flushProgress();
    });
    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();

    firstSave.resolve(successfulUpdate);
    await pauseSave;
    await waitFor(() =>
      expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2),
    );
    await exitSave;

    const saveSessionId = updateMediaWatchProgress.mock.calls[0]?.[3];
    expect(saveSessionId).toEqual(expect.any(String));
    expect(updateMediaWatchProgress).toHaveBeenNthCalledWith(
      1,
      { kind: "movie", id: 7 },
      300,
      1000,
      saveSessionId,
      1,
    );
    expect(updateMediaWatchProgress).toHaveBeenNthCalledWith(
      2,
      { kind: "movie", id: 7 },
      450,
      1000,
      saveSessionId,
      2,
    );
  });

  it("addresses an episode's own watch-progress route", async () => {
    updateMediaWatchProgress.mockResolvedValueOnce(successfulUpdate);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: episodeMediaRef(70103),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });
    expect(updateMediaWatchProgress).toHaveBeenCalledWith(
      episodeMediaRef(70103),
      300,
      1000,
      expect.any(String),
      1,
    );

    currentTimeRef.current = 320;
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/shows/episodes/70103/watch-progress",
        expect.objectContaining({ method: "PUT", keepalive: true }),
      ),
    );
    expect(requestBody(fetchMock).save_sequence).toBe(2);
  });

  it("uses a keepalive request on pagehide without saving again on unmount", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { unmount } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/movies/7/watch-progress",
        expect.objectContaining({
          method: "PUT",
          keepalive: true,
        }),
      ),
    );
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 300,
      duration_sec: 1000,
      save_session_id: expect.any(String),
      save_sequence: 1,
    });

    unmount();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(updateMediaWatchProgress).not.toHaveBeenCalled();
  });

  it("saves with a keepalive request when the tab is hidden", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    const visibilitySpy = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("hidden");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    visibilitySpy.mockRestore();

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/movies/7/watch-progress",
        expect.objectContaining({
          method: "PUT",
          keepalive: true,
        }),
      ),
    );
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 300,
      duration_sec: 1000,
      save_session_id: expect.any(String),
      save_sequence: 1,
    });
  });

  it("dedupes hidden-then-pagehide into a single keepalive save", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    const visibilitySpy = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("hidden");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("pagehide"));
    });
    visibilitySpy.mockRestore();

    await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
  });

  it("falls back to the tech-details duration when the video has none yet", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 0 };
    renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
        fallbackDurationSec: 5400,
      }),
    );

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/movies/7/watch-progress",
        expect.objectContaining({
          method: "PUT",
          keepalive: true,
        }),
      ),
    );
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 300,
      duration_sec: 5400,
      save_session_id: expect.any(String),
      save_sequence: 1,
    });
  });

  it("dispatches a captured lifecycle snapshot while an ordinary save is unresolved", async () => {
    const firstSave = deferred<typeof successfulUpdate>();
    updateMediaWatchProgress.mockReturnValueOnce(firstSave.promise);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());

    currentTimeRef.current = 450;
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    currentTimeRef.current = 600;
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/movies/7/watch-progress",
      expect.objectContaining({
        keepalive: true,
      }),
    );
    const saveSessionId = updateMediaWatchProgress.mock.calls[0]?.[3];
    expect(updateMediaWatchProgress).toHaveBeenCalledWith(
      { kind: "movie", id: 7 },
      300,
      1000,
      saveSessionId,
      1,
    );
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 450,
      duration_sec: 1000,
      save_session_id: saveSessionId,
      save_sequence: 2,
    });

    firstSave.resolve(successfulUpdate);
    await pauseSave;
  });

  it("waits for a pending save of the same position before the exit resolves", async () => {
    const pauseRequest = deferred<typeof successfulUpdate>();
    updateMediaWatchProgress.mockReturnValueOnce(pauseRequest.promise);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());

    let exitResolved = false;
    let exitSave!: Promise<void>;
    act(() => {
      exitSave = result.current.flushProgress().then(() => {
        exitResolved = true;
      });
    });
    await act(async () => {});
    expect(exitResolved).toBe(false);

    pauseRequest.resolve(successfulUpdate);
    await pauseSave;
    await exitSave;

    expect(exitResolved).toBe(true);
    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();
  });

  it("retries on exit when the pending save of the same position fails", async () => {
    const pauseRequest = deferred<{ error: true; message: string }>();
    updateMediaWatchProgress
      .mockReturnValueOnce(pauseRequest.promise)
      .mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());

    let exitSave!: Promise<void>;
    act(() => {
      exitSave = result.current.flushProgress();
    });
    pauseRequest.resolve({ error: true, message: "offline" });
    await pauseSave;
    await exitSave;

    expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2);
    expect(updateMediaWatchProgress).toHaveBeenLastCalledWith(
      { kind: "movie", id: 7 },
      300,
      1000,
      expect.any(String),
      2,
    );
  });

  it("sends one keepalive for a position whose ordinary save is unfinished", async () => {
    const pauseRequest = deferred<typeof successfulUpdate>();
    updateMediaWatchProgress.mockReturnValueOnce(pauseRequest.promise);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());

    const visibilitySpy = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("hidden");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("pagehide"));
    });
    visibilitySpy.mockRestore();

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 300,
      duration_sec: 1000,
      save_session_id: expect.any(String),
      save_sequence: 2,
    });

    pauseRequest.resolve(successfulUpdate);
    await pauseSave;
  });

  it("keeps a keepalive's report when an older save of that position fails", async () => {
    const pauseRequest = deferred<{ error: true; message: string }>();
    updateMediaWatchProgress.mockReturnValueOnce(pauseRequest.promise);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMediaWatchProgress).toHaveBeenCalledOnce());
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(fetchMock).toHaveBeenCalledOnce();

    pauseRequest.resolve({ error: true, message: "offline" });
    await pauseSave;
    await act(async () => {
      await result.current.flushProgress();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();
  });

  it("does not count a save skipped for an unknown duration as reported", async () => {
    updateMediaWatchProgress.mockResolvedValue(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 0 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });
    expect(updateMediaWatchProgress).not.toHaveBeenCalled();

    durationRef.current = 1000;
    await act(async () => {
      await result.current.flushProgress();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();
    expect(updateMediaWatchProgress).toHaveBeenCalledWith(
      { kind: "movie", id: 7 },
      300,
      1000,
      expect.any(String),
      1,
    );
  });

  it("does not count a keepalive skipped for an unknown duration as reported", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 0 };
    renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      window.dispatchEvent(new Event("pagehide"));
    });
    expect(fetchMock).not.toHaveBeenCalled();

    durationRef.current = 1000;
    await act(async () => {
      window.dispatchEvent(new Event("pagehide"));
    });

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(requestBody(fetchMock)).toEqual({
      progress_sec: 300,
      duration_sec: 1000,
      save_session_id: expect.any(String),
      save_sequence: 1,
    });
  });

  it("saves progress above the 30-second floor", async () => {
    updateMediaWatchProgress.mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 45 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledWith(
      { kind: "movie", id: 7 },
      45,
      1000,
      expect.any(String),
      1,
    );
  });

  it("reports nothing from a page that never played", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: false,
        currentTimeRef,
        durationRef,
      }),
    );

    const visibilitySpy = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("hidden");
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("pagehide"));
      await result.current.handlePauseSave();
      await result.current.flushProgress();
    });
    visibilitySpy.mockRestore();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(updateMediaWatchProgress).not.toHaveBeenCalled();
  });

  it("does not report a position again on close or exit", async () => {
    // A paused tab left open while the viewer went on watching elsewhere: its
    // stale position must not overwrite the newer progress.
    updateMediaWatchProgress.mockResolvedValue(successfulUpdate);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });
    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();

    currentTimeRef.current = 300.4;
    const visibilitySpy = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("hidden");
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("pagehide"));
      await result.current.flushProgress();
      await result.current.handlePauseSave();
    });
    visibilitySpy.mockRestore();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(updateMediaWatchProgress).toHaveBeenCalledOnce();
  });

  it("reports a position moved since the last save, even while paused", async () => {
    updateMediaWatchProgress.mockResolvedValue(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: false,
        currentTimeRef,
        durationRef,
      }),
    );

    // The end is reported even without a play seen here, which gives the
    // page a last reported position to measure the next one from.
    await act(async () => {
      await result.current.handleEndedSave();
    });
    currentTimeRef.current = 450;
    await act(async () => {
      await result.current.flushProgress();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2);
    expect(updateMediaWatchProgress).toHaveBeenLastCalledWith(
      { kind: "movie", id: 7 },
      450,
      1000,
      expect.any(String),
      2,
    );
  });

  it("retries a position whose save failed", async () => {
    updateMediaWatchProgress
      .mockResolvedValueOnce({ error: true, message: "offline" })
      .mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });
    await act(async () => {
      await result.current.flushProgress();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2);
    expect(updateMediaWatchProgress).toHaveBeenLastCalledWith(
      { kind: "movie", id: 7 },
      300,
      1000,
      expect.any(String),
      2,
    );
  });

  it("retries a position whose keepalive save failed", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("{}", { status: 500 }))
      .mockResolvedValueOnce(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
    // Let the failed response settle before the next lifecycle event.
    await act(async () => {});
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it("always saves the end, even right after a pause save", async () => {
    updateMediaWatchProgress.mockResolvedValue({
      error: false as const,
      data: { watched: true },
    });
    const currentTimeRef = { current: 999.8 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useWatchProgressSaver({
        media: movieMediaRef(7),
        playing: true,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
      await result.current.handleEndedSave();
    });

    expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2);
    expect(updateMediaWatchProgress).toHaveBeenLastCalledWith(
      { kind: "movie", id: 7 },
      1000,
      1000,
      expect.any(String),
      2,
    );
  });

  it("skips interval saves while the position has not moved", async () => {
    vi.useFakeTimers();
    try {
      updateMediaWatchProgress.mockResolvedValue(successfulUpdate);
      const currentTimeRef = { current: 300 };
      const durationRef = { current: 1000 };
      renderHook(() =>
        useWatchProgressSaver({
          media: movieMediaRef(7),
          playing: true,
          currentTimeRef,
          durationRef,
        }),
      );

      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(updateMediaWatchProgress).toHaveBeenCalledOnce();

      // Buffering: still playing, no time has passed in the media.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });
      expect(updateMediaWatchProgress).toHaveBeenCalledOnce();

      currentTimeRef.current = 315;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(updateMediaWatchProgress).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });
});
