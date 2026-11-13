import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AlbumCard from "@/components/music/AlbumCard";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { getAlbumDetails } from "@/lib/api";
import { stubMediaElement } from "../helpers/dom";
import { nullableString } from "../helpers/fixtures";
import { albumDetailsResponse, albumTrack, simpleAlbum } from "../helpers/music";
import { renderWithQueryClient } from "../helpers/render";

const showActionFailedMock = vi.fn();

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getAlbumDetails: vi.fn(),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
  recordPlayEvent: vi.fn(),
}));
vi.mock("@/lib/toast-helpers", async () => ({
  ...(await vi.importActual<typeof import("@/lib/toast-helpers")>(
    "@/lib/toast-helpers",
  )),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

// An album card's play button awaits the album's tracks before a queue exists,
// the same window #267 closed for Shuffle all and Play all. An answer that
// lands after the reader pressed Stop, signed out (which stops the player) or
// started something newer must not take the player over or toast.
describe("an album card's pending play", () => {
  const blueRecord = albumDetailsResponse(42, "Blue Record", "The Band", "Alabaster");
  const newerTrack = albumTrack(901, 9, "Newer Song", 1);

  // The card plus the two things a reader can do while its album loads.
  function Harness() {
    const actions = useAudioPlayerActions();

    return (
      <>
        <AlbumCard
          album={simpleAlbum({
            id: 42,
            title: "Blue Record",
            musician: nullableString("The Band"),
          })}
        />
        <button type="button" onClick={actions.stop}>
          stop
        </button>
        <button
          type="button"
          onClick={() =>
            actions.playQueue([newerTrack], {
              cover: null,
              title: "Newer Record",
              musician: "Other Band",
            })
          }
        >
          play newer
        </button>
      </>
    );
  }

  let resolveAlbum: (value: unknown) => void = () => {};

  beforeEach(() => {
    stubMediaElement();
    vi.mocked(getAlbumDetails).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveAlbum = resolve;
        }) as never,
    );
  });

  afterEach(() => {
    cleanup();
  });

  const startAlbum = async () => {
    renderWithQueryClient(
      <AudioPlayerProvider>
        <Harness />
      </AudioPlayerProvider>,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Play Blue Record by The Band" }),
    );
    await waitFor(() => expect(getAlbumDetails).toHaveBeenCalledTimes(1));
  };

  // Resolves the held album and waits for the card to finish with it: its
  // play button is no longer marked busy. Queried by label, not role, since an
  // open fullscreen player aria-hides the card.
  const releaseAlbum = async (response: unknown) => {
    await act(async () => {
      resolveAlbum(response);
    });
    await waitFor(() => {
      expect(
        document.querySelector('[aria-label="Play Blue Record by The Band"]'),
      ).toHaveAttribute("aria-disabled", "false");
    });
  };

  // Starting a queue opens the fullscreen player, a modal that aria-hides the
  // rest of the tree — including the harness — so press its buttons by text.
  const press = (name: string) => fireEvent.click(screen.getByText(name));

  const nowPlaying = () => screen.queryByRole("heading", { level: 1 });

  // The control for the tests below: a guard that dropped every answer would
  // otherwise pass them all.
  it("plays the album when nothing happened while it loaded", async () => {
    await startAlbum();
    await releaseAlbum({ error: false, data: blueRecord });

    expect(nowPlaying()).toHaveTextContent("Alabaster");
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  it.each([
    ["the album", { error: false, data: blueRecord }],
    ["a failure", { error: true, message: "album query failed", status: 500 }],
  ])("drops %s when it lands after Stop", async (_label, response) => {
    await startAlbum();
    press("stop");
    await releaseAlbum(response);

    expect(nowPlaying()).toBeNull();
    expect(document.querySelector("audio")).toBeNull();
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  it("keeps a queue started while the album loaded", async () => {
    await startAlbum();
    press("play newer");
    expect(nowPlaying()).toHaveTextContent("Newer Song");

    await releaseAlbum({ error: false, data: blueRecord });

    expect(nowPlaying()).toHaveTextContent("Newer Song");
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });
});
