import { describe, expect, it } from "vitest";
import { isBitmapSubtitleCodec } from "@/lib/playback";

describe("isBitmapSubtitleCodec", () => {
  // The server's WebVTT route refuses every one of these with a 415.
  it.each(["hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", "xsub"])(
    "treats %s as a bitmap codec",
    (codec) => {
      expect(isBitmapSubtitleCodec(codec)).toBe(true);
    },
  );

  it("ignores the codec name's case", () => {
    expect(isBitmapSubtitleCodec("XSUB")).toBe(true);
    expect(isBitmapSubtitleCodec("HDMV_PGS_SUBTITLE")).toBe(true);
  });

  it.each(["subrip", "ass", "mov_text", "webvtt", ""])(
    "treats %j as a text codec",
    (codec) => {
      expect(isBitmapSubtitleCodec(codec)).toBe(false);
    },
  );
});
