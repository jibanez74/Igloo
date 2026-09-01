import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { renderWithQueryClient } from "@/test/helpers/render";
import { convertToAudioTrack } from "@/lib/audio-utils";
import type { PlayableTrackData } from "@/types";
import { stubMediaElement } from "../helpers/dom";

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
}));

function rawTrack({
  id,
  title,
  albumTitle,
  musician,
  cover,
}: {
  id: number;
  title: string;
  albumTitle: string | null;
  musician: string;
  cover: string | null;
}): PlayableTrackData {
  return {
    id,
    title,
    file_path: `/music/${id}.flac`,
    duration: 100,
    codec: "flac",
    bit_rate: 900000,
    album_id: { Int64: id, Valid: true },
    musician_id: { Int64: id, Valid: true },
    album_cover: cover
      ? { String: cover, Valid: true }
      : { String: "", Valid: false },
    musician_name: { String: musician, Valid: true },
    album_title:
      albumTitle !== null
        ? { String: albumTitle, Valid: true }
        : { String: "", Valid: false },
  };
}

function QueueHarness({
  rawTracks,
  startTrackId,
}: {
  rawTracks: PlayableTrackData[];
  startTrackId: number;
}) {
  const actions = useAudioPlayerActions();

  return (
    <button
      type="button"
      onClick={() => actions.playTrackFromList(rawTracks, startTrackId)}
    >
      start queue
    </button>
  );
}

function renderQueue(rawTracks: PlayableTrackData[], startTrackId: number) {
  renderWithQueryClient(
    <AudioPlayerProvider>
      <QueueHarness rawTracks={rawTracks} startTrackId={startTrackId} />
    </AudioPlayerProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "start queue" }));
}

