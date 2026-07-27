import { describe, it, expect } from "vitest";
import {
  getAvailableModes,
  getDefaultPlaybackSettings,
  resolveAudioTrackForMode,
  resolveModeForAudioTrack,
  resolvePlaybackSettings,
} from "@/lib/playback";
import type { StreamModeId } from "@/types/playback";
import type { PlaybackSettingsType } from "@/types/settings";
import type { AudioStreamType, SubtitleType } from "@/types/movies";

const audioStream = (
  index: number,
  language: string | null,
): AudioStreamType => ({
  id: index,
  movie_id: 1,
  stream_index: index,
  codec: "aac",
  codec_profile: { Valid: false, String: "" },
  bit_rate: 128000,
  sample_rate: { Valid: false, Int64: 0 },
  channels: 2,
  channel_layout: { Valid: true, String: "stereo" },
  language:
    language === null
      ? { Valid: false, String: "" }
      : { Valid: true, String: language },
  title: { Valid: false, String: "" },
  is_default: false,
});

const subtitleStream = (
  index: number,
  language: string | null,
): SubtitleType => ({
  id: index,
  movie_id: 1,
  stream_index: index,
  codec: "subrip",
  language:
    language === null
      ? { Valid: false, String: "" }
      : { Valid: true, String: language },
  title: { Valid: false, String: "" },
  is_forced: false,
  is_default: false,
});

const ALL_MODES: { id: StreamModeId }[] = [
  { id: "direct" },
  { id: "remux" },
  { id: "2160p_16mbps" },
  { id: "1080p_8mbps" },
  { id: "1080p_6mbps" },
  { id: "1080p_4mbps" },
  { id: "720p_3mbps" },
];

const SUB_HD_MODES: { id: StreamModeId }[] = [
  { id: "remux" },
  { id: "720p_3mbps" },
];

const PROFILES = [
  { id: "2160p_16mbps", label: "2160p · 16 Mbps", height: 2160, video_mbps: 16 },
  { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
  { id: "1080p_6mbps", label: "1080p · 6 Mbps", height: 1080, video_mbps: 6 },
  { id: "1080p_4mbps", label: "1080p · 4 Mbps", height: 1080, video_mbps: 4 },
  { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
];

const makePrefs = (
  overrides: Partial<PlaybackSettingsType> = {},
): PlaybackSettingsType => ({
  profiles: PROFILES,
  preferred_profile: null,
  download_mbps: null,
  server_upload_mbps: null,
  hardware_acceleration_device: "cpu",
  is_admin: false,
  preferred_audio_language: null,
  preferred_subtitle_language: null,
  ...overrides,
});

describe("getDefaultPlaybackSettings", () => {
  it("falls back to availableModes[0] when no user prefs are passed", () => {
    expect(getDefaultPlaybackSettings(ALL_MODES).mode).toBe("direct");
  });

  it("uses preferred_profile when it is in availableModes", () => {
    const prefs = makePrefs({ preferred_profile: "1080p_8mbps" });
    expect(getDefaultPlaybackSettings(ALL_MODES, prefs).mode).toBe(
      "1080p_8mbps",
    );
  });

  it("ignores preferred_profile when it is not in availableModes", () => {
    const prefs = makePrefs({ preferred_profile: "2160p_16mbps" });
    expect(getDefaultPlaybackSettings(SUB_HD_MODES, prefs).mode).toBe("remux");
  });

  it("falls back when preferred_profile and download recommendation are unavailable", () => {
    const prefs = makePrefs({
      preferred_profile: "2160p_16mbps",
      download_mbps: 100,
    });
    expect(getDefaultPlaybackSettings(SUB_HD_MODES, prefs).mode).toBe("remux");
  });

  it("uses download_mbps recommendation when no preferred_profile is set", () => {
    const prefs = makePrefs({ download_mbps: 5 });
    expect(getDefaultPlaybackSettings(ALL_MODES, prefs).mode).toBe(
      "1080p_4mbps",
    );
  });

  it("clamps recommendation by server_upload_mbps", () => {
    const prefs = makePrefs({
      download_mbps: 100,
      server_upload_mbps: 5,
    });
    expect(getDefaultPlaybackSettings(ALL_MODES, prefs).mode).toBe(
      "1080p_4mbps",
    );
  });

  it("falls back to availableModes[0] when recommendation is not in availableModes", () => {
    const prefs = makePrefs({ download_mbps: 100 });
    expect(getDefaultPlaybackSettings(SUB_HD_MODES, prefs).mode).toBe("remux");
  });

  it("prefers preferred_profile over the download_mbps recommendation", () => {
    const prefs = makePrefs({
      preferred_profile: "720p_3mbps",
      download_mbps: 100,
    });
    expect(getDefaultPlaybackSettings(ALL_MODES, prefs).mode).toBe(
      "720p_3mbps",
    );
  });

  it("treats null user prefs the same as no prefs", () => {
    expect(getDefaultPlaybackSettings(ALL_MODES, null).mode).toBe("direct");
  });

  it("returns audioTrack 0 and subtitleTrack null", () => {
    const result = getDefaultPlaybackSettings(ALL_MODES);
    expect(result.audioTrack).toBe(0);
    expect(result.subtitleTrack).toBeNull();
  });

  it("picks the audio track matching preferred_audio_language", () => {
    const prefs = makePrefs({ preferred_audio_language: "en" });
    const audio = [
      audioStream(0, "spa"),
      audioStream(1, "eng"),
      audioStream(2, "fre"),
    ];
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, audio);
    expect(result.audioTrack).toBe(1);
  });

  it("falls back to audioTrack 0 when preferred_audio_language has no match", () => {
    const prefs = makePrefs({ preferred_audio_language: "de" });
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, audio);
    expect(result.audioTrack).toBe(0);
  });

  it("returns subtitleTrack null when preferred_subtitle_language is 'off'", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "off" });
    const subs = [subtitleStream(0, "eng"), subtitleStream(1, "spa")];
    const result = getDefaultPlaybackSettings(
      ALL_MODES,
      prefs,
      undefined,
      subs,
    );
    expect(result.subtitleTrack).toBeNull();
  });

  it("picks the subtitle track matching preferred_subtitle_language", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "es" });
    const subs = [
      subtitleStream(0, "eng"),
      subtitleStream(1, "spa"),
      subtitleStream(2, "fre"),
    ];
    const result = getDefaultPlaybackSettings(
      ALL_MODES,
      prefs,
      undefined,
      subs,
    );
    expect(result.subtitleTrack).toBe(1);
  });

  it("falls back to subtitleTrack null when preferred_subtitle_language has no match", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "de" });
    const subs = [subtitleStream(0, "eng"), subtitleStream(1, "spa")];
    const result = getDefaultPlaybackSettings(
      ALL_MODES,
      prefs,
      undefined,
      subs,
    );
    expect(result.subtitleTrack).toBeNull();
  });

  it("returns audioTrack 0 when preferred_audio_language is set but audioStreams is empty", () => {
    const prefs = makePrefs({ preferred_audio_language: "en" });
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, []);
    expect(result.audioTrack).toBe(0);
  });

  it("returns subtitleTrack null when preferred_subtitle_language is set but subtitleStreams is empty", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "es" });
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, undefined, []);
    expect(result.subtitleTrack).toBeNull();
  });

  it("does not match an unmapped 3-letter stream code against a 2-letter user pref", () => {
    const prefs = makePrefs({ preferred_audio_language: "fi" });
    const audio = [audioStream(0, "eng"), audioStream(1, "fil")];
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, audio);
    expect(result.audioTrack).toBe(0);
  });

  it("never auto-selects a bitmap subtitle for the preferred language", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "en" });
    const subtitles = [
      { ...subtitleStream(0, "eng"), codec: "hdmv_pgs_subtitle" },
      subtitleStream(1, "eng"),
    ];
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, [], subtitles);
    expect(result.subtitleTrack).toBe(1);
  });
});

