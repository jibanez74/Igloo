import { describe, it, expect, vi, afterEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { fireEvent } from "@testing-library/dom";
import { shouldDirectPlayFallback } from "@/lib/movie-playback";
import { useDirectPlayFallback } from "@/hooks/useDirectPlayFallback";
import {
  DIRECT_PLAY_STALL_TIMEOUT_MS,
  MEDIA_ERR_DECODE,
  MEDIA_ERR_SRC_NOT_SUPPORTED,
} from "@/lib/constants";
import type { StreamModeId } from "@/types";

const eligibleArgs = {
  trigger: MEDIA_ERR_SRC_NOT_SUPPORTED as number | "stall",
  isHlsPlayback: false,
  resolvedMode: "direct" as StreamModeId,
  techLoaded: true,
  directAvailable: true,
  alreadyAttempted: false,
};

describe("shouldDirectPlayFallback", () => {
  it.each<[string, number | "stall"]>([
    ["MEDIA_ERR_DECODE", MEDIA_ERR_DECODE],
    ["MEDIA_ERR_SRC_NOT_SUPPORTED", MEDIA_ERR_SRC_NOT_SUPPORTED],
    ["the stall guard", "stall"],
  ])("falls back on %s when direct play was affirmatively chosen", (_n, trigger) => {
    expect(shouldDirectPlayFallback({ ...eligibleArgs, trigger })).toBe(true);
  });

  it.each<[string, Partial<typeof eligibleArgs>]>([
    ["MEDIA_ERR_ABORTED", { trigger: 1 }],
    ["MEDIA_ERR_NETWORK", { trigger: 2 }],
    ["a missing error code", { trigger: undefined as unknown as number }],
    ["HLS playback", { isHlsPlayback: true }],
    ["a non-direct resolved mode", { resolvedMode: "remux" as StreamModeId }],
    // D16 guard: an error from a pre-eligibility request must not fall back.
    ["pending technical details", { techLoaded: false }],
    ["a file the app decided is not direct-playable", { directAvailable: false }],
    ["a window that already fell back", { alreadyAttempted: true }],
  ])("never falls back on %s", (_n, overrides) => {
    expect(shouldDirectPlayFallback({ ...eligibleArgs, ...overrides })).toBe(
      false,
    );
  });
});

type HookProps = Parameters<typeof useDirectPlayFallback>[0];

function hookProps(video: HTMLVideoElement, onFallback: () => void): HookProps {
  return {
    streamWindowKey: "1:direct:0:session:0",
    videoRef: { current: video },
    isHlsPlayback: false,
    resolvedMode: "direct",
    techLoaded: true,
    directAvailable: true,
    playerMounted: true,
    onFallback,
  };
}

describe("useDirectPlayFallback", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("falls back exactly once when no metadata arrives within the timeout", async () => {
    vi.useFakeTimers();
    const video = document.createElement("video");
    const onFallback = vi.fn();
    renderHook(() => useDirectPlayFallback(hookProps(video, onFallback)));

    await act(async () => {
      await vi.advanceTimersByTimeAsync(DIRECT_PLAY_STALL_TIMEOUT_MS - 1);
    });
    expect(onFallback).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(onFallback).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(DIRECT_PLAY_STALL_TIMEOUT_MS * 2);
    });
    expect(onFallback).toHaveBeenCalledTimes(1);
  });

  it("does not fall back once metadata loads", async () => {
    vi.useFakeTimers();
    const video = document.createElement("video");
    const onFallback = vi.fn();
    renderHook(() => useDirectPlayFallback(hookProps(video, onFallback)));

    fireEvent(video, new Event("loadedmetadata"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(DIRECT_PLAY_STALL_TIMEOUT_MS * 2);
    });
    expect(onFallback).not.toHaveBeenCalled();
  });

  it("stays disarmed for HLS playback", async () => {
    vi.useFakeTimers();
    const video = document.createElement("video");
    const onFallback = vi.fn();
    renderHook(() =>
      useDirectPlayFallback({
        ...hookProps(video, onFallback),
        isHlsPlayback: true,
        resolvedMode: "remux",
      }),
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(DIRECT_PLAY_STALL_TIMEOUT_MS * 2);
    });
    expect(onFallback).not.toHaveBeenCalled();
  });

  it("consumes decode errors and reports unrelated codes as unconsumed", () => {
    const video = document.createElement("video");
    const onFallback = vi.fn();
    const { result } = renderHook(() =>
      useDirectPlayFallback(hookProps(video, onFallback)),
    );

    expect(result.current.handleNativeError(2)).toBe(false);
    expect(onFallback).not.toHaveBeenCalled();

    expect(result.current.handleNativeError(MEDIA_ERR_DECODE)).toBe(true);
    expect(onFallback).toHaveBeenCalledTimes(1);

    // One-shot per stream window: a second error is no longer consumed.
    expect(result.current.handleNativeError(MEDIA_ERR_DECODE)).toBe(false);
    expect(onFallback).toHaveBeenCalledTimes(1);
  });

  it("re-arms when the stream window changes", () => {
    const video = document.createElement("video");
    const onFallback = vi.fn();
    const { result, rerender } = renderHook(
      (props: HookProps) => useDirectPlayFallback(props),
      { initialProps: hookProps(video, onFallback) },
    );

    expect(result.current.handleNativeError(MEDIA_ERR_DECODE)).toBe(true);

    rerender({
      ...hookProps(video, onFallback),
      streamWindowKey: "1:direct:0:session:120",
    });
    expect(result.current.handleNativeError(MEDIA_ERR_DECODE)).toBe(true);
    expect(onFallback).toHaveBeenCalledTimes(2);
  });
});