describe("playTrackFromList", () => {
  beforeEach(() => {
    stubMediaElement();
  });

  afterEach(() => {
    cleanup();
  });

  const alabaster = rawTrack({
    id: 1,
    title: "Alabaster",
    albumTitle: "Stone Record",
    musician: "The Band",
    cover: "/covers/stone.jpg",
  });
  const basalt = rawTrack({
    id: 2,
    title: "Basalt",
    albumTitle: "Dark Record",
    musician: "Other Band",
    cover: null,
  });
  const chalk = rawTrack({
    id: 3,
    title: "Chalk",
    albumTitle: "White Record",
    musician: "Third Band",
    cover: "/covers/white.jpg",
  });

  it("queues the deduped list starting at the chosen track", () => {
    // The duplicate Alabaster entry must be dropped from the queue.
    renderQueue([alabaster, basalt, chalk, alabaster], 2);

    expect(
      screen.getByRole("dialog", { name: "Now playing: Basalt by Other Band" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Track 2 of 3")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Previous track" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Next track" }),
    ).toBeInTheDocument();
  });

  it("resolves per-track cover, musician, and album title on navigation", () => {
    renderQueue([alabaster, basalt, chalk], 2);

    // Basalt has no cover and its own album metadata.
    expect(screen.getByText("Dark Record")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "No album cover available" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));

    expect(screen.getByText("White Record")).toBeInTheDocument();
    expect(screen.getByText("Third Band")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Album cover for White Record" }),
    ).toBeInTheDocument();

    // Going back must restore Basalt's null cover instead of keeping Chalk's
    // (a mapped-to-null cover is not the same as an unmapped track).
    fireEvent.click(screen.getByRole("button", { name: "Previous track" }));

    expect(screen.getByText("Dark Record")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "No album cover available" }),
    ).toBeInTheDocument();
  });

  it("clears the album title for a track without one instead of keeping the previous track's", () => {
    const drift = rawTrack({
      id: 4,
      title: "Drift",
      albumTitle: null,
      musician: "Loose Single",
      cover: null,
    });

    renderQueue([alabaster, drift], 1);

    expect(screen.getByText("Stone Record")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));

    expect(screen.queryByText("Stone Record")).not.toBeInTheDocument();
  });

  it("ignores a start track that is not in the list", () => {
    renderQueue([alabaster], 999);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  describe("clicking the current track", () => {
    function getAudio(): HTMLAudioElement {
      const audio = document.querySelector("audio");
      if (!audio) throw new Error("audio element not rendered");
      return audio;
    }

    function markPlaying(audio: HTMLAudioElement) {
      Object.defineProperty(audio, "paused", {
        get: () => false,
        configurable: true,
      });
      fireEvent(audio, new Event("play"));
    }

    it("pauses instead of rebuilding the queue and re-opening the player", () => {
      renderQueue([alabaster, basalt], 1);

      const audio = getAudio();
      markPlaying(audio);
      const loadCallsAfterStart = vi.mocked(HTMLMediaElement.prototype.load)
        .mock.calls.length;

      fireEvent.click(
        screen.getByRole("button", { name: "Minimize player (Escape)" }),
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "start queue" }));

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(
        vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length,
      ).toBe(loadCallsAfterStart);
    });

    it("resumes a paused current track without reloading it", () => {
      renderQueue([alabaster, basalt], 1);

      fireEvent.click(
        screen.getByRole("button", { name: "Minimize player (Escape)" }),
      );

      const loadCallsAfterStart = vi.mocked(HTMLMediaElement.prototype.load)
        .mock.calls.length;
      vi.mocked(HTMLMediaElement.prototype.play).mockClear();

      // jsdom's audio element reports paused by default.
      fireEvent.click(screen.getByRole("button", { name: "start queue" }));

      expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
      expect(
        vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length,
      ).toBe(loadCallsAfterStart);
    });

    // The header "Play all"/"Shuffle" buttons go through playQueue, which must
    // restart even when its first track is the one already playing.
    it("does not swallow playQueue when the first track is already current", () => {
      const tracks = [alabaster, basalt].map(convertToAudioTrack);

      function StartOverHarness() {
        const actions = useAudioPlayerActions();

        return (
          <>
            <button
              type="button"
              onClick={() =>
                actions.playTrack(tracks[0], tracks, {
                  cover: null,
                  title: "Stone Record",
                  musician: "The Band",
                })
              }
            >
              play track
            </button>
            <button
              type="button"
              onClick={() =>
                actions.playQueue(tracks, {
                  cover: null,
                  title: "Stone Record",
                  musician: "The Band",
                })
              }
            >
              play all
            </button>
          </>
        );
      }

      renderWithQueryClient(
        <AudioPlayerProvider>
          <StartOverHarness />
        </AudioPlayerProvider>,
      );

      fireEvent.click(screen.getByRole("button", { name: "play track" }));

      const audio = getAudio();
      markPlaying(audio);
      audio.currentTime = 42;
      const loadCallsAfterStart = vi.mocked(HTMLMediaElement.prototype.load)
        .mock.calls.length;

      fireEvent.click(
        screen.getByRole("button", { name: "Minimize player (Escape)" }),
      );
      fireEvent.click(screen.getByRole("button", { name: "play all" }));

      // Same track id means the stream URL is unchanged, so the queue restart
      // has to rewind and resume by hand; the fullscreen view re-opens and the
      // click is never read as a pause.
      expect(audio.currentTime).toBe(0);
      expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
      expect(
        vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length,
      ).toBe(loadCallsAfterStart);
    });

    it("toggles through the playTrack entry point as well", () => {
      const tracks = [alabaster, basalt].map(convertToAudioTrack);

      function PlayTrackHarness() {
        const actions = useAudioPlayerActions();

        return (
          <button
            type="button"
            onClick={() =>
              actions.playTrack(tracks[0], tracks, {
                cover: null,
                title: "Stone Record",
                musician: "The Band",
              })
            }
          >
            play track
          </button>
        );
      }

      renderWithQueryClient(
        <AudioPlayerProvider>
          <PlayTrackHarness />
        </AudioPlayerProvider>,
      );

      fireEvent.click(screen.getByRole("button", { name: "play track" }));

      const audio = getAudio();
      markPlaying(audio);
      const loadCallsAfterStart = vi.mocked(HTMLMediaElement.prototype.load)
        .mock.calls.length;

      fireEvent.click(
        screen.getByRole("button", { name: "Minimize player (Escape)" }),
      );

      fireEvent.click(screen.getByRole("button", { name: "play track" }));

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(
        vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length,
      ).toBe(loadCallsAfterStart);
    });
  });
});

