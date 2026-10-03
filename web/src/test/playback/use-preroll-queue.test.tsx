import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { usePrerollQueue } from "@/hooks/usePrerollQueue";
import type { PrerollTrailer } from "@/types";

type HookProps = Parameters<typeof usePrerollQueue>[0];

function trailer(key: string, title = key): PrerollTrailer {
  return {
    title,
    youtube_key: key,
    source: "library",
    movie_id: 1,
    tmdb_id: null,
  };
}

const queue = [trailer("one", "One"), trailer("two", "Two")];

function renderQueue(initialProps: HookProps) {
  return renderHook(props => usePrerollQueue(props), { initialProps });
}

describe("usePrerollQueue", () => {
  it("is loading until the queue resolves, then plays the first trailer", () => {
    const { result, rerender } = renderQueue({
      trailers: undefined,
      loadFailed: false,
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.done).toBe(false);
    expect(result.current.current).toBeNull();

    rerender({ trailers: queue, loadFailed: false });

    expect(result.current.loading).toBe(false);
    expect(result.current.current?.title).toBe("One");
    expect(result.current.position).toBe(1);
    expect(result.current.total).toBe(2);
    expect(result.current.isLast).toBe(false);
  });

  it("skips to the next trailer and finishes after the last one", () => {
    const { result } = renderQueue({ trailers: queue, loadFailed: false });

    act(() => result.current.skip());
    expect(result.current.current?.title).toBe("Two");
    expect(result.current.position).toBe(2);
    expect(result.current.isLast).toBe(true);
    expect(result.current.done).toBe(false);

    act(() => result.current.skip());
    expect(result.current.done).toBe(true);
    expect(result.current.current).toBeNull();
  });

  it("starts the movie on skip all", () => {
    const { result } = renderQueue({ trailers: queue, loadFailed: false });

    act(() => result.current.skipAll());

    expect(result.current.done).toBe(true);
    expect(result.current.current).toBeNull();
  });

  it("treats an embed error like a finished trailer", () => {
    const { result } = renderQueue({ trailers: queue, loadFailed: false });

    act(() => result.current.handleTrailerError());
    expect(result.current.current?.title).toBe("Two");

    act(() => result.current.handleTrailerEnded());
    expect(result.current.done).toBe(true);
  });

  it("finishes immediately on an empty queue", () => {
    const { result } = renderQueue({ trailers: [], loadFailed: false });

    expect(result.current.loading).toBe(false);
    expect(result.current.done).toBe(true);
  });

  it("finishes immediately when the queue request failed", () => {
    const { result } = renderQueue({ trailers: undefined, loadFailed: true });

    expect(result.current.loading).toBe(false);
    expect(result.current.done).toBe(true);
  });
});
