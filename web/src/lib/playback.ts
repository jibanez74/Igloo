import { STREAM_MODES, type StreamModeId } from "@/lib/constants";

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
