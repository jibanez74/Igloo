import { describe, it, expect } from "vitest";
import { getDefaultPlaybackSettings } from "@/lib/playback";
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
});
