// Builders for the technical-details stream rows (GET
// /api/movies/{id}/technical-details): a 1080p H.264 video, an English AAC
// stereo track and an English SubRip subtitle. Override any field per test.

import type { AudioStreamType, SubtitleType, VideoStreamType } from "@/types";
import { nullableInt64, nullableString } from "./fixtures";

export function videoStream(
  overrides: Partial<VideoStreamType> = {},
): VideoStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 0,
    codec: "h264",
    codec_profile: nullableString("Main"),
    codec_level: nullableInt64(41),
    bit_rate: 4_000_000,
    width: 1920,
    height: 1080,
    coded_width: nullableInt64(1920),
    coded_height: nullableInt64(1080),
    aspect_ratio: nullableString("16:9"),
    frame_rate: 24,
    avg_frame_rate: nullableString("24/1"),
    bit_depth: nullableInt64(8),
    pixel_format: nullableString("yuv420p"),
    color_range: nullableString(),
    color_space: nullableString(),
    color_primaries: nullableString(),
    color_transfer: nullableString(),
    field_order: nullableString(),
    rotation: nullableInt64(),
    language: nullableString(),
    title: nullableString(),
    ...overrides,
  };
}

export function audioStream(
  overrides: Partial<AudioStreamType> = {},
): AudioStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 1,
    codec: "aac",
    codec_profile: nullableString("LC"),
    bit_rate: 192_000,
    sample_rate: nullableInt64(48_000),
    channels: 2,
    channel_layout: nullableString("stereo"),
    language: nullableString("eng"),
    title: nullableString(),
    is_default: false,
    ...overrides,
  };
}

export function subtitleStream(
  overrides: Partial<SubtitleType> = {},
): SubtitleType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 2,
    codec: "subrip",
    language: nullableString("eng"),
    title: nullableString(),
    is_forced: false,
    is_default: false,
    ...overrides,
  };
}
