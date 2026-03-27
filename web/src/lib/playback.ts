import { STREAM_MODES, type StreamModeId } from "@/lib/constants";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import type { AudioStreamType } from "@/types/movies";

const BROWSER_COMPATIBLE_VIDEO_CODECS = ["h264", "h.264", "avc", "avc1"];
const BROWSER_COMPATIBLE_AUDIO_CODECS = ["aac", "mp3", "opus", "vorbis", "flac"];
const BROWSER_COMPATIBLE_MIME_TYPES = ["video/mp4", "video/webm", "video/ogg"];

export type { StreamModeId };
export { STREAM_MODES };

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
};

export const DEFAULT_PLAYBACK_SETTINGS: PlaybackSettings = {
  mode: "direct",
  audioTrack: 0,
};

function isVideoDirectPlayable(codec: string): boolean {
  return BROWSER_COMPATIBLE_VIDEO_CODECS.includes(codec.toLowerCase());
}

function isAudioDirectPlayable(codec: string): boolean {
  return BROWSER_COMPATIBLE_AUDIO_CODECS.includes(codec.toLowerCase());
}

function isContainerDirectPlayable(mimeType: string): boolean {
  return BROWSER_COMPATIBLE_MIME_TYPES.includes(mimeType.toLowerCase());
}

/**
 * Returns the stream modes available for a given source.
 *
 * When codec/container info is provided the list is filtered:
 * - "direct" only when video + audio + container are all browser-compatible
 * - "remux" only when the video codec is H.264 (copy-safe for HLS fMP4)
 * - transcode profiles filtered by source resolution as before
 *
 * Without codec info (pre-techData load), all modes are returned so the
 * dialog can render immediately.
 */
export function getAvailableModes(
  sourceHeight: number,
  videoCodec?: string,
  audioCodec?: string,
  mimeType?: string,
) {
  const hasCodecInfo = videoCodec !== undefined;

  return STREAM_MODES.filter((m) => {
    if (m.type === "direct") {
      if (!hasCodecInfo) return true;
      return (
        isVideoDirectPlayable(videoCodec) &&
        isAudioDirectPlayable(audioCodec ?? "") &&
        isContainerDirectPlayable(mimeType ?? "")
      );
    }
    if (m.type === "remux") {
      if (!hasCodecInfo) return true;
      return isVideoDirectPlayable(videoCodec);
    }
    // transcode — filter by resolution (maxHeight is never 0 here; direct/remux handled above)
    return sourceHeight > 0 && m.maxHeight <= sourceHeight;
  });
}

/**
 * Picks the best default mode based on the movie's codecs and container.
 *
 * - H.264 + AAC + mp4/webm → direct (no processing needed)
 * - H.264 + non-AAC         → remux  (video copy, audio transcode)
 * - non-H.264               → best-fit transcode profile for source resolution
 */
export function getDefaultMode(
  videoCodec: string,
  audioCodec: string,
  mimeType: string,
  sourceHeight: number,
): StreamModeId {
  if (isVideoDirectPlayable(videoCodec)) {
    if (isAudioDirectPlayable(audioCodec) && isContainerDirectPlayable(mimeType)) {
      return "direct";
    }
    return "remux";
  }

  // Non-H.264: pick the highest transcode profile that fits the source
  const transcodes = STREAM_MODES.filter(
    (m) =>
      m.type === "transcode" &&
      sourceHeight > 0 &&
      m.maxHeight <= sourceHeight,
  );
  if (transcodes.length > 0) return transcodes[0].id;

  // Fallback: lowest profile
  return "720p_3mbps";
}

/** Common ISO 639-1 codes → English display names for audio track labels. */
const AUDIO_LANGUAGE_NAMES: Record<string, string> = {
  en: "English",
  es: "Spanish",
  fr: "French",
  de: "German",
  it: "Italian",
  ja: "Japanese",
  ko: "Korean",
  zh: "Chinese",
  pt: "Portuguese",
  ru: "Russian",
  hi: "Hindi",
  nl: "Dutch",
  sv: "Swedish",
  no: "Norwegian",
  da: "Danish",
  fi: "Finnish",
  pl: "Polish",
  tr: "Turkish",
};
function formatAudioLanguageName(
  raw: string | undefined,
): string | undefined {
  const code = raw?.trim().toLowerCase();
  if (!code) return undefined;
  const two = code.slice(0, 2);
  if (AUDIO_LANGUAGE_NAMES[two]) return AUDIO_LANGUAGE_NAMES[two];
  return code.length <= 3
    ? code.toUpperCase()
    : code.charAt(0).toUpperCase() + code.slice(1);
}

/**
 * Human-readable channel layout (FFmpeg-style layout or channel count).
 */
function describePlaybackChannelLayout(
  channelLayout: string | undefined,
  channels: number,
): string {
  const l = channelLayout?.toLowerCase() ?? "";
  if (l.includes("mono") || channels === 1) return "Mono";
  if (l.includes("stereo") || channels === 2) return "Stereo";
  if (l.includes("5.1") || l.includes("5.1(")) return "5.1 surround";
  if (l.includes("7.1")) return "7.1 surround";
  if (l.includes("quad") || l.includes("4.0")) return "Quad";
  if (channels >= 6) return "Surround";
  if (channels === 2) return "Stereo";
  if (channels === 1) return "Mono";
  return `${channels} channels`;
}

/**
 * Dropdown label for an audio stream (language + channel layout; no codec).
 */
export function formatPlaybackAudioLabel(
  stream: AudioStreamType,
  index: number,
): string {
  const langRaw = unwrapStringOrUndefined(stream.language);
  const langName = formatAudioLanguageName(langRaw);
  const channels = describePlaybackChannelLayout(
    unwrapStringOrUndefined(stream.channel_layout),
    stream.channels,
  );
  if (langName) {
    return `${langName} · ${channels}`;
  }
  return `Track ${index + 1} · ${channels}`;
}

/**
 * Short summary of what playback will feel like for the chosen mode and track.
 */
export function describePlaybackExperience(
  mode: StreamModeId,
  audioStream: AudioStreamType | undefined,
  audioTrackIndex: number,
): string {
  const entry = STREAM_MODES.find(m => m.id === mode);
  const modeType = entry?.type ?? "transcode";

  let videoPart: string;
  if (modeType === "direct") {
    videoPart =
      "Your movie plays directly in the browser with no conversion—the best option when your file already matches what the browser can play.";
  } else if (modeType === "remux") {
    videoPart =
      "The picture stays the same; the soundtrack is adjusted so playback works reliably in your browser.";
  } else {
    const label = entry?.label ?? "your selected quality";
    videoPart = `Video is converted and streamed for smooth playback (${label}). A steady internet connection helps.`;
  }

  const audioPart = audioStream
    ? ` You'll hear: ${formatPlaybackAudioLabel(audioStream, audioTrackIndex)}.`
    : " Default audio is used.";

  return videoPart + audioPart;
}
