import { describe, expect, it, vi } from "vitest";
import {
  playMediaElement,
  toggleMediaPlayback,
  trimQueueHistory,
} from "@/lib/audio-utils";

function makeTracks(count: number) {
  return Array.from({ length: count }, (_, index) => ({ id: index + 1 }));
}

describe("playMediaElement", () => {
  it("swallows the rejection when the browser blocks playback", async () => {
    const media = {
      play: vi.fn().mockRejectedValue(new Error("NotAllowedError")),
    } as unknown as HTMLMediaElement;

    await expect(playMediaElement(media)).resolves.toBeUndefined();
    expect(media.play).toHaveBeenCalledOnce();
  });

  it("ignores a missing element", async () => {
    await expect(playMediaElement(null)).resolves.toBeUndefined();
  });
});

describe("toggleMediaPlayback", () => {
  it("plays when paused and pauses when playing", () => {
    const media = {
      paused: true,
      play: vi.fn().mockResolvedValue(undefined),
      pause: vi.fn(),
    } as unknown as HTMLMediaElement;

    toggleMediaPlayback(media);
    expect(media.play).toHaveBeenCalledOnce();

    (media as unknown as { paused: boolean }).paused = false;
    toggleMediaPlayback(media);
    expect(media.pause).toHaveBeenCalledOnce();
  });
});

describe("trimQueueHistory", () => {
  it("returns the queue unchanged while the current track is within the window", () => {
    const tracks = makeTracks(10);

    const result = trimQueueHistory(tracks, 4, 5);

    expect(result.tracks).toBe(tracks);
    expect(result.dropped).toEqual([]);
  });

  it("returns the queue unchanged when there is no current track", () => {
    const tracks = makeTracks(10);

    expect(trimQueueHistory(tracks, null, 2).tracks).toBe(tracks);
    expect(trimQueueHistory(tracks, 999, 2).tracks).toBe(tracks);
  });

  it("drops tracks beyond the window and reports them in order", () => {
    const tracks = makeTracks(10);

    // Current track id 8 sits at index 7; keeping 3 behind drops indexes 0-3.
    const result = trimQueueHistory(tracks, 8, 3);

    expect(result.dropped.map(track => track.id)).toEqual([1, 2, 3, 4]);
    expect(result.tracks.map(track => track.id)).toEqual([
      5, 6, 7, 8, 9, 10,
    ]);
  });

  it("keeps exactly keepBehind tracks before the current one", () => {
    const tracks = makeTracks(100);

    const result = trimQueueHistory(tracks, 80, 50);

    expect(result.tracks[0].id).toBe(30);
    expect(result.tracks.indexOf(tracks[79])).toBe(50);
    expect(result.dropped).toHaveLength(29);
  });
});
