import { useEffect } from "react";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { getShuffleTracks } from "@/lib/api";
import { renderWithQueryClient } from "@/test/helpers/render";
import type { TrackListItemType } from "@/types";

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

const originalLoad = HTMLMediaElement.prototype.load;
const originalPlay = HTMLMediaElement.prototype.play;
const originalPause = HTMLMediaElement.prototype.pause;

const BATCH_SIZE = 10;
// Mirrors MAX_TRACKS_BEHIND in AudioPlayerContext plus the shuffle lookahead
// threshold and one fetched batch — the ceiling a trimmed queue can reach.
const MAX_EXPECTED_QUEUE_SIZE = 65;

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

function EndlessQueueHarness() {
  const actions = useAudioPlayerActions();
  const state = useAudioPlayerState();

  return (
    <div>
      <button
        type="button"
        onClick={() => void actions.startShufflePlayback()}
      >
        start shuffle
      </button>
      <output aria-label="current track id">
        {state.currentTrack?.id ?? ""}
      </output>
      <output aria-label="queue size">{state.tracks.length}</output>
      <output aria-label="trimmed count">{state.trimmedCount}</output>
    </div>
  );
}

describe("endless shuffle queue", () => {
  beforeEach(() => {
    HTMLMediaElement.prototype.load = vi.fn();
    HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined);
    HTMLMediaElement.prototype.pause = vi.fn();

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
    HTMLMediaElement.prototype.load = originalLoad;
    HTMLMediaElement.prototype.play = originalPlay;
    HTMLMediaElement.prototype.pause = originalPause;
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
      await waitFor(() => {
        expect(screen.getByLabelText("current track id")).toHaveTextContent(
          "1",
        );
      });

      // Advance far enough past MAX_TRACKS_BEHIND that appends must trim
      // (with 10-track batches the first trimming append lands at index 56).
      // findByRole waits out in-flight fetches near the end of the queue.
      for (let targetId = 2; targetId <= 66; targetId++) {
        const nextButton = await screen.findByRole("button", {
          name: "Next track",
        });
        fireEvent.click(nextButton);

        await waitFor(() => {
          expect(screen.getByLabelText("current track id")).toHaveTextContent(
            String(targetId),
          );
        });
      }

      const trimmedCount = Number(
        screen.getByLabelText("trimmed count").textContent,
      );
      const queueSize = Number(
        screen.getByLabelText("queue size").textContent,
      );

      expect(trimmedCount).toBeGreaterThan(0);
      expect(queueSize).toBeLessThanOrEqual(MAX_EXPECTED_QUEUE_SIZE);

      // 65 next-clicks from track 1: the counter must read 66 with the
      // trimmed history added back in, never jumping backwards.
      expect(
        screen.getByText(
          (_, element) =>
            element?.tagName === "P" &&
            element.textContent === `Track 66 of ${trimmedCount + queueSize}`,
        ),
      ).toBeInTheDocument();
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
    await waitFor(() => {
      expect(screen.getByLabelText("current track id")).toHaveTextContent("1");
    });

    // Advance into the lookahead threshold so the next batch fetch starts.
    for (let targetId = 2; targetId <= 7; targetId++) {
      fireEvent.click(
        await screen.findByRole("button", { name: "Next track" }),
      );
      await waitFor(() => {
        expect(screen.getByLabelText("current track id")).toHaveTextContent(
          String(targetId),
        );
      });
    }
    await waitFor(() => expect(pendingBatches.length).toBeGreaterThan(0));

    expect(screen.getByLabelText("probe current")).toHaveTextContent("true");
    const rendersBeforeAppend = probeRenders.count;

    pendingBatches.shift()?.();
    await waitFor(() => {
      expect(screen.getByLabelText("queue size")).toHaveTextContent("20");
    });

    expect(probeRenders.count).toBe(rendersBeforeAppend);
  });
});
