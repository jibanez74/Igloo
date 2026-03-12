export const STREAM_MODES = [
  { id: "direct", label: "Direct stream", maxHeight: 0 },
  { id: "2160p_16mbps", label: "HLS 2160p 16 Mbps", maxHeight: 2160 },
  { id: "1080p_8mbps", label: "HLS 1080p 8 Mbps", maxHeight: 1080 },
  { id: "1080p_6mbps", label: "HLS 1080p 6 Mbps", maxHeight: 1080 },
  { id: "1080p_4mbps", label: "HLS 1080p 4 Mbps", maxHeight: 1080 },
  { id: "720p_3mbps", label: "HLS 720p 3 Mbps", maxHeight: 720 },
] as const;

export type StreamModeId = (typeof STREAM_MODES)[number]["id"];

export type PlaybackSettings = {
  mode: StreamModeId;
  audioTrack: number;
};

export const DEFAULT_PLAYBACK_SETTINGS: PlaybackSettings = {
  mode: "direct",
  audioTrack: 0,
};

export function getAvailableModes(sourceHeight: number) {
  return STREAM_MODES.filter(
    (m) => m.maxHeight === 0 || (sourceHeight > 0 && m.maxHeight <= sourceHeight),
  );
}
