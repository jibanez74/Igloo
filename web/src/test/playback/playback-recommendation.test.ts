import { describe, it, expect } from "vitest";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import type { PlaybackProfileType } from "@/types";

const PROFILES: PlaybackProfileType[] = [
  { id: "240p_1mbps", label: "240p · 1 Mbps", height: 240, video_mbps: 1 },
  { id: "480p_2mbps", label: "480p · 2 Mbps", height: 480, video_mbps: 2 },
  { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
  { id: "1080p_6mbps", label: "1080p · 6 Mbps", height: 1080, video_mbps: 6 },
  { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
];

describe("recommendedProfileId", () => {
  it("returns null when both download and server upload are null", () => {
    expect(recommendedProfileId(PROFILES, null, null)).toBeNull();
  });

  it("uses download alone when server upload is null", () => {
    expect(recommendedProfileId(PROFILES, 100, null)).toBe("1080p_8mbps");
  });

  it("uses server upload alone when download is null", () => {
    expect(recommendedProfileId(PROFILES, null, 25)).toBe("1080p_8mbps");
  });

  it("takes the min of download and server upload", () => {
    expect(recommendedProfileId(PROFILES, 100, 25)).toBe(
      recommendedProfileId(PROFILES, 25, 25),
    );
    expect(recommendedProfileId(PROFILES, 100, 25)).toBe("1080p_8mbps");
  });

  it("selects the lowest profile when only the smallest fits under the cap", () => {
    // cap = 2 * 0.8 = 1.6 Mbps — only the 1 Mbps profile fits
    expect(recommendedProfileId(PROFILES, 2, null)).toBe("240p_1mbps");
  });

  it("falls back to the lowest profile when no profile fits under the cap", () => {
    // cap = 0.5 * 0.8 = 0.4 Mbps — below every profile's video_mbps
    expect(recommendedProfileId(PROFILES, 0.5, null)).toBe("240p_1mbps");
  });

  it("picks the highest profile that fits under the headroom cap", () => {
    expect(recommendedProfileId(PROFILES, 10, 10)).toBe("1080p_8mbps");
    expect(recommendedProfileId(PROFILES, 5, 5)).toBe("720p_3mbps");
  });

  it("picks the highest matching profile, not the first encountered", () => {
    const shuffled: PlaybackProfileType[] = [
      PROFILES[2]!,
      PROFILES[0]!,
      PROFILES[4]!,
      PROFILES[1]!,
      PROFILES[3]!,
    ];
    expect(recommendedProfileId(shuffled, 100, null)).toBe("1080p_8mbps");
  });

  it("returns null when given an empty profile catalog", () => {
    expect(recommendedProfileId([], 100, null)).toBeNull();
  });
});
