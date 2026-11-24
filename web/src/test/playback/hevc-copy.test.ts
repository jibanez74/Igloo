import { describe, expect, it, vi } from "vitest";
import {
  detectHevcCopyCapability,
  HEVC_COPY_PROBE_TYPE,
  isRemuxSafeHevc,
  NO_HEVC_COPY,
  remuxCopiesHevc,
  type HevcCopyCapability,
} from "@/lib/hevc-copy";
import {
  getAvailableModes,
  hlsVideoCodecsFor,
  type DirectPlayVideoInfo,
} from "@/lib/playback";
import { buildStreamUrl } from "@/lib/video-playback";
import { movieMediaRef } from "@/lib/media-ref";
import { nullableInt64, nullableString } from "../helpers/fixtures";

const capable: HevcCopyCapability = { decodesHevc: true, hdrDisplay: false };
const capableHdr: HevcCopyCapability = { decodesHevc: true, hdrDisplay: true };

const hevcVideo = (
  overrides: Partial<DirectPlayVideoInfo> = {},
): DirectPlayVideoInfo => ({
  codec: "hevc",
  codec_profile: { String: "Main 10", Valid: true },
  codec_level: { Int64: 153, Valid: true },
  height: 2160,
  bit_depth: { Int64: 10, Valid: true },
  pixel_format: { String: "yuv420p10le", Valid: true },
  field_order: nullableString(),
  color_transfer: nullableString(),
  dv_bl_signal_compatibility_id: nullableInt64(),
  ...overrides,
});

const hdr10 = { color_transfer: { String: "smpte2084", Valid: true } };

describe("detectHevcCopyCapability", () => {
  const noMedia = () => {
    throw new Error("an incapable decoder must not ask about the display");
  };

  it("asks MSE when hls.js plays the stream", () => {
    const isTypeSupported = vi.fn(() => true);
    const canPlayType = vi.fn(() => "");

    expect(
      detectHevcCopyCapability({
        nativeHls: false,
        canPlayType,
        mediaSource: { isTypeSupported },
        matchesMedia: (query) => query === "(dynamic-range: high)",
      }),
    ).toEqual({ decodesHevc: true, hdrDisplay: true });
    expect(isTypeSupported).toHaveBeenCalledWith(HEVC_COPY_PROBE_TYPE);
    expect(canPlayType).not.toHaveBeenCalled();
  });

  it("asks canPlayType for a native HLS engine, trusting only probably", () => {
    const env = (answer: string) => ({
      nativeHls: true,
      canPlayType: () => answer,
      mediaSource: undefined,
      matchesMedia: () => false,
    });

    expect(detectHevcCopyCapability(env("probably"))).toEqual({
      decodesHevc: true,
      hdrDisplay: false,
    });
    expect(detectHevcCopyCapability(env("maybe"))).toEqual(NO_HEVC_COPY);
    expect(detectHevcCopyCapability(env(""))).toEqual(NO_HEVC_COPY);
  });

  it("declares nothing without an engine that decodes HEVC", () => {
    expect(
      detectHevcCopyCapability({
        nativeHls: false,
        canPlayType: () => "probably",
        mediaSource: undefined,
        matchesMedia: noMedia,
      }),
    ).toEqual(NO_HEVC_COPY);
    expect(
      detectHevcCopyCapability({
        nativeHls: false,
        canPlayType: () => "probably",
        mediaSource: { isTypeSupported: () => false },
        matchesMedia: noMedia,
      }),
    ).toEqual(NO_HEVC_COPY);
  });

  it("reports no HDR display when matchMedia is missing", () => {
    expect(
      detectHevcCopyCapability({
        nativeHls: false,
        canPlayType: () => "",
        mediaSource: { isTypeSupported: () => true },
        matchesMedia: undefined,
      }),
    ).toEqual({ decodesHevc: true, hdrDisplay: false });
  });
});

