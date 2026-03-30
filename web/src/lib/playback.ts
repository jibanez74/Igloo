import {
  BITMAP_SUBTITLE_CODECS,
  LANGUAGE_NAMES,
  STREAM_MODES,
  type StreamModeId,
} from "@/lib/constants";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import type { AudioStreamType, SubtitleType, VideoStreamType } from "@/types/movies";

const BROWSER_COMPATIBLE_VIDEO_CODECS = ["h264", "h.264", "avc", "avc1"];
const BROWSER_COMPATIBLE_AUDIO_CODECS = ["aac", "mp3", "opus", "vorbis", "flac"];
const BROWSER_COMPATIBLE_MIME_TYPES = ["video/mp4", "video/webm", "video/ogg"];

export type { StreamModeId };
export { STREAM_MODES };

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
  subtitleTrack: number | null;
};

export const DEFAULT_PLAYBACK_SETTINGS: PlaybackSettings = {
  mode: "direct",
  audioTrack: 0,
  subtitleTrack: null,
};

const COVER_ART_CODECS = ["mjpeg", "png", "gif", "bmp"];

/** First real video stream, skipping embedded cover art (mjpeg/png/gif/bmp). */
export function getPrimaryVideoStream(
  streams: VideoStreamType[] | undefined,
): VideoStreamType | undefined {
  if (!streams || streams.length === 0) return undefined;
  const primary = streams.find(
    (s) => !COVER_ART_CODECS.includes(s.codec.toLowerCase()),
  );
  return primary ?? streams[0];
}

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
 * Available stream modes for the source. Without `videoCodec`, all modes are
 * returned (e.g. before technical details load). With codecs, filters direct /
 * remux / transcode per browser and resolution rules.
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
    return sourceHeight > 0 && m.maxHeight <= sourceHeight;
  });
}

/** Default mode from codecs, container, and source height (direct → remux → transcode). */
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

  const transcodes = STREAM_MODES.filter(
    (m) =>
      m.type === "transcode" &&
      sourceHeight > 0 &&
      m.maxHeight <= sourceHeight,
  );
  if (transcodes.length > 0) return transcodes[0].id;

  return "720p_3mbps";
}

export function formatLanguageName(
  raw: string | undefined,
): string | undefined {
  const code = raw?.trim().toLowerCase();
  if (!code) return undefined;
  const two = code.slice(0, 2);
  if (LANGUAGE_NAMES[two]) return LANGUAGE_NAMES[two];
  return code.length <= 3
    ? code.toUpperCase()
    : code.charAt(0).toUpperCase() + code.slice(1);
}

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

export function formatPlaybackAudioLabel(
  stream: AudioStreamType,
  index: number,
): string {
  const langRaw = unwrapStringOrUndefined(stream.language);
  const langName = formatLanguageName(langRaw);
  const channels = describePlaybackChannelLayout(
    unwrapStringOrUndefined(stream.channel_layout),
    stream.channels,
  );
  if (langName) {
    return `${langName} · ${channels}`;
  }
  return `Track ${index + 1} · ${channels}`;
}

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

export function isBitmapSubtitleCodec(codec: string): boolean {
  return (BITMAP_SUBTITLE_CODECS as readonly string[]).includes(
    codec.toLowerCase(),
  );
}

export function formatSubtitleLabel(
  subtitle: SubtitleType,
  index: number,
): string {
  const lang = formatLanguageName(unwrapStringOrUndefined(subtitle.language));
  const title = unwrapStringOrUndefined(subtitle.title);

  const parts: string[] = [];
  if (lang) parts.push(lang);
  if (title && title !== lang) parts.push(title);
  if (subtitle.is_forced) parts.push("Forced");
  if (subtitle.is_default) parts.push("Default");

  if (parts.length > 0) return parts.join(" · ");
  return `Track ${index + 1}`;
}
