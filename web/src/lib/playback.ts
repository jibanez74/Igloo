import {
  BITMAP_SUBTITLE_CODECS,
  ISO_639_2_TO_1,
  LANGUAGE_NAMES,
  STREAM_MODES,
  SUBTITLE_OFF_VALUE,
} from "@/lib/constants";
import {
  buildDirectPlayTypeString,
  createCanPlayProbe,
  type CanPlayProbe,
} from "@/lib/direct-play-probe";
import { unwrapInt, unwrapStringOrUndefined } from "@/lib/nullable";
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
/**
 * Direct play is MP4-only by design. Neither Chrome nor Firefox plays Matroska
 * in a <video> element — it stalls silently at 0ms with no MediaError — so do
 * NOT add video/x-matroska here. video/webm and video/ogg were dead entries:
 * WebM cannot carry H.264 (the only allowed video codec) and .ogv is not a
 * valid scanner extension. See docs/web-direct-playback-audit.md §3.2, §5.6.
 */
const BROWSER_COMPATIBLE_MIME_TYPES = ["video/mp4"];

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

/** Video fields the direct-play eligibility rules consult. */
export type DirectPlayVideoInfo = Pick<
  VideoStreamType,
  "codec" | "codec_profile" | "codec_level" | "height" | "bit_depth" | "pixel_format"
>;

const NON_BROWSER_H264_PROFILE_MARKERS = ["10", "4:2:2", "422", "4:4:4", "444"];
const NON_BROWSER_H264_PIXEL_FORMAT_MARKERS = [
  "10",
  "12",
  "14",
  "16",
  "422",
  "444",
];

/**
 * Browsers cannot decode 10-bit, 4:2:2 or 4:4:4 H.264 even though the codec
 * name passes the allowlist. Mirrors the server's
 * isBrowserSafeH264RemuxCandidate rules (server/cmd/api/hls_session.go) —
 * kept as an explicit static rule even though the canPlayType probe usually
 * catches these, because the watch-room server relies on the same rules and
 * cannot probe. Audit §3.3 (D2).
 */
function isBrowserSafeH264(video: DirectPlayVideoInfo): boolean {
  const bitDepth = unwrapInt(video.bit_depth);
  if (bitDepth !== null && bitDepth > 8) return false;

  const pixelFormat = unwrapStringOrUndefined(video.pixel_format)
    ?.trim()
    .toLowerCase();
  if (
    pixelFormat &&
    NON_BROWSER_H264_PIXEL_FORMAT_MARKERS.some((m) => pixelFormat.includes(m))
  ) {
    return false;
  }

  const profile = unwrapStringOrUndefined(video.codec_profile)
    ?.trim()
    .toLowerCase();
  if (
    profile &&
    NON_BROWSER_H264_PROFILE_MARKERS.some((m) => profile.includes(m))
  ) {
    return false;
  }

  return true;
}

/** Audio fields the direct-play eligibility rules consult. */
export type DirectPlayAudioInfo = Pick<
  AudioStreamType,
  "codec" | "codec_profile" | "is_default"
>;

/**
 * Whether direct play can guarantee which audio stream the browser decodes:
 * refuse on ambiguity, not on absence. With no `default` flags at all,
 * browsers follow container track order, so the first stream is the one that
 * plays. Mirrors directPlayAudioSelectionUnambiguous in the Go server
 * (watch_room_handler.go) — keep the two in sync. Audit §6.2 (D8).
 */
export function directPlayAudioSelectionEligible(
  audioStreams: Pick<AudioStreamType, "is_default">[],
): boolean {
  if (audioStreams.length <= 1) return true;
  const defaultCount = audioStreams.filter((s) => s.is_default).length;
  if (defaultCount === 0) return true;
  return defaultCount === 1 && audioStreams[0].is_default;
}

export type AvailableModesArgs = {
  /**
   * The primary video stream. When absent while `videoStreamsLoaded` is
   * false, all non-transcode modes are offered (metadata still in flight);
   * when absent while it is true, the movie has no playable video stream and
   * direct/remux are refused (audit D17).
   */
  video?: DirectPlayVideoInfo;
  /**
   * Whether technical details have loaded, so an absent `video` means "no
   * playable video stream" rather than "unknown".
   */
  videoStreamsLoaded: boolean;
  /**
   * Audio streams in `stream_index` order. Only the FIRST stream can affect
   * direct-play eligibility: direct play serves the raw container, and the
   * browser decodes the container's first audio track.
   */
  audioStreams?: DirectPlayAudioInfo[];
  mimeType?: string;
  /** Injectable for tests; defaults to a real `canPlayType` probe. */
  canPlay?: CanPlayProbe;
};

const defaultCanPlayProbe = createCanPlayProbe();

/**
 * Available stream modes for the source. Without `video`, all modes are
 * returned (e.g. before technical details load). With codecs, filters direct /
 * remux / transcode per browser and resolution rules.
 *
 * Invariant relied on by `resolveModeForAudioTrack`: direct requires a playable
 * video codec, audio codec and container while remux requires only the video
 * codec, so remux is available whenever direct is.
 */
export function getAvailableModes(args: AvailableModesArgs) {
  const { video, audioStreams, mimeType } = args;
  const sourceHeight = video?.height ?? 0;

  return STREAM_MODES.filter((m) => {
    if (m.type === "direct") {
      if (!video) return !args.videoStreamsLoaded;
      const staticRulesPass =
        isVideoDirectPlayable(video.codec) &&
        isBrowserSafeH264(video) &&
        isAudioDirectPlayable(audioStreams?.[0]?.codec) &&
        isContainerDirectPlayable(mimeType ?? "") &&
        directPlayAudioSelectionEligible(audioStreams ?? []);
      if (!staticRulesPass) return false;
      // The probe may only narrow eligibility, never widen it: the watch-room
      // server enforces the same direct ⊂ remux invariant from the static
      // rules alone and cannot ask a browser. Keep this an AND, never an OR.
      // See docs/web-direct-playback-audit.md §3.4 (D3).
      const canPlay = args.canPlay ?? defaultCanPlayProbe;
      const typeString = buildDirectPlayTypeString(video, audioStreams?.[0]);
      return canPlay(typeString) !== "";
    }
    if (m.type === "remux") {
      if (!video) return !args.videoStreamsLoaded;
      return isVideoDirectPlayable(video.codec);
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

/**
 * Player-badge label for direct play. Whenever direct play is actually
 * chosen, the audible stream is ordinal 0 of the stream_index-ordered rows
 * (see directPlayAudioSelectionEligible), so its language can be named with
 * certainty (audit D9). Falls back to the generic mode label when the
 * language is unknown.
 */
export function directPlayModeLabel(
  audioStreams: AudioStreamType[] | undefined,
): string {
  const fallback =
    STREAM_MODES.find(m => m.id === "direct")?.label ?? "direct";
  const langName = formatLanguageName(
    unwrapStringOrUndefined(audioStreams?.[0]?.language),
  );
  if (!langName) return fallback;
  const base = fallback.split(" — ")[0];
  return `${base} — ${langName} audio`;
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
