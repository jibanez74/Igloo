import { describe, expect, it } from "vitest";
import { trackRowProps } from "@/lib/track-row-props";
import type { PlayableTrackData } from "@/types";

function track(overrides: Partial<PlayableTrackData> = {}): PlayableTrackData {
  return {
    id: 4,
    title: "Driftwood",
    duration: 214_000,
    codec: "flac",
    bit_rate: 960_000,
    album_id: { Int64: 8, Valid: true },
    musician_id: { Int64: 20, Valid: true },
    album_cover: { String: "cover.jpg", Valid: true },
    musician_name: { String: "The Band", Valid: true },
    ...overrides,
  };
}

describe("trackRowProps", () => {
  it("unwraps the nullable album, musician and artist name", () => {
    expect(trackRowProps(track())).toEqual({
      id: 4,
      title: "Driftwood",
      duration: 214_000,
      subtitle: "The Band",
      albumId: 8,
      musicianId: 20,
    });
  });

  it("falls back to Unknown Artist when the name is absent", () => {
    const props = trackRowProps(
      track({ musician_name: { String: "", Valid: false } }),
    );

    expect(props.subtitle).toBe("Unknown Artist");
  });

  it("leaves an absent album or musician null rather than zero", () => {
    const props = trackRowProps(
      track({
        album_id: { Int64: 0, Valid: false },
        musician_id: { Int64: 0, Valid: false },
      }),
    );

    expect(props.albumId).toBeNull();
    expect(props.musicianId).toBeNull();
  });

  // The track list carries only the fields TrackItem needs; the queue reads
  // the row itself, so nothing else should ride along.
  it("passes on only the six props every list shares", () => {
    expect(Object.keys(trackRowProps(track())).sort()).toEqual([
      "albumId",
      "duration",
      "id",
      "musicianId",
      "subtitle",
      "title",
    ]);
  });
});
