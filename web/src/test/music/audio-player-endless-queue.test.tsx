import { useEffect } from "react";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { getShuffleTracks } from "@/lib/api";
import { renderWithQueryClient } from "@/test/helpers/render";
import type { TrackListItemType } from "@/types";
import { stubMediaElement } from "../helpers/dom";

vi.mock("@/lib/api", () => ({
  getShuffleTracks: vi.fn(),
  getTracksPaginated: vi.fn(),
  recordPlayEvent: vi.fn(),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
  toggleLikeTrack: vi.fn(),
}));

const BATCH_SIZE = 10;

function rawTrack(id: number): TrackListItemType {
  return {
    id,
    title: `Song ${id}`,
    duration: 100,
    codec: "flac",
    bit_rate: 900000,
    file_path: `/music/${id}.flac`,
    album_id: { Int64: id, Valid: true },
    album_title: { String: "", Valid: false },
    album_cover: { String: "", Valid: false },
    musician_id: { Int64: id, Valid: true },
    musician_name: { String: "The Band", Valid: true },
  };
}

// The queue lives in provider state and is never published as a context, so
// these assertions read the player's own UI: the "Track N of M" counter and
// the track title heading.
function trackCounter() {
  return screen.getByText(/^Track \d+ of \d+$/).textContent ?? "";
}

function expectCounterAt(position: number) {
  expect(trackCounter()).toMatch(new RegExp(`^Track ${position} of \\d+$`));
}

function EndlessQueueHarness() {
  const actions = useAudioPlayerActions();

  return (
    <button type="button" onClick={() => void actions.startShufflePlayback()}>
      start shuffle
    </button>
  );
}

describe("endless shuffle queue", () => {
  beforeEach(() => {
    stubMediaElement();

    let nextId = 1;
    vi.mocked(getShuffleTracks).mockImplementation(async () => ({
      error: false as const,
      data: {
        tracks: Array.from({ length: BATCH_SIZE }, () => rawTrack(nextId++)),
      },
    }));
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it(
    "keeps the queue bounded and the track counter monotonic across trims",
    async () => {
      renderWithQueryClient(
        <AudioPlayerProvider>
          <EndlessQueueHarness />
        </AudioPlayerProvider>,
      );

      fireEvent.click(screen.getByRole("button", { name: "start shuffle" }));
      await waitFor(() => expectCounterAt(1));

      // Advance far enough past MAX_TRACKS_BEHIND that appends must trim
      // (with 10-track batches the first trimming append lands at index 56).
      // findByRole waits out in-flight fetches near the end of the queue.
      // Asserting the counter on every step is the monotonicity check: without
      // trimmedCount added back in, it would jump backwards at the first trim.
      for (let targetId = 2; targetId <= 66; targetId++) {
        const nextButton = await screen.findByRole("button", {
          name: "Next track",
        });
        fireEvent.click(nextButton);

        await waitFor(() => expectCounterAt(targetId));
      }

      expect(
        screen.getByRole("heading", { level: 1 }),
      ).toHaveTextContent("Song 66");

      // Trimming must not lose tracks from the counter's denominator either:
      // every track ever fetched stays accounted for.
      const total = Number(trackCounter().match(/of (\d+)$/)?.[1]);
      expect(total).toBe(vi.mocked(getShuffleTracks).mock.calls.length * BATCH_SIZE);
    },
    15000,
  );

  // Regression guard for the now-playing context split: queue appends must not
  // re-render matcher subscribers (track rows). Also makes a future React
  // Compiler bail-out in AudioPlayerProvider observable — if the provider stops
  // memoizing the now-playing value, this starts failing.
  it("does not re-render matcher subscribers when a batch is appended without a track change", async () => {
    const pendingBatches: Array<() => void> = [];
    let fetchCalls = 0;
    let nextId = 1;
    const makeBatch = () => ({
      error: false as const,
      data: {
        tracks: Array.from({ length: BATCH_SIZE }, () => rawTrack(nextId++)),
      },
    });
    vi.mocked(getShuffleTracks).mockImplementation(() => {
      fetchCalls++;
      if (fetchCalls === 1) return Promise.resolve(makeBatch());
      // Hold lookahead batches until the test releases them, so the append
      // can be observed in isolation from track changes.
      return new Promise(resolve => {
        pendingBatches.push(() => resolve(makeBatch()));
      });
    });

    const probeRenders = { count: 0 };
    function MatcherProbe() {
      const matchTrackPlayback = useTrackPlaybackMatcher();
      const { isCurrentTrack } = matchTrackPlayback(7);
      useEffect(() => {
        probeRenders.count++;
      });
      return (
        <output aria-label="probe current">{String(isCurrentTrack)}</output>
      );
    }

    renderWithQueryClient(
      <AudioPlayerProvider>
        <EndlessQueueHarness />
        <MatcherProbe />
      </AudioPlayerProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "start shuffle" }));
    await waitFor(() => expectCounterAt(1));

    // Advance into the lookahead threshold so the next batch fetch starts.
    for (let targetId = 2; targetId <= 7; targetId++) {
      fireEvent.click(
        await screen.findByRole("button", { name: "Next track" }),
      );
      await waitFor(() => expectCounterAt(targetId));
    }
    await waitFor(() => expect(pendingBatches.length).toBeGreaterThan(0));

    expect(screen.getByLabelText("probe current")).toHaveTextContent("true");
    const rendersBeforeAppend = probeRenders.count;

    pendingBatches.shift()?.();
    // The appended batch shows up as a bigger denominator; nothing trims this
    // early, so 10 queued tracks become 20.
    await waitFor(() => {
      expect(trackCounter()).toBe("Track 7 of 20");
    });

    expect(probeRenders.count).toBe(rendersBeforeAppend);
  });
});

