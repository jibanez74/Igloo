import {
  BITMAP_SUBTITLE_CODECS,
  ISO_639_2_TO_1,
  LANGUAGE_NAMES,
  STREAM_MODES,
  SUBTITLE_OFF_VALUE,
} from "@/lib/constants";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import type { AudioStreamType, SubtitleType, VideoStreamType } from "@/types/movies";
import type { PlaybackSettings, StreamModeId } from "@/types/playback";
import type { PlaybackSettingsType } from "@/types/settings";

type PlaybackModeOption = {
  id: StreamModeId;
};

type PlaybackSettingsInput = Omit<PlaybackSettings, "subtitleTrack"> & {
  subtitleTrack?: number | null;
};

const BROWSER_COMPATIBLE_VIDEO_CODECS = ["h264", "h.264", "avc", "avc1"];
const BROWSER_COMPATIBLE_AUDIO_CODECS = ["aac", "mp3", "opus", "vorbis", "flac"];
const BROWSER_COMPATIBLE_MIME_TYPES = ["video/mp4", "video/webm", "video/ogg"];

/**
 * True when the browser can play HLS via MSE without hls.js (e.g. Safari).
 * Evaluated once at module load; false in non-browser environments.
 */
export const supportsNativeHLS = (() => {
  if (typeof document === "undefined") return false;
  const v = document.createElement("video");
  return (
    v.canPlayType("application/vnd.apple.mpegurl") !== "" ||
    v.canPlayType("application/x-mpegURL") !== ""
  );
})();

/** The only track direct play can deliver: the container's first audio stream. */
const DIRECT_PLAY_AUDIO_TRACK = 0;

