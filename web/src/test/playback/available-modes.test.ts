import { describe, it, expect, vi } from "vitest";
import {
  directPlayAudioSelectionEligible,
  directPlayModeLabel,
  getAvailableModes,
} from "@/lib/playback";
import type { DirectPlayAudioInfo, DirectPlayVideoInfo } from "@/lib/playback";
import type { AudioStreamType } from "@/types/movies";

const nullString = { String: "", Valid: false };
const nullInt = { Int64: 0, Valid: false };

const h264Video = (
  overrides: Partial<DirectPlayVideoInfo> = {},
): DirectPlayVideoInfo => ({
  codec: "h264",
  codec_profile: { String: "High", Valid: true },
  codec_level: { Int64: 41, Valid: true },
  height: 1080,
  bit_depth: { Int64: 8, Valid: true },
  pixel_format: { String: "yuv420p", Valid: true },
  ...overrides,
});

const aacAudio = (
  overrides: Partial<DirectPlayAudioInfo> = {},
): DirectPlayAudioInfo => ({
  codec: "aac",
  codec_profile: { String: "LC", Valid: true },
  is_default: false,
  ...overrides,
});

const modeIds = (modes: ReturnType<typeof getAvailableModes>) =>
  modes.map((m) => m.id);

describe("getAvailableModes container gate", () => {
  it("offers direct play for an eligible MP4 source", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [aacAudio()],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).toContain("direct");
    expect(ids).toContain("remux");
  });

  // Audit matrix row 7: MKV must never be offered for direct play — Chrome and
  // Firefox stall silently at 0ms with no MediaError. video/webm and video/ogg
  // are unreachable dead values and must stay refused too.
  it.each([
    "video/x-matroska",
    "video/webm",
    "video/ogg",
    "application/octet-stream",
  ])("refuses direct play for %s while keeping remux", (mimeType) => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [aacAudio()],
        mimeType,
      }),
    );
    expect(ids).not.toContain("direct");
    expect(ids).toContain("remux");
  });
});

describe("direct-play audio ambiguity gate", () => {
  // Audit §6.2: refuse on ambiguity, not on absence.
  it.each([
    ["single stream", [false], true],
    ["single default-flagged stream", [true], true],
    // Row 16: no flags at all — browsers follow track order, stream 0 plays.
    ["multiple streams, no defaults", [false, false, false], true],
    ["multiple streams, single default on stream 0", [true, false], true],
    // Row 16b / 18: the container says a later stream is the one that plays.
    ["single default on a non-zero index", [false, true], false],
    // Row 16c: more than one default is browser-defined behaviour.
    ["multiple defaults", [true, true], false],
  ])("%s → eligible=%s", (_name, defaults, wantEligible) => {
    const streams = defaults.map((is_default) => ({ is_default }));
    expect(directPlayAudioSelectionEligible(streams)).toBe(wantEligible);
  });

  // Row 18: AAC commentary first, the flagged main track second — the codec
  // gate alone would pass (stream 0 is AAC), the ambiguity gate must refuse.
  it("refuses direct play when the default disposition is not on stream 0", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [
          aacAudio({ is_default: false }),
          aacAudio({ is_default: true }),
        ],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).not.toContain("direct");
    expect(ids).toContain("remux");
  });

  it("keeps direct play for multiple streams when stream 0 is the single default", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [
          aacAudio({ is_default: true }),
          aacAudio({ is_default: false }),
        ],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).toContain("direct");
  });

  it("keeps direct play for multiple streams with no default flags", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [aacAudio(), aacAudio(), aacAudio()],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).toContain("direct");
  });
});

