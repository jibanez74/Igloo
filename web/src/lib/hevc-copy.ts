import { unwrapInt, unwrapNormalizedString } from "@/lib/nullable";
import type { PlaybackVideoStreamType } from "@/types/playback";

/**
 * What this browser can do with a copied HEVC stream: decode HEVC Main 10
 * from fragmented MP4, and show HDR. A remux copies HDR10 or HLG untouched,
 * so an HDR source is only declared to a display that reports HDR; otherwise
 * the server transcodes it to tone-mapped H.264, as before.
 */
export type HevcCopyCapability = {
  decodesHevc: boolean;
  hdrDisplay: boolean;
};

/** The capability of a client that never declares HEVC (watch rooms, no DOM). */
export const NO_HEVC_COPY: HevcCopyCapability = {
  decodesHevc: false,
  hdrDisplay: false,
};

/**
 * HEVC Main 10, level 5.1 (the 4K Blu-ray ceiling) in an `hvc1` sample entry,
 * which is how the server writes a copied stream (`-tag:v hvc1`).
 */
export const HEVC_COPY_PROBE_TYPE = 'video/mp4; codecs="hvc1.2.4.L153.B0"';

const HDR_DISPLAY_QUERY = "(dynamic-range: high)";

type HevcCopyEnvironment = {
  /** True when HLS goes to the browser's own engine (see prefersNativeHLS). */
  nativeHls: boolean;
  canPlayType: (type: string) => string;
  mediaSource: Pick<typeof MediaSource, "isTypeSupported"> | undefined;
  matchesMedia: ((query: string) => boolean) | undefined;
};

/**
 * Asks the engine that will play the stream. hls.js feeds Media Source
 * Extensions, so MSE answers for it; a native HLS engine answers through
 * `canPlayType`, where only "probably" counts: a false "no" costs a transcode,
 * which is what happens today, while a false "yes" costs a broken start.
 */
export function detectHevcCopyCapability({
  nativeHls,
  canPlayType,
  mediaSource,
  matchesMedia,
}: HevcCopyEnvironment): HevcCopyCapability {
  const decodesHevc = nativeHls
    ? canPlayType(HEVC_COPY_PROBE_TYPE) === "probably"
    : mediaSource?.isTypeSupported(HEVC_COPY_PROBE_TYPE) === true;
  if (!decodesHevc) return NO_HEVC_COPY;
  return {
    decodesHevc,
    hdrDisplay: matchesMedia?.(HDR_DISPLAY_QUERY) === true,
  };
}

/** Video fields the HEVC copy rules read, beyond the H.264 ones. */
export type HevcCopyVideoInfo = Pick<
  PlaybackVideoStreamType,
  "codec" | "codec_profile" | "bit_depth" | "pixel_format" | "field_order"
> &
  Partial<
    Pick<
      PlaybackVideoStreamType,
      "color_transfer" | "dv_bl_signal_compatibility_id"
    >
  >;

const HEVC_CODECS = ["hevc", "h265", "h.265"];
const REMUX_HEVC_PROFILES = ["main", "main 10"];
const REMUX_HEVC_PIXEL_FORMATS = ["yuv420p", "yuv420p10le"];
const INTERLACED_FIELD_ORDERS = ["tt", "bb", "tb", "bt"];
/** HDR10 (1, and 6 on Blu-ray profile 7), SDR (2) and HLG (4). */
const REMUX_DOLBY_VISION_BASE_LAYERS = [1, 2, 4, 6];
const HDR_TRANSFERS = ["smpte2084", "arib-std-b67"];

export function isHevcCodec(codec: string): boolean {
  return HEVC_CODECS.includes(codec.trim().toLowerCase());
}

/**
 * The server's HEVC remux gate, which it applies to a request carrying
 * `video_codecs=hevc`. Mirrors isHEVCRemuxCandidate in
 * server/cmd/api/hls_session.go; keep the two in sync, or the client offers
 * a remux the server turns into a transcode. Absent fields do not reject.
 */
export function isRemuxSafeHevc(video: HevcCopyVideoInfo): boolean {
  if (!isHevcCodec(video.codec)) return false;

  const bitDepth = unwrapInt(video.bit_depth);
  if (bitDepth !== null && bitDepth > 10) return false;

  const pixelFormat = unwrapNormalizedString(video.pixel_format);
  if (pixelFormat && !REMUX_HEVC_PIXEL_FORMATS.includes(pixelFormat)) {
    return false;
  }

  const profile = unwrapNormalizedString(video.codec_profile);
  if (profile && !REMUX_HEVC_PROFILES.includes(profile)) return false;

  const fieldOrder = unwrapNormalizedString(video.field_order);
  if (fieldOrder && INTERLACED_FIELD_ORDERS.includes(fieldOrder)) return false;

  const baseLayer = unwrapInt(video.dv_bl_signal_compatibility_id);
  if (baseLayer !== null && !REMUX_DOLBY_VISION_BASE_LAYERS.includes(baseLayer)) {
    return false;
  }

  return true;
}

/** PQ (HDR10, Dolby Vision base layers) or HLG, by the stored transfer. */
export function isHdrTransfer(video: HevcCopyVideoInfo): boolean {
  const transfer = unwrapNormalizedString(video.color_transfer);
  return transfer !== undefined && HDR_TRANSFERS.includes(transfer);
}

/**
 * Whether a remux of this source should declare `video_codecs=hevc`: the
 * server would copy it, this browser decodes it, and an HDR source has an
 * HDR display to go to.
 */
export function remuxCopiesHevc(
  video: HevcCopyVideoInfo,
  capability: HevcCopyCapability,
): boolean {
  if (!capability.decodesHevc || !isRemuxSafeHevc(video)) return false;
  return !isHdrTransfer(video) || capability.hdrDisplay;
}