const DEFAULT_PLAYBACK_SETTINGS: PlaybackSettings = {
  mode: "direct",
  audioTrack: DIRECT_PLAY_AUDIO_TRACK,
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

function isAudioDirectPlayable(codec: string | undefined): boolean {
  if (!codec) return true;
  return BROWSER_COMPATIBLE_AUDIO_CODECS.includes(codec.toLowerCase());
}

function isContainerDirectPlayable(mimeType: string): boolean {
  return BROWSER_COMPATIBLE_MIME_TYPES.includes(mimeType.toLowerCase());
}

/**
 * Available stream modes for the source. Without `videoCodec`, all modes are
 * returned (e.g. before technical details load). With codecs, filters direct /
 * remux / transcode per browser and resolution rules.
 *
 * `audioCodec` must be the FIRST audio stream's codec: direct play serves the
 * raw container, so no other stream can affect direct-play eligibility.
 *
 * Invariant relied on by `resolveModeForAudioTrack`: direct requires a playable
 * video codec, audio codec and container while remux requires only the video
 * codec, so remux is available whenever direct is.
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
        isAudioDirectPlayable(audioCodec) &&
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

/**
 * Direct play serves the raw container, so the browser always decodes the
 * container's first audio track and any other selection would be silently
 * ignored. Upgrading to remux keeps the video stream copied while letting the
 * server map the chosen track. Safe unconditionally — see `getAvailableModes`.
 */
export function resolveModeForAudioTrack(
  mode: StreamModeId,
  audioTrack: number,
): StreamModeId {
  if (mode === "direct" && audioTrack !== DIRECT_PLAY_AUDIO_TRACK) {
    return "remux";
  }
  return mode;
}

/** Counterpart to `resolveModeForAudioTrack` for when the mode is the choice being made. */
export function resolveAudioTrackForMode(
  mode: StreamModeId,
  audioTrack: number,
): number {
  if (mode === "direct") return DIRECT_PLAY_AUDIO_TRACK;
  return audioTrack;
}

export function normalizeLang(raw: string | undefined): string | undefined {
  const lower = raw?.trim().toLowerCase();
  if (!lower) return undefined;
  if (lower.length === 2) return lower;
  if (lower.length === 3) return ISO_639_2_TO_1[lower];
  return undefined;
}

export function getDefaultPlaybackSettings(
  availableModes: readonly PlaybackModeOption[],
  userPrefs?: PlaybackSettingsType | null,
  audioStreams?: AudioStreamType[],
  subtitleStreams?: SubtitleType[],
): PlaybackSettings {
  const fallbackMode =
    availableModes[0]?.id ?? DEFAULT_PLAYBACK_SETTINGS.mode;

  const isInAvailableModes = (id: string) =>
    availableModes.some((m) => m.id === id);

  let mode: StreamModeId = fallbackMode;
  if (userPrefs) {
    if (
      userPrefs.preferred_profile &&
      isInAvailableModes(userPrefs.preferred_profile)
    ) {
      mode = userPrefs.preferred_profile as StreamModeId;
    } else if (userPrefs.download_mbps != null) {
      const recommended = recommendedProfileId(
        userPrefs.profiles,
        userPrefs.download_mbps,
        userPrefs.server_upload_mbps,
      );
      if (recommended && isInAvailableModes(recommended)) {
        mode = recommended as StreamModeId;
      }
    }
  }

  let audioTrack = 0;
  const preferredAudio = normalizeLang(
    userPrefs?.preferred_audio_language ?? undefined,
  );
  if (preferredAudio && audioStreams && audioStreams.length > 0) {
    const matchIndex = audioStreams.findIndex(
      (s) =>
        normalizeLang(unwrapStringOrUndefined(s.language)) === preferredAudio,
    );
    if (matchIndex >= 0) audioTrack = matchIndex;
  }

  let subtitleTrack: number | null = null;
  const preferredSubtitle = userPrefs?.preferred_subtitle_language ?? null;
  if (preferredSubtitle && preferredSubtitle !== SUBTITLE_OFF_VALUE) {
    const normalized = normalizeLang(preferredSubtitle);
    if (normalized && subtitleStreams && subtitleStreams.length > 0) {
      const matchIndex = subtitleStreams.findIndex(
        (s) =>
          !isBitmapSubtitleCodec(s.codec) &&
          normalizeLang(unwrapStringOrUndefined(s.language)) === normalized,
      );
      if (matchIndex >= 0) subtitleTrack = matchIndex;
    }
  }

  return {
    mode: resolveModeForAudioTrack(mode, audioTrack),
    audioTrack,
    subtitleTrack,
  };
}

export function resolvePlaybackSettings(
  settings: PlaybackSettingsInput | null | undefined,
  availableModes: readonly PlaybackModeOption[],
  audioStreams: AudioStreamType[] | undefined,
  subtitleStreams: SubtitleType[] | undefined,
  userPrefs?: PlaybackSettingsType | null,
): PlaybackSettings {
  const defaults = getDefaultPlaybackSettings(
    availableModes,
    userPrefs,
    audioStreams,
    subtitleStreams,
  );
  if (!settings) return defaults;

  const resolvedMode = availableModes.some((mode) => mode.id === settings.mode)
    ? settings.mode
    : defaults.mode;

  const audioStreamCount = audioStreams?.length ?? 0;
  const resolvedAudioTrack =
    audioStreamCount > 0 &&
    Number.isInteger(settings.audioTrack) &&
    settings.audioTrack >= 0 &&
    settings.audioTrack < audioStreamCount
      ? settings.audioTrack
      : defaults.audioTrack;

  const subtitleStreamCount = subtitleStreams?.length ?? 0;
  let resolvedSubtitleTrack = defaults.subtitleTrack;
  if (settings.subtitleTrack === null) {
    resolvedSubtitleTrack = null;
  } else if (
    settings.subtitleTrack !== undefined &&
    Number.isInteger(settings.subtitleTrack) &&
    settings.subtitleTrack >= 0 &&
    settings.subtitleTrack < subtitleStreamCount &&
    !isBitmapSubtitleCodec(
      subtitleStreams?.[settings.subtitleTrack]?.codec ?? "",
    )
  ) {
    resolvedSubtitleTrack = settings.subtitleTrack;
  }

  return {
    mode: resolveModeForAudioTrack(resolvedMode, resolvedAudioTrack),
    audioTrack: resolvedAudioTrack,
    subtitleTrack: resolvedSubtitleTrack,
  };
}

function formatLanguageName(
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

export function describePlaybackChannelLayout(
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
