import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useHevcCopyFallback } from "@/hooks/useHevcCopyFallback";

type HookProps = Parameters<typeof useHevcCopyFallback>[0];

function renderFallback(overrides: Partial<HookProps> = {}) {
  const onFallback = vi.fn();
  const initialProps: HookProps = {
    streamWindowKey: "7:remux:0:session:0",
    hevcCopyDeclared: true,
    onFallback,
    ...overrides,
  };
  const hook = renderHook((props: HookProps) => useHevcCopyFallback(props), {
    initialProps,
  });
  return { ...hook, onFallback, initialProps };
}

describe("useHevcCopyFallback", () => {
  it("falls back once and swallows the same window's later reports", () => {
    const { result, onFallback } = renderFallback();

    expect(result.current.handleDecodeFailure()).toBe(true);
    expect(result.current.handleDecodeFailure()).toBe(true);
    expect(onFallback).toHaveBeenCalledTimes(1);
  });

  it("passes on where the viewer was before an hls.js recovery", () => {
    const { result, onFallback } = renderFallback();

    result.current.handleDecodeFailure({ timeSec: 600, playing: true });

    expect(onFallback).toHaveBeenCalledWith({ timeSec: 600, playing: true });
  });

  it("leaves a stream that declared no HEVC to the normal error", () => {
    const { result, onFallback } = renderFallback({ hevcCopyDeclared: false });

    expect(result.current.handleDecodeFailure()).toBe(false);
    expect(onFallback).not.toHaveBeenCalled();
  });

  it("re-arms when the stream window changes", () => {
    const { result, rerender, onFallback, initialProps } = renderFallback();

    result.current.handleDecodeFailure();
    rerender({ ...initialProps, streamWindowKey: "7:remux:0:session:600" });
    result.current.handleDecodeFailure();

    expect(onFallback).toHaveBeenCalledTimes(2);
  });
});
