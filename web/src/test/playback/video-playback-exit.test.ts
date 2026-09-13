import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import {
  CONTINUE_WATCHING_KEY,
  EPISODE_WATCH_PROGRESS_KEY,
  MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS,
  MOVIE_WATCH_PROGRESS_KEY,
  SHOW_SEASON_EPISODES_KEY,
} from "@/lib/constants";
import {
  refreshWatchQueries,
  staysOnCurrentPlayback,
  synchronizePlaybackExit,
} from "@/lib/video-playback-exit";
import { episodeMediaRef, movieMediaRef } from "@/lib/media-ref";

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

afterEach(() => {
  vi.useRealTimers();
});

describe("movie playback exit synchronization", () => {
  it("saves progress before refreshing the watch queries", async () => {
    const save = deferred<void>();
    const refresh = vi.fn().mockResolvedValue(undefined);
    const pause = vi.fn();

    const synchronization = synchronizePlaybackExit({
      pausePlayback: pause,
      flushProgress: () => save.promise,
      refreshWatchQueries: refresh,
      onSaveError: vi.fn(),
    });

    expect(pause).toHaveBeenCalledOnce();
    expect(refresh).not.toHaveBeenCalled();

    save.resolve();
    await synchronization;

    expect(refresh).toHaveBeenCalledOnce();
  });

  it("refreshes and completes when saving or refreshing fails", async () => {
    const refresh = vi.fn().mockRejectedValue(new Error("refresh failed"));
    const onSaveError = vi.fn();

    await expect(
      synchronizePlaybackExit({
        pausePlayback: vi.fn(),
        flushProgress: () => Promise.reject(new Error("save failed")),
        refreshWatchQueries: refresh,
        onSaveError,
      }),
    ).resolves.toBeUndefined();

    expect(onSaveError).toHaveBeenCalledOnce();
    expect(refresh).toHaveBeenCalledOnce();
  });

  it("releases navigation when saving does not settle within the budget", async () => {
    vi.useFakeTimers();
    const save = deferred<void>();
    const refresh = vi.fn().mockResolvedValue(undefined);
    const onSettled = vi.fn();

    const synchronization = synchronizePlaybackExit({
      pausePlayback: vi.fn(),
      flushProgress: () => save.promise,
      refreshWatchQueries: refresh,
      onSaveError: vi.fn(),
    });
    void synchronization.then(onSettled);

    await vi.advanceTimersByTimeAsync(
      MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS - 1,
    );
    expect(onSettled).not.toHaveBeenCalled();
    expect(refresh).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    await synchronization;
    expect(onSettled).toHaveBeenCalledOnce();
    expect(refresh).not.toHaveBeenCalled();

    save.resolve();
    await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledOnce();
  });

  it("uses one total budget across saving and refreshing", async () => {
    vi.useFakeTimers();
    const save = deferred<void>();
    const refresh = deferred<void>();
    const onSettled = vi.fn();
    const refreshWatchQueries = vi.fn(() => refresh.promise);

    const synchronization = synchronizePlaybackExit({
      pausePlayback: vi.fn(),
      flushProgress: () => save.promise,
      refreshWatchQueries,
      onSaveError: vi.fn(),
    });
    void synchronization.then(onSettled);

    await vi.advanceTimersByTimeAsync(1_500);
    save.resolve();
    await vi.advanceTimersByTimeAsync(0);
    expect(refreshWatchQueries).toHaveBeenCalledOnce();

    await vi.advanceTimersByTimeAsync(
      MOVIE_PLAYBACK_EXIT_SYNC_TIMEOUT_MS - 1_501,
    );
    expect(onSettled).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    await synchronization;
    expect(onSettled).toHaveBeenCalledOnce();

    refresh.resolve();
  });

  it("invalidates continue watching and the movie's watch progress", async () => {
    const queryClient = new QueryClient();
    const movieId = 7;
    await queryClient.prefetchQuery({
      queryKey: [CONTINUE_WATCHING_KEY],
      queryFn: () => Promise.resolve("continue-watching"),
    });
    await queryClient.prefetchQuery({
      queryKey: [MOVIE_WATCH_PROGRESS_KEY, movieId],
      queryFn: () => Promise.resolve("watch-progress"),
    });

    await refreshWatchQueries(queryClient, movieMediaRef(movieId));

    // refetchType "all" refetches the inactive continue-watching query.
    expect(
      queryClient.getQueryState([CONTINUE_WATCHING_KEY])?.dataUpdateCount,
    ).toBe(2);
    expect(
      queryClient.getQueryState([MOVIE_WATCH_PROGRESS_KEY, movieId])
        ?.isInvalidated,
    ).toBe(true);

    queryClient.clear();
  });

  it("invalidates the season list and the episode's watch progress", async () => {
    const queryClient = new QueryClient();
    const episodeId = 9;
    await queryClient.prefetchQuery({
      queryKey: [SHOW_SEASON_EPISODES_KEY, 401, 1],
      queryFn: () => Promise.resolve("episodes"),
    });
    await queryClient.prefetchQuery({
      queryKey: [EPISODE_WATCH_PROGRESS_KEY, episodeId],
      queryFn: () => Promise.resolve("watch-progress"),
    });

    await refreshWatchQueries(queryClient, episodeMediaRef(episodeId));

    expect(
      queryClient.getQueryState([SHOW_SEASON_EPISODES_KEY, 401, 1])
        ?.dataUpdateCount,
    ).toBe(2);
    expect(
      queryClient.getQueryState([EPISODE_WATCH_PROGRESS_KEY, episodeId])
        ?.isInvalidated,
    ).toBe(true);

    queryClient.clear();
  });

  it("only bypasses synchronization within the same movie pathname", () => {
    const current = {
      routeId: "/_auth/movies/$id/play",
      pathname: "/movies/7/play",
    };

    expect(staysOnCurrentPlayback(current, current)).toBe(true);
    expect(
      staysOnCurrentPlayback(current, {
        ...current,
        pathname: "/movies/8/play",
      }),
    ).toBe(false);
    expect(
      staysOnCurrentPlayback(current, {
        routeId: "/_auth/",
        pathname: "/",
      }),
    ).toBe(false);
  });
});
