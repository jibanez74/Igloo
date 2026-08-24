import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";
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
});
