import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import type { PlayableTrackData } from "@/types";

const originalLoad = HTMLMediaElement.prototype.load;
const originalPlay = HTMLMediaElement.prototype.play;
const originalPause = HTMLMediaElement.prototype.pause;

function rawTrack({
  id,
  title,
  albumTitle,
  musician,
  cover,
}: {
  id: number;
  title: string;
  albumTitle: string;
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
    album_title: { String: albumTitle, Valid: true },
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
  render(
    <AudioPlayerProvider>
      <QueueHarness rawTracks={rawTracks} startTrackId={startTrackId} />
    </AudioPlayerProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "start queue" }));
}

describe("playTrackFromList", () => {
  beforeEach(() => {
    HTMLMediaElement.prototype.load = vi.fn();
    HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined);
    HTMLMediaElement.prototype.pause = vi.fn();
  });

  afterEach(() => {
    cleanup();
    HTMLMediaElement.prototype.load = originalLoad;
    HTMLMediaElement.prototype.play = originalPlay;
    HTMLMediaElement.prototype.pause = originalPause;
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

  it("ignores a start track that is not in the list", () => {
    renderQueue([alabaster], 999);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