describe("isRemuxSafeHevc mirrors the server's HEVC gate", () => {
  it.each<[string, Partial<DirectPlayVideoInfo>]>([
    ["Main 10 4:2:0", {}],
    [
      "8-bit Main",
      {
        codec_profile: { String: "Main", Valid: true },
        bit_depth: { Int64: 8, Valid: true },
        pixel_format: { String: "yuv420p", Valid: true },
      },
    ],
    ["the h265 spelling", { codec: "h265" }],
    ["absent fields", { codec_profile: nullableString(), bit_depth: nullableInt64(), pixel_format: nullableString() }],
    ["a Blu-ray profile 7 base layer", { dv_bl_signal_compatibility_id: nullableInt64(6) }],
    ["an HDR10 profile 8 base layer", { dv_bl_signal_compatibility_id: nullableInt64(1) }],
  ])("accepts %s", (_name, overrides) => {
    expect(isRemuxSafeHevc(hevcVideo(overrides))).toBe(true);
  });

  it.each<[string, Partial<DirectPlayVideoInfo>]>([
    ["H.264", { codec: "h264" }],
    ["12-bit", { bit_depth: { Int64: 12, Valid: true } }],
    ["4:2:2", { pixel_format: { String: "yuv422p10le", Valid: true } }],
    ["Rext", { codec_profile: { String: "Rext", Valid: true } }],
    ["interlaced", { field_order: { String: "tt", Valid: true } }],
    ["Dolby Vision profile 5", { dv_bl_signal_compatibility_id: nullableInt64(0) }],
  ])("refuses %s", (_name, overrides) => {
    expect(isRemuxSafeHevc(hevcVideo(overrides))).toBe(false);
  });
});

describe("remuxCopiesHevc", () => {
  it("copies SDR HEVC to any browser that decodes it", () => {
    expect(remuxCopiesHevc(hevcVideo(), capable)).toBe(true);
    expect(remuxCopiesHevc(hevcVideo(), NO_HEVC_COPY)).toBe(false);
  });

  it.each(["smpte2084", "arib-std-b67", "SMPTE2084 "])(
    "copies %s HDR only to an HDR display",
    (transfer) => {
      const video = hevcVideo({
        color_transfer: { String: transfer, Valid: true },
      });
      expect(remuxCopiesHevc(video, capable)).toBe(false);
      expect(remuxCopiesHevc(video, capableHdr)).toBe(true);
    },
  );
});

describe("getAvailableModes with HEVC", () => {
  const modeIds = (hevcCopy: HevcCopyCapability, video = hevcVideo()) =>
    getAvailableModes({
      videoStreamsLoaded: true,
      video,
      audioStreams: [],
      mimeType: "video/mp4",
      hevcCopy,
    }).map((m) => m.id);

  it("offers remux, never direct, when the browser takes the copy", () => {
    const ids = modeIds(capable);
    expect(ids).toContain("remux");
    expect(ids).not.toContain("direct");
    expect(ids).toContain("2160p_16mbps");
  });

  it("offers only transcodes when the browser cannot take the copy", () => {
    expect(modeIds(NO_HEVC_COPY)).not.toContain("remux");
    expect(modeIds(capable, hevcVideo(hdr10))).not.toContain("remux");
    expect(
      modeIds(capable, hevcVideo({ codec_profile: { String: "Rext", Valid: true } })),
    ).not.toContain("remux");
  });

  it("offers HDR remux to an HDR display", () => {
    expect(modeIds(capableHdr, hevcVideo(hdr10))).toContain("remux");
  });

  it("defaults to no HEVC copy where the browser offers none (jsdom)", () => {
    const ids = getAvailableModes({
      videoStreamsLoaded: true,
      video: hevcVideo(),
      audioStreams: [],
      mimeType: "video/mp4",
    }).map((m) => m.id);
    expect(ids).not.toContain("remux");
  });
});

describe("hlsVideoCodecsFor", () => {
  it("declares hevc for a remux the browser takes as a copy", () => {
    expect(hlsVideoCodecsFor("remux", hevcVideo(), capable)).toBe("hevc");
  });

  it("declares nothing for transcodes, H.264, or a copy the browser refuses", () => {
    expect(hlsVideoCodecsFor("1080p_8mbps", hevcVideo(), capable)).toBeUndefined();
    expect(hlsVideoCodecsFor("direct", hevcVideo(), capable)).toBeUndefined();
    expect(
      hlsVideoCodecsFor("remux", hevcVideo({ codec: "h264" }), capable),
    ).toBeUndefined();
    expect(hlsVideoCodecsFor("remux", hevcVideo(hdr10), capable)).toBeUndefined();
    expect(hlsVideoCodecsFor("remux", undefined, capable)).toBeUndefined();
  });
});

describe("buildStreamUrl video_codecs", () => {
  const session = "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4";

  it("carries the declaration before the reload key", () => {
    expect(
      buildStreamUrl(movieMediaRef(9), "remux", 0, 0, 2, session, "hevc"),
    ).toBe(
      `/api/movies/9/hls/remux/playlist.m3u8?playback_session=${session}&start=0&audio_track=0&video_codecs=hevc&reload=2`,
    );
  });

  it("leaves it out when none is declared", () => {
    expect(buildStreamUrl(movieMediaRef(9), "remux", 0, 0, 0, session)).toBe(
      `/api/movies/9/hls/remux/playlist.m3u8?playback_session=${session}&start=0&audio_track=0`,
    );
  });
});