describe("endless shuffle queue on an exhausted library", () => {
  beforeEach(() => {
    stubMediaElement();
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  // Stand in for the server: honour the exclusion list the client sends, which
  // is the whole point of the parameter. A library this small used to hand the
  // same ids back on every refill, so the client burned three round trips per
  // track advance and then went quiet.
  function mockLibrary(size: number) {
    vi.mocked(getShuffleTracks).mockImplementation(
      async (limit = 50, excludeTrackIds: number[] = []) => {
        const excluded = new Set(excludeTrackIds);
        const available = Array.from({ length: size }, (_, i) => i + 1)
          .filter(id => !excluded.has(id))
          .slice(0, limit);

        return {
          error: false as const,
          data: { tracks: available.map(rawTrack) },
        };
      },
    );
  }

  it("sends the queued track ids so a refill never repeats them", async () => {
    mockLibrary(24);

    renderWithQueryClient(
      <AudioPlayerProvider>
        <EndlessQueueHarness />
      </AudioPlayerProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "start shuffle" }));
    await waitFor(() => expectCounterAt(1));

    // The opening fetch has nothing to exclude yet.
    expect(vi.mocked(getShuffleTracks).mock.calls[0][1]).toBeUndefined();

    // Walk into the lookahead window so a refill fires.
    for (let position = 2; position <= 16; position++) {
      fireEvent.click(
        await screen.findByRole("button", { name: "Next track" }),
      );
      await waitFor(() => expectCounterAt(position));
    }

    await waitFor(() =>
      expect(vi.mocked(getShuffleTracks).mock.calls.length).toBeGreaterThan(1),
    );

    const refillExclusions = vi.mocked(getShuffleTracks).mock.calls[1][1] ?? [];
    expect(refillExclusions.length).toBeGreaterThan(0);

    // Every id the queue already holds is excluded, so nothing can come back
    // twice and the "Track N of M" denominator only ever counts fresh tracks.
    const total = Number(trackCounter().match(/of (\d+)$/)?.[1]);
    expect(total).toBeLessThanOrEqual(24);
  });

  it("says so when the library has nothing left instead of going quiet", async () => {
    const { toast } = await import("sonner");
    const infoSpy = vi.spyOn(toast, "info");

    mockLibrary(12);

    renderWithQueryClient(
      <AudioPlayerProvider>
        <EndlessQueueHarness />
      </AudioPlayerProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "start shuffle" }));
    await waitFor(() => expectCounterAt(1));

    // 12 tracks queued, lookahead is 10, so the refill fires almost at once and
    // the server has nothing left that the queue does not already hold.
    for (let position = 2; position <= 4; position++) {
      fireEvent.click(
        await screen.findByRole("button", { name: "Next track" }),
      );
      await waitFor(() => expectCounterAt(position));
    }

    await waitFor(() => expect(infoSpy).toHaveBeenCalled());
    expect(infoSpy.mock.calls[0][0]).toBe("That's the whole library");

    infoSpy.mockRestore();
  });
});
