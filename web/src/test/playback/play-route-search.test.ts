import { describe, expect, it } from "vitest";
import {
  playbackSettingsToPlaySearch,
  playSearchSchema,
  subtitleTrackFromPlaySearch,
} from "@/lib/route-search";

describe("play route search conversion", () => {
  it("parses and round-trips an explicit subtitle-off selection", () => {
    const search = playSearchSchema.parse({
      mode: "direct",
      audio_track: "0",
      subtitle_track: "off",
    });
    const subtitleTrack = subtitleTrackFromPlaySearch(search.subtitle_track);

    expect(subtitleTrack).toBeNull();
    if (subtitleTrack === undefined) {
      throw new Error("Explicit subtitle-off unexpectedly became omitted.");
    }
    expect(
      playbackSettingsToPlaySearch({
        mode: search.mode ?? "direct",
        audioTrack: search.audio_track,
        subtitleTrack,
      }),
    ).toEqual({
      mode: "direct",
      audio_track: 0,
      subtitle_track: "off",
    });
  });

  it("keeps omitted and numeric subtitle selections distinct", () => {
    const omitted = playSearchSchema.parse({});
    const numeric = playSearchSchema.parse({ subtitle_track: "2" });

    expect(subtitleTrackFromPlaySearch(omitted.subtitle_track)).toBeUndefined();
    expect(subtitleTrackFromPlaySearch(numeric.subtitle_track)).toBe(2);
    expect(
      playbackSettingsToPlaySearch({
        mode: "remux",
        audioTrack: 1,
        subtitleTrack: 2,
      }),
    ).toEqual({
      mode: "remux",
      audio_track: 1,
      subtitle_track: 2,
    });
  });
});