describe("resolvePlaybackSettings", () => {
  it("falls back to the user's preferred profile when the mode is invalid", () => {
    const prefs = makePrefs({ preferred_profile: "1080p_8mbps" });
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: null },
      SUB_HD_MODES.concat([{ id: "1080p_8mbps" }]),
      [],
      [],
      prefs,
    );
    expect(result.mode).toBe("1080p_8mbps");
  });

  it("falls back to the preferred audio language when the track is out of range", () => {
    const prefs = makePrefs({ preferred_audio_language: "es" });
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 9, subtitleTrack: null },
      ALL_MODES,
      audio,
      [],
      prefs,
    );
    expect(result.audioTrack).toBe(1);
    expect(result.mode).toBe("remux");
  });

  it("keeps a valid explicit selection over user preferences", () => {
    const prefs = makePrefs({
      preferred_profile: "720p_3mbps",
      preferred_audio_language: "es",
    });
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: null },
      ALL_MODES,
      audio,
      [],
      prefs,
    );
    expect(result).toEqual({ mode: "direct", audioTrack: 0, subtitleTrack: null });
  });

  it("keeps an explicit subtitle-off selection over user preferences", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "es" });
    const subtitles = [
      subtitleStream(0, "eng"),
      subtitleStream(1, "spa"),
    ];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: null },
      ALL_MODES,
      [],
      subtitles,
      prefs,
    );
    expect(result.subtitleTrack).toBeNull();
  });

  it.each([
    ["an omitted selection", undefined],
    ["an out-of-range selection", 99],
  ])("uses the preferred text subtitle for %s", (_label, subtitleTrack) => {
    const prefs = makePrefs({ preferred_subtitle_language: "es" });
    const subtitles = [
      subtitleStream(0, "eng"),
      subtitleStream(1, "spa"),
    ];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack },
      ALL_MODES,
      [],
      subtitles,
      prefs,
    );
    expect(result.subtitleTrack).toBe(1);
  });

  it("rejects a subtitle selection that points at a bitmap track", () => {
    const subtitles = [
      { ...subtitleStream(0, "eng"), codec: "hdmv_pgs_subtitle" },
      subtitleStream(1, "spa"),
    ];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: 0 },
      ALL_MODES,
      [],
      subtitles,
    );
    expect(result.subtitleTrack).toBeNull();
  });

  it("falls back from a bitmap selection to the preferred text subtitle", () => {
    const prefs = makePrefs({ preferred_subtitle_language: "en" });
    const subtitles = [
      { ...subtitleStream(0, "eng"), codec: "hdmv_pgs_subtitle" },
      subtitleStream(1, "eng"),
    ];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: 0 },
      ALL_MODES,
      [],
      subtitles,
      prefs,
    );
    expect(result.subtitleTrack).toBe(1);
  });

  it("keeps a text subtitle selection", () => {
    const subtitles = [
      { ...subtitleStream(0, "eng"), codec: "hdmv_pgs_subtitle" },
      subtitleStream(1, "spa"),
    ];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: 1 },
      ALL_MODES,
      [],
      subtitles,
    );
    expect(result.subtitleTrack).toBe(1);
  });

  it("upgrades direct play to remux when a non-first audio track is selected", () => {
    const audio = [audioStream(0, "eng"), audioStream(1, "fra"), audioStream(2, "spa")];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 2, subtitleTrack: null },
      ALL_MODES,
      audio,
      [],
    );
    expect(result).toEqual({ mode: "remux", audioTrack: 2, subtitleTrack: null });
  });

  it("keeps direct play for the first audio track", () => {
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = resolvePlaybackSettings(
      { mode: "direct", audioTrack: 0, subtitleTrack: null },
      ALL_MODES,
      audio,
      [],
    );
    expect(result.mode).toBe("direct");
  });

  it("leaves non-direct modes untouched for a non-first audio track", () => {
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = resolvePlaybackSettings(
      { mode: "720p_3mbps", audioTrack: 1, subtitleTrack: null },
      ALL_MODES,
      audio,
      [],
    );
    expect(result).toEqual({ mode: "720p_3mbps", audioTrack: 1, subtitleTrack: null });
  });

  it("upgrades a preferred-language default away from direct play", () => {
    const prefs = makePrefs({ preferred_audio_language: "es" });
    const audio = [audioStream(0, "eng"), audioStream(1, "spa")];
    const result = getDefaultPlaybackSettings(ALL_MODES, prefs, audio, []);
    expect(result.mode).toBe("remux");
    expect(result.audioTrack).toBe(1);
  });
});

