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

  it("uses a keepalive request on pagehide without saving again on unmount", async () => {
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

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/movies/7/watch-progress",
        expect.objectContaining({
          method: "PUT",
          keepalive: true,
          body: JSON.stringify({ progress_sec: 300, duration_sec: 1000 }),
        }),
      ),
    );

    unmount();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(updateMovieWatchProgress).not.toHaveBeenCalled();
  });

  it("saves with a keepalive request when the tab is hidden", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    renderHook(() =>
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
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
          body: JSON.stringify({ progress_sec: 300, duration_sec: 1000 }),
        }),
      ),
    );
  });

  it("dedupes hidden-then-pagehide into a single keepalive save", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
    const currentTimeRef = { current: 300 };
    const durationRef = { current: 1000 };
    renderHook(() =>
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
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
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
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
          body: JSON.stringify({ progress_sec: 300, duration_sec: 5400 }),
        }),
      ),
    );
  });

  it("dispatches a captured lifecycle snapshot while an ordinary save is unresolved", async () => {
    const firstSave = deferred<typeof successfulUpdate>();
    updateMovieWatchProgress.mockReturnValueOnce(firstSave.promise);
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", fetchMock);
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
    act(() => {
      window.dispatchEvent(new Event("pagehide"));
    });
    currentTimeRef.current = 600;
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/movies/7/watch-progress",
      expect.objectContaining({
        keepalive: true,
        body: JSON.stringify({ progress_sec: 450, duration_sec: 1000 }),
      }),
    );

    firstSave.resolve(successfulUpdate);
    await pauseSave;
  });

  it("saves progress above the 30-second floor", async () => {
    updateMovieWatchProgress.mockResolvedValueOnce(successfulUpdate);
    const currentTimeRef = { current: 45 };
    const durationRef = { current: 1000 };
    const { result } = renderHook(() =>
      useMovieWatchProgressSaver({
        movieId: 7,
        playing: false,
        currentTimeRef,
        durationRef,
      }),
    );

    await act(async () => {
      await result.current.handlePauseSave();
    });

    expect(updateMovieWatchProgress).toHaveBeenCalledWith(7, 45, 1000);
  });
});
