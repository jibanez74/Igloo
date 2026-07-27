import { describe, it, expect, vi } from "vitest";
import {
  directPlayAudioSelectionEligible,
  getAvailableModes,
} from "@/lib/playback";
import type { DirectPlayAudioInfo, DirectPlayVideoInfo } from "@/lib/playback";

const nullString = { String: "", Valid: false };
const nullInt = { Int64: 0, Valid: false };

const h264Video = (
  overrides: Partial<DirectPlayVideoInfo> = {},
): DirectPlayVideoInfo => ({
  codec: "h264",
  codec_profile: { String: "High", Valid: true },
  codec_level: { Int64: 41, Valid: true },
  height: 1080,
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
        video: h264Video(),
        audioStreams: [aacAudio(), aacAudio(), aacAudio()],
        mimeType: "video/mp4",
      }),
    );
    expect(ids).toContain("direct");
  });
});

describe("getAvailableModes canPlayType gate", () => {
  it("asks the probe with the full RFC 6381 type string", () => {
    const canPlay = vi.fn().mockReturnValue("probably");
    getAvailableModes({
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
        video: h264Video({ codec_profile: nullString, codec_level: nullInt }),
        audioStreams: [aacAudio()],
        mimeType: "video/mp4",
        canPlay: (typeString) => (typeString === "video/mp4" ? "maybe" : ""),
      }),
    );
    expect(ids).toContain("direct");
  });
});
