import { describe, expect, it } from "vitest";
import { buildSubtitleTrackInfo } from "@/lib/video-playback";
import { episodeMediaRef, movieMediaRef } from "@/lib/media-ref";
import { effectiveModeLabel, streamModeLabel } from "@/lib/playback";
import { subtitleStream } from "../helpers/tech-details";

describe("buildSubtitleTrackInfo", () => {
  it("builds episode subtitle URLs under the episode route", () => {
    const info = buildSubtitleTrackInfo({
      media: episodeMediaRef(9),
      resolvedSubtitleTrack: 1,
      techLoaded: true,
      subtitleStreams: [subtitleStream(), subtitleStream()],
      actualHlsStartSec: 30,
    });

    expect(info?.url).toBe("/api/shows/episodes/9/subtitles/1/web.vtt?start=30");
  });

  // Cues are extracted with absolute source timestamps while a rebased HLS
  // session's media timeline starts at zero, so the server needs the offset to
  // shift them or every subtitle is out by the session start (audit H4).
  it("passes the session start so the server can rebase the cues", () => {
    const info = buildSubtitleTrackInfo({
      media: movieMediaRef(7),
      resolvedSubtitleTrack: 0,
      techLoaded: true,
      subtitleStreams: [subtitleStream()],
      actualHlsStartSec: 590,
    });

    expect(info?.url).toBe("/api/movies/7/subtitles/0/web.vtt?start=590");
  });

  // The measured actual start is fractional (a keyframe timestamp such as
  // 591.174), and the server parses `start` as a float — flooring it would
  // misalign every cue by up to a second.
  it("preserves a fractional session start", () => {
    const info = buildSubtitleTrackInfo({
      media: movieMediaRef(7),
      resolvedSubtitleTrack: 0,
      techLoaded: true,
      subtitleStreams: [subtitleStream()],
      actualHlsStartSec: 591.174,
    });

    expect(info?.url).toBe("/api/movies/7/subtitles/0/web.vtt?start=591.174");
  });

  it("omits the offset when the session starts at zero", () => {
    const info = buildSubtitleTrackInfo({
      media: movieMediaRef(7),
      resolvedSubtitleTrack: 0,
      techLoaded: true,
      subtitleStreams: [subtitleStream()],
      actualHlsStartSec: 0,
    });

    expect(info?.url).toBe("/api/movies/7/subtitles/0/web.vtt");
  });

  it("omits the offset for direct play, which has no offset to apply", () => {
    const info = buildSubtitleTrackInfo({
      media: movieMediaRef(7),
      resolvedSubtitleTrack: 0,
      techLoaded: true,
      subtitleStreams: [subtitleStream()],
    });

    expect(info?.url).toBe("/api/movies/7/subtitles/0/web.vtt");
  });

  // A changed URL is what re-attaches the <track>, since VideoPlayer keys its
  // subtitle effect on the URL alone.
  it("changes the URL when the session rebases", () => {
    const args = {
      media: movieMediaRef(7),
      resolvedSubtitleTrack: 0,
      techLoaded: true,
      subtitleStreams: [subtitleStream()],
    };

    const first = buildSubtitleTrackInfo({
      ...args,
      actualHlsStartSec: 0,
    });
    const rebased = buildSubtitleTrackInfo({
      ...args,
      actualHlsStartSec: 900,
    });

    expect(first?.url).not.toBe(rebased?.url);
  });
});

describe("effectiveModeLabel", () => {
  it("names the profile that actually ran when remux fell back", () => {
    expect(effectiveModeLabel("remux", "2160p_16mbps")).toBe(
      "4K — highest quality — remux unavailable",
    );
  });

  it("keeps the requested label when the server ran what was asked", () => {
    expect(effectiveModeLabel("remux", "remux")).toBe(
      "Original video, adjusted audio",
    );
  });

  it("keeps the requested label before the server has reported", () => {
    expect(effectiveModeLabel("720p_3mbps", null)).toBe(
      "720p — lower bandwidth",
    );
  });

  it("ignores an unrecognized profile rather than showing a raw id", () => {
    expect(effectiveModeLabel("remux", "something_else")).toBe(
      "Original video, adjusted audio",
    );
  });
});

describe("streamModeLabel", () => {
  it("names a known stream mode", () => {
    expect(streamModeLabel("remux")).toBe("Original video, adjusted audio");
  });

  it("falls back to the raw id for a mode it does not list", () => {
    expect(streamModeLabel("something_else")).toBe("something_else");
  });
});
