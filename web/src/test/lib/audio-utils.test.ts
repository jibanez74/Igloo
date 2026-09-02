import { describe, expect, it, vi } from "vitest";
import {
  dedupeById,
  playMediaElement,
  shuffleArray,
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
  // The endless-queue contract: AudioPlayerProvider trims on every append, so
  // a queue played for hours stays bounded no matter how many batches arrive.
  it("keeps an endlessly appended queue bounded", () => {
    const KEEP_BEHIND = 50;
    const BATCH = 10;
    let tracks = makeTracks(BATCH);
    let nextId = BATCH + 1;
    let trimmed = 0;

    // Walk 500 tracks forward, appending a batch whenever the lookahead runs
    // low, exactly as the provider's append effect does.
    for (let position = 0; position < 500; position++) {
      const currentId = tracks[Math.min(position - trimmed, tracks.length - 1)].id;

      if (tracks.length - tracks.findIndex(t => t.id === currentId) < 5) {
        const result = trimQueueHistory(tracks, currentId, KEEP_BEHIND);
        trimmed += result.dropped.length;
        tracks = [
          ...result.tracks,
          ...Array.from({ length: BATCH }, () => ({ id: nextId++ })),
        ];
      }
    }

    expect(tracks.length).toBeLessThanOrEqual(KEEP_BEHIND + BATCH + 5);
    expect(trimmed).toBeGreaterThan(0);
    // Nothing is lost from the "Track N of M" denominator.
    expect(trimmed + tracks.length).toBe(nextId - 1);
  });
});

describe("dedupeById", () => {
  it("keeps the first occurrence of each id and preserves order", () => {
    const items = [{ id: 3 }, { id: 1 }, { id: 3 }, { id: 2 }, { id: 1 }];

    expect(dedupeById(items).map(item => item.id)).toEqual([3, 1, 2]);
  });

  it("returns an equal list when there is nothing to drop", () => {
    const items = [{ id: 1 }, { id: 2 }];

    expect(dedupeById(items)).toEqual(items);
  });
});

describe("shuffleArray", () => {
  it("returns a permutation of the input", () => {
    const input = makeTracks(50);

    const shuffled = shuffleArray(input);

    expect(shuffled).toHaveLength(input.length);
    expect([...shuffled].sort((a, b) => a.id - b.id)).toEqual(input);
  });

  it("does not mutate the input", () => {
    const input = makeTracks(20);
    const snapshot = [...input];

    shuffleArray(input);

    expect(input).toEqual(snapshot);
  });

  it("handles empty and single-element arrays", () => {
    expect(shuffleArray([])).toEqual([]);
    expect(shuffleArray([{ id: 1 }])).toEqual([{ id: 1 }]);
  });

  // Fisher-Yates walks i from the end down to 1 and swaps with a random j in
  // [0, i]. Forcing j to its maximum makes every swap a no-op, which pins the
  // draw range: an off-by-one that let j exceed i would throw the ordering off.
  it("leaves the order untouched when every draw picks the highest index", () => {
    const randomSpy = vi
      .spyOn(Math, "random")
      .mockReturnValue(0.999999999999);
    const input = makeTracks(8);

    expect(shuffleArray(input)).toEqual(input);

    randomSpy.mockRestore();
  });

  // The mirror case: forcing j to 0 swaps each element into slot 0 in turn,
  // which rotates the array left by one. That only holds if j is drawn from
  // [0, i] inclusive of 0 and i counts down from length - 1.
  it("rotates left by one when every draw picks index zero", () => {
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(0);

    expect(shuffleArray(makeTracks(4)).map(track => track.id)).toEqual([
      2, 3, 4, 1,
    ]);

    randomSpy.mockRestore();
  });
});