describe("browser-safe H.264 gate", () => {
  // Audit matrix row 22: browsers cannot decode 10-bit / 4:2:2 / 4:4:4 H.264;
  // these must fall to the HLS path even though the codec name passes.
  it.each([
    ["High 10 profile", { codec_profile: { String: "High 10", Valid: true } }],
    [
      "High 4:2:2 profile",
      { codec_profile: { String: "High 4:2:2", Valid: true } },
    ],
    [
      "High 4:4:4 Predictive profile",
      { codec_profile: { String: "High 4:4:4 Predictive", Valid: true } },
    ],
    ["10-bit depth alone", { bit_depth: { Int64: 10, Valid: true } }],
    [
      "10-bit pixel format alone",
      { pixel_format: { String: "yuv420p10le", Valid: true } },
    ],
    [
      "4:2:2 pixel format alone",
      { pixel_format: { String: "yuv422p", Valid: true } },
    ],
    [
      "4:4:4 pixel format alone",
      { pixel_format: { String: "yuv444p", Valid: true } },
    ],
  ])("refuses direct play for %s while keeping remux", (_name, overrides) => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(overrides),
        audioStreams: [aacAudio()],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).not.toContain("direct");
    expect(ids).toContain("remux");
  });

  // "nv12" and "yuv410p" contain "12"/"10", so a substring marker list read
  // them as high bit depth and refused a file browsers decode fine.
  it.each(["yuv420p", "yuvj420p", "nv12", "nv21"])(
    "keeps direct play for the 8-bit 4:2:0 pixel format %s",
    pixelFormat => {
      const ids = modeIds(
        getAvailableModes({
          videoStreamsLoaded: true,
          video: h264Video({
            pixel_format: { String: pixelFormat, Valid: true },
          }),
          audioStreams: [aacAudio()],
          mimeType: "video/mp4",
        }),
      );
      expect(ids).toContain("direct");
    },
  );

  it("does not consult the probe when the static rules refuse", () => {
    const canPlay = vi.fn().mockReturnValue("probably");
    getAvailableModes({
      videoStreamsLoaded: true,
      video: h264Video({ bit_depth: { Int64: 10, Valid: true } }),
      audioStreams: [aacAudio()],
      mimeType: "video/mp4",
      canPlay,
    });
    expect(canPlay).not.toHaveBeenCalled();
  });
});

describe("directPlayModeLabel", () => {
  const audioStream = (language: string | null): AudioStreamType => ({
    id: 1,
    movie_id: 1,
    stream_index: 1,
    codec: "aac",
    codec_profile: nullString,
    bit_rate: 128000,
    sample_rate: nullInt,
    channels: 2,
    channel_layout: { String: "stereo", Valid: true },
    language:
      language === null
        ? { String: "", Valid: false }
        : { String: language, Valid: true },
    title: { String: "", Valid: false },
    is_default: false,
  });

  // Audit D9: when direct play is active the audible stream is always
  // ordinal 0, so the badge can name its language with certainty.
  it("names the first stream's language", () => {
    expect(directPlayModeLabel([audioStream("eng")])).toBe(
      "Original file — English audio",
    );
  });

  it("falls back to the generic label when the language is unknown", () => {
    expect(directPlayModeLabel([audioStream(null)])).toBe(
      "Original file — plays as-is",
    );
  });

  it("falls back to the generic label with no audio streams", () => {
    expect(directPlayModeLabel([])).toBe("Original file — plays as-is");
    expect(directPlayModeLabel(undefined)).toBe("Original file — plays as-is");
  });
});

describe("getAvailableModes canPlayType gate", () => {
  it("asks the probe with the full RFC 6381 type string", () => {
    const canPlay = vi.fn().mockReturnValue("probably");
    getAvailableModes({
      videoStreamsLoaded: true,
      video: h264Video(),
      audioStreams: [aacAudio()],
      mimeType: "video/mp4",
      canPlay,
    });
    expect(canPlay).toHaveBeenCalledWith(
      'video/mp4; codecs="avc1.640029, mp4a.40.2"',
    );
  });

  it("removes only direct when the probe refuses a statically eligible file", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [aacAudio()],
        mimeType: "video/mp4",
        canPlay: () => "",
      }),
    );
    expect(ids).not.toContain("direct");
    expect(ids).toContain("remux");
  });

  it("never lets the probe widen eligibility past the static rules", () => {
    const canPlay = vi.fn().mockReturnValue("probably");
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video(),
        audioStreams: [aacAudio()],
        mimeType: "video/x-matroska",
        canPlay,
      }),
    );
    expect(ids).not.toContain("direct");
    expect(canPlay).not.toHaveBeenCalled();
  });

  it("treats a 'maybe' answer as playable", () => {
    const ids = modeIds(
      getAvailableModes({
        videoStreamsLoaded: true,
        video: h264Video({ codec_profile: nullString, codec_level: nullInt }),
        audioStreams: [aacAudio()],
        mimeType: "video/mp4",
        canPlay: (typeString) => (typeString === "video/mp4" ? "maybe" : ""),
      }),
    );
    expect(ids).toContain("direct");
  });
});
