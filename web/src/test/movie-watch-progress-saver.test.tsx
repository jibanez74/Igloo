import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useMovieWatchProgressSaver } from "@/hooks/useMovieWatchProgressSaver";

const updateMovieWatchProgress = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  updateMovieWatchProgress,
}));

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

const successfulUpdate = {
  error: false as const,
  data: { watched: false },
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("movie watch progress saver", () => {
  it("queues the exit snapshot after an in-flight save", async () => {
    const firstSave = deferred<typeof successfulUpdate>();
    updateMovieWatchProgress
      .mockReturnValueOnce(firstSave.promise)
      .mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
        currentTimeRef,
        durationRef,
      }),
    );

    let pauseSave!: Promise<void>;
    act(() => {
      pauseSave = result.current.handlePauseSave();
    });
    await waitFor(() => expect(updateMovieWatchProgress).toHaveBeenCalledOnce());

    currentTimeRef.current = 450;
    let exitSave!: Promise<void>;
    act(() => {
      exitSave = result.current.flushProgress();
    });
    expect(updateMovieWatchProgress).toHaveBeenCalledOnce();

    firstSave.resolve(successfulUpdate);
    await pauseSave;
    await waitFor(() =>
      expect(updateMovieWatchProgress).toHaveBeenCalledTimes(2),
    );
    await exitSave;

    expect(updateMovieWatchProgress).toHaveBeenNthCalledWith(1, 7, 300, 1000);
    expect(updateMovieWatchProgress).toHaveBeenNthCalledWith(2, 7, 450, 1000);
  });

  it("uses a keepalive request on pagehide without saving again on unmount", () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    const { unmount } = renderHook(() =>
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
        currentTimeRef,
        durationRef,
      }),
    );

    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/movies/7/watch-progress",
      expect.objectContaining({
        method: "PUT",
        keepalive: true,
        body: JSON.stringify({ progress_sec: 300, duration_sec: 1000 }),
      }),
    );

    unmount();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(updateMovieWatchProgress).not.toHaveBeenCalled();
  });
});