describe("audio track and mode resolvers", () => {
  it("upgrades direct to remux only for a non-first track", () => {
    expect(resolveModeForAudioTrack("direct", 0)).toBe("direct");
    expect(resolveModeForAudioTrack("direct", 1)).toBe("remux");
    expect(resolveModeForAudioTrack("remux", 3)).toBe("remux");
    expect(resolveModeForAudioTrack("720p_3mbps", 3)).toBe("720p_3mbps");
  });

  it("snaps the audio track back to the first stream for direct play", () => {
    expect(resolveAudioTrackForMode("direct", 3)).toBe(0);
    expect(resolveAudioTrackForMode("direct", 0)).toBe(0);
    expect(resolveAudioTrackForMode("remux", 3)).toBe(3);
    expect(resolveAudioTrackForMode("1080p_8mbps", 3)).toBe(3);
  });

  // resolveModeForAudioTrack upgrades to remux without checking availability,
  // which is only safe while remux is offered wherever direct is.
  it("never offers direct play without remux", () => {
    const nullString = { String: "", Valid: false };
    const nullInt = { Int64: 0, Valid: false };
    const videoCodecs = ["h264", "avc1", "hevc", "vp9", undefined];
    const audioCodecs = ["aac", "ac3", "dts", "flac", undefined];
    const containers = ["video/mp4", "video/webm", "video/x-matroska", ""];
    const heights = [0, 480, 1080, 2160];

    for (const videoCodec of videoCodecs) {
      for (const audioCodec of audioCodecs) {
        for (const mimeType of containers) {
          for (const height of heights) {
            const modes = getAvailableModes({
              video:
                videoCodec === undefined
                  ? undefined
                  : {
                      codec: videoCodec,
                      codec_profile: nullString,
                      codec_level: nullInt,
                      height,
                      bit_depth: nullInt,
                      pixel_format: nullString,
                    },
              audioStreams:
                audioCodec === undefined
                  ? []
                  : [
                      {
                        codec: audioCodec,
                        codec_profile: nullString,
                        is_default: false,
                      },
                    ],
              mimeType,
            });
            const ids = modes.map(m => m.id);
            if (ids.includes("direct")) {
              expect(ids).toContain("remux");
            }
          }
        }
      }
    }
  });
});
