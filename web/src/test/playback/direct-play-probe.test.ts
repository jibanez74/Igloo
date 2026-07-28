import { describe, it, expect, vi } from "vitest";
import {
  buildDirectPlayTypeString,
  createCanPlayProbe,
} from "@/lib/direct-play-probe";

const video = (profile: string | null, level: number | null) => ({
  codec_profile:
    profile === null
      ? { String: "", Valid: false }
      : { String: profile, Valid: true },
  codec_level:
    level === null ? { Int64: 0, Valid: false } : { Int64: level, Valid: true },
});

const audio = (codec: string, profile: string | null = null) => ({
  codec,
  codec_profile:
    profile === null
      ? { String: "", Valid: false }
      : { String: profile, Valid: true },
});

describe("buildDirectPlayTypeString", () => {
  it("builds the avc1 + mp4a string for a High 4.1 AAC-LC source", () => {
    expect(buildDirectPlayTypeString(video("High", 41), audio("aac", "LC"))).toBe(
      'video/mp4; codecs="avc1.640029, mp4a.40.2"',
    );
  });

  it.each([
    ["Constrained Baseline", 30, "avc1.42401E"],
    ["Baseline", 30, "avc1.42E01E"],
    ["Main", 40, "avc1.4D4028"],
    ["High", 51, "avc1.640033"],
    ["High 10", 41, "avc1.6E0029"],
    ["High 4:2:2", 41, "avc1.7A0029"],
    ["High 4:4:4 Predictive", 41, "avc1.F40029"],
  ])("maps profile %s level %d to %s", (profile, level, wantCodec) => {
    expect(buildDirectPlayTypeString(video(profile, level))).toBe(
      `video/mp4; codecs="${wantCodec}"`,
    );
  });

  it.each([
    ["HE-AAC", "mp4a.40.5"],
    ["HE-AACv2", "mp4a.40.29"],
    ["LC", "mp4a.40.2"],
  ])("maps AAC profile %s to %s", (profile, wantCodec) => {
    expect(buildDirectPlayTypeString(video("High", 41), audio("aac", profile))).toBe(
      `video/mp4; codecs="avc1.640029, ${wantCodec}"`,
    );
  });

  it("defaults AAC without a stored profile to AAC-LC", () => {
    expect(buildDirectPlayTypeString(video("High", 41), audio("aac"))).toBe(
      'video/mp4; codecs="avc1.640029, mp4a.40.2"',
    );
  });

  it.each(["mp3", "flac", "opus"])(
    "omits %s from the codec string instead of guessing a token",
    (codec) => {
      expect(buildDirectPlayTypeString(video("High", 41), audio(codec))).toBe(
        'video/mp4; codecs="avc1.640029"',
      );
    },
  );

  it.each([
    ["unknown profile", video("Weird Profile", 41)],
    ["missing profile", video(null, 41)],
    ["missing level", video("High", null)],
    ["level out of range", video("High", 999)],
  ])("falls back to bare video/mp4 for %s", (_name, v) => {
    expect(buildDirectPlayTypeString(v)).toBe("video/mp4");
  });
});

describe("createCanPlayProbe", () => {
  it("delegates to the injected element and reuses it across calls", () => {
    const canPlayType = vi.fn().mockReturnValue("probably");
    const createElement = vi.fn(() => ({ canPlayType }));
    const probe = createCanPlayProbe(createElement);

    expect(probe("video/mp4")).toBe("probably");
    expect(probe('video/mp4; codecs="avc1.640029"')).toBe("probably");
    expect(createElement).toHaveBeenCalledTimes(1);
    expect(canPlayType).toHaveBeenCalledTimes(2);
  });

  it("reports 'maybe' when no media element can be created", () => {
    const probe = createCanPlayProbe(() => null);
    expect(probe("video/mp4")).toBe("maybe");
  });

  it("does not create the element until the first probe call", () => {
    const createElement = vi.fn(() => ({ canPlayType: () => "" }));
    createCanPlayProbe(createElement);
    expect(createElement).not.toHaveBeenCalled();
  });
});