describe("shuffleQueue", () => {
  beforeEach(() => {
    stubMediaElement();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  const alabaster = rawTrack({
    id: 1,
    title: "Alabaster",
    albumTitle: "Stone Record",
    musician: "The Band",
    cover: "/covers/stone.jpg",
  });
  const basalt = rawTrack({
    id: 2,
    title: "Basalt",
    albumTitle: "Dark Record",
    musician: "Other Band",
    cover: null,
  });
  const chalk = rawTrack({
    id: 3,
    title: "Chalk",
    albumTitle: "White Record",
    musician: "Third Band",
    cover: "/covers/white.jpg",
  });

  const playlistInfo = {
    cover: "/covers/playlist.jpg",
    title: "Road Trip",
    musician: null,
  };

  function ShuffleHarness({
    rawTracks,
    withMetadata,
  }: {
    rawTracks: PlayableTrackData[];
    withMetadata: boolean;
  }) {
    const actions = useAudioPlayerActions();

    return (
      <button
        type="button"
        onClick={() =>
          actions.shuffleQueue(
            rawTracks.map(convertToAudioTrack),
            playlistInfo,
            withMetadata ? rawTracks : undefined,
          )
        }
      >
        shuffle
      </button>
    );
  }

  function renderShuffle(
    rawTracks: PlayableTrackData[],
    { withMetadata }: { withMetadata: boolean },
  ) {
    renderWithQueryClient(
      <AudioPlayerProvider>
        <ShuffleHarness rawTracks={rawTracks} withMetadata={withMetadata} />
      </AudioPlayerProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "shuffle" }));
  }

  // Pinning Math.random to 0 makes Fisher-Yates a left rotation by one, so the
  // queue order is fully determined: [A, B, C] becomes [B, C, A]. That is the
  // whole point of the button - it must not start on the list's first track.
  it("starts on the shuffled first track, not the list's", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);

    renderShuffle([alabaster, basalt, chalk], { withMetadata: true });

    expect(
      screen.getByRole("dialog", { name: "Now playing: Basalt by Other Band" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Track 1 of 3")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Chalk");

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Alabaster",
    );
  });

  // Regression guard: a shuffled playlist used to show the playlist's own name
  // in the artist slot and its cover for every track, because shuffleQueue had
  // no way to carry the per-track metadata the rows already held.
  it("resolves each track's own artist, album, and cover when given rawTracks", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);

    renderShuffle([alabaster, basalt, chalk], { withMetadata: true });

    expect(screen.getByText("Dark Record")).toBeInTheDocument();
    expect(screen.queryByText("Road Trip")).not.toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "No album cover available" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));

    expect(screen.getByText("White Record")).toBeInTheDocument();
    expect(screen.getByText("Third Band")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Album cover for White Record" }),
    ).toBeInTheDocument();
  });

  // Album and musician queues are single-artist, so they omit rawTracks on
  // purpose and the queue-wide info has to carry across every track.
  it("keeps the queue-wide album info when rawTracks is omitted", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);

    renderShuffle([alabaster, basalt, chalk], { withMetadata: false });

    expect(screen.getAllByText("Road Trip").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Next track" }));

    expect(screen.getAllByText("Road Trip").length).toBeGreaterThan(0);
    expect(screen.queryByText("White Record")).not.toBeInTheDocument();
  });

  it("does nothing for an empty queue", () => {
    renderShuffle([], { withMetadata: true });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
