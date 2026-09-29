import { describe, expect, it } from "vitest";
import { deriveMediaCapabilityBadges } from "@/lib/media-capabilities";
import type { MovieTechnicalDetailsResponse } from "@/types/movies";
import { nullableInt64, nullableString } from "../helpers/fixtures";
import {
  audioStream,
  subtitleStream,
  videoStream,
} from "../helpers/tech-details";

function tech(
  overrides: Partial<MovieTechnicalDetailsResponse>,
): MovieTechnicalDetailsResponse {
  return {
    movie: {
      file_name: "movie.mkv",
      size: 1,
      container: "mkv",
      mime_type: "video/x-matroska",
      run_time: nullableInt64(119),
      duration: { Float64: 7140, Valid: true },
    },
    video_streams: [],
    audio_streams: [],
    subtitles: [],
    chapters: [],
    ...overrides,
  };
}

describe("deriveMediaCapabilityBadges", () => {
  it("returns an empty list while technical details are loading", () => {
    expect(deriveMediaCapabilityBadges(undefined)).toEqual([]);
  });

  it("derives 4K, HDR, 7.1, and CC for a full-capability source", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [
          videoStream({
            width: 3840,
            height: 2160,
            color_transfer: nullableString("smpte2084"),
          }),
        ],
        audio_streams: [
          audioStream({
            codec: "truehd",
            channels: 8,
            channel_layout: nullableString("7.1"),
          }),
        ],
        subtitles: [subtitleStream()],
      }),
    );

    expect(badges.map(b => b.label)).toEqual(["4K", "HDR", "7.1", "CC"]);
  });

  it("labels anamorphic 4K-width sources as 4K", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [videoStream({ width: 3840, height: 1600 })],
      }),
    );

    expect(badges.map(b => b.label)).toEqual(["4K"]);
  });

  it("derives HD and 5.1 without HDR or subtitles", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [videoStream({ width: 1920, height: 1080 })],
        audio_streams: [
          audioStream({
            channels: 6,
            channel_layout: nullableString("5.1(side)"),
          }),
        ],
      }),
    );

    expect(badges.map(b => b.label)).toEqual(["HD", "5.1"]);
  });

  it("uses a generic Surround badge for unrecognized layouts like 6.1", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        audio_streams: [
          audioStream({
            codec: "dts",
            channels: 7,
            channel_layout: nullableString("6.1"),
          }),
        ],
      }),
    );

    expect(badges).toEqual([
      { label: "Surround", description: "Surround sound audio" },
    ]);
  });

  it("labels HLG transfers as HDR", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [
          videoStream({
            width: 3840,
            height: 2160,
            color_transfer: nullableString("arib-std-b67"),
          }),
        ],
      }),
    );

    expect(badges.map(b => b.label)).toEqual(["4K", "HDR"]);
  });

  it("skips resolution and surround badges for SD stereo sources", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [videoStream({ width: 720, height: 480 })],
        audio_streams: [audioStream({ channels: 2 })],
      }),
    );

    expect(badges).toEqual([]);
  });

  it("ignores embedded cover art streams when picking the video stream", () => {
    const badges = deriveMediaCapabilityBadges(
      tech({
        video_streams: [
          videoStream({ codec: "mjpeg", width: 600, height: 900 }),
          videoStream({ width: 3840, height: 2160 }),
        ],
      }),
    );

    expect(badges.map(b => b.label)).toEqual(["4K"]);
  });
});
