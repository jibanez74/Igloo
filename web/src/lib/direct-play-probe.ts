import { unwrapInt, unwrapStringOrUndefined } from "@/lib/nullable";
import type { AudioStreamType, VideoStreamType } from "@/types/movies";

/** Result of `HTMLMediaElement.canPlayType`: `""` | `"maybe"` | `"probably"`. */
export type CanPlayProbe = (typeString: string) => string;

/**
 * ffprobe H.264 profile names → RFC 6381 `avc1` profile + constraint-flag
 * prefix (PPCC). ffprobe does not expose constraint bits, so each entry uses
 * the interoperable convention muxers and browsers agree on.
 */
const H264_PROFILE_PREFIXES: Record<string, string> = {
  "constrained baseline": "4240",
  baseline: "42E0",
  main: "4D40",
  extended: "5800",
  high: "6400",
  "high 10": "6E00",
  "high 10 intra": "6E10",
  "high 4:2:2": "7A00",
  "high 4:2:2 intra": "7A10",
  "high 4:4:4 predictive": "F400",
};

/** ffprobe AAC profile names → RFC 6381 audio codec strings. */
const AAC_PROFILE_CODECS: Record<string, string> = {
  lc: "mp4a.40.2",
  "he-aac": "mp4a.40.5",
  "he-aacv2": "mp4a.40.29",
};

type ProbeVideoInfo = Pick<VideoStreamType, "codec_profile" | "codec_level">;
type ProbeAudioInfo = Pick<AudioStreamType, "codec" | "codec_profile">;

function audioCodecString(audio: ProbeAudioInfo | undefined): string | undefined {
  // mp3/flac/opus tokens inside video/mp4 get inconsistent canPlayType answers
  // across browsers; omitting them keeps the probe from falsely narrowing —
  // the static audio allowlist still governs those codecs.
  if (!audio || audio.codec.toLowerCase() !== "aac") return undefined;

  const profile = unwrapStringOrUndefined(audio.codec_profile)
    ?.trim()
    .toLowerCase();
  if (!profile) return AAC_PROFILE_CODECS.lc;
  return AAC_PROFILE_CODECS[profile] ?? AAC_PROFILE_CODECS.lc;
}

/**
 * Builds the MIME type string a `canPlayType` probe should ask about for a
 * direct-play candidate. Callers only reach this after the static rules have
 * confirmed an H.264 family codec in an MP4 container.
 *
 * When the profile is unmappable or the level is missing, this returns the
 * bare `video/mp4` — `canPlayType` answers `"maybe"` for it, so the probe adds
 * no narrowing when it has nothing reliable to say. Never guess a profile:
 * guessing low would widen eligibility relative to reality, and guessing high
 * would spuriously refuse valid files.
 */
export function buildDirectPlayTypeString(
  video: ProbeVideoInfo,
  audio?: ProbeAudioInfo,
): string {
  const profile = unwrapStringOrUndefined(video.codec_profile)
    ?.trim()
    .toLowerCase();
  const prefix = profile ? H264_PROFILE_PREFIXES[profile] : undefined;
  const level = unwrapInt(video.codec_level);
  const levelUsable = level != null && level > 0 && level <= 255;
  if (!prefix || !levelUsable) return "video/mp4";

  const levelHex = level.toString(16).padStart(2, "0").toUpperCase();
  const codecs = [`avc1.${prefix}${levelHex}`];
  const audioCodec = audioCodecString(audio);
  if (audioCodec) codecs.push(audioCodec);
  return `video/mp4; codecs="${codecs.join(", ")}"`;
}

/**
 * Creates a `canPlayType` probe backed by a lazily created `<video>` element.
 * A plain injectable function on purpose — module-load IIFEs (like
 * `supportsNativeHLS`) cannot be unit-tested. When no element can be created
 * (no DOM), the probe reports `"maybe"`: a non-probing environment must not
 * spuriously refuse, and it cannot widen because the static rules still apply.
 */
export function createCanPlayProbe(
  createElement: () => Pick<HTMLVideoElement, "canPlayType"> | null = () =>
    typeof document === "undefined" ? null : document.createElement("video"),
): CanPlayProbe {
  let element: Pick<HTMLVideoElement, "canPlayType"> | null | undefined;
  return (typeString) => {
    if (element === undefined) {
      element = createElement();
    }
    if (element === null) return "maybe";
    return element.canPlayType(typeString);
  };
}
