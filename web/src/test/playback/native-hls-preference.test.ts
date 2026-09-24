import { describe, it, expect, vi } from "vitest";
import { shouldPreferNativeHls } from "@/lib/playback";

const reportsNativeHls = (type: string) =>
  type.toLowerCase().includes("mpegurl") ? "maybe" : "";
const noNativeHls = () => "";

const mediaSource = (supported: boolean) => ({
  isTypeSupported: vi.fn(() => supported),
});

describe("shouldPreferNativeHls", () => {
  it("keeps hls.js when native HLS is reported but MSE can play the stream (Chrome)", () => {
    const mse = mediaSource(true);
    expect(
      shouldPreferNativeHls({ canPlayType: reportsNativeHls, mediaSource: mse }),
    ).toBe(false);
    expect(mse.isTypeSupported).toHaveBeenCalledWith(
      'video/mp4; codecs="avc1.42E01E,mp4a.40.2"',
    );
  });

  it("uses native HLS when there is no MediaSource (iPhone Safari)", () => {
    expect(
      shouldPreferNativeHls({
        canPlayType: reportsNativeHls,
        mediaSource: undefined,
      }),
    ).toBe(true);
  });

  it("uses native HLS when MSE cannot play the fMP4 H.264/AAC output", () => {
    expect(
      shouldPreferNativeHls({
        canPlayType: reportsNativeHls,
        mediaSource: mediaSource(false),
      }),
    ).toBe(true);
  });

  it("never prefers native HLS the browser does not report", () => {
    expect(
      shouldPreferNativeHls({ canPlayType: noNativeHls, mediaSource: undefined }),
    ).toBe(false);
    expect(
      shouldPreferNativeHls({
        canPlayType: noNativeHls,
        mediaSource: mediaSource(true),
      }),
    ).toBe(false);
  });

  it("accepts the legacy x-mpegURL answer", () => {
    expect(
      shouldPreferNativeHls({
        canPlayType: (type) => (type === "application/x-mpegURL" ? "maybe" : ""),
        mediaSource: undefined,
      }),
    ).toBe(true);
  });
});
