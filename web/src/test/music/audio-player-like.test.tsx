import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { getLikedTrackIds, toggleLikeTrack } from "@/lib/api";
import { showActionFailed } from "@/lib/toast-helpers";
import { renderWithQueryClient } from "@/test/helpers/render";
import type { PlayableTrackData } from "@/types";
import { stubMediaElement } from "../helpers/dom";

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn(),
  toggleLikeTrack: vi.fn(),
  recordPlayEvent: vi.fn().mockResolvedValue({ error: false, data: {} }),
}));

vi.mock("@/lib/toast-helpers", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/toast-helpers")>()),
  showActionFailed: vi.fn(),
}));

const TRACK_ID = 42;
const ADD_LABEL = "Add Alabaster to liked";
const REMOVE_LABEL = "Remove Alabaster from liked";

const rawTrack: PlayableTrackData = {
  id: TRACK_ID,
  title: "Alabaster",
  file_path: `/music/${TRACK_ID}.flac`,
  duration: 100,
  codec: "flac",
  bit_rate: 900000,
  album_id: { Int64: 7, Valid: true },
  musician_id: { Int64: 8, Valid: true },
  album_cover: { String: "", Valid: false },
  musician_name: { String: "The Band", Valid: true },
  album_title: { String: "Blue Record", Valid: true },
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function likedIds(ids: number[]) {
  return { error: false as const, data: { liked_track_ids: ids } };
}

function PlayerHarness() {
  const actions = useAudioPlayerActions();

  return (
    <>
      <button
        type="button"
        onClick={() => actions.playTrackFromList([rawTrack], TRACK_ID)}
      >
        start playback
      </button>
      <button type="button" onClick={actions.minimize}>
        minimize player
      </button>
    </>
  );
}

function renderPlayer() {
  const view = renderWithQueryClient(
    <AudioPlayerProvider>
      <PlayerHarness />
    </AudioPlayerProvider>,
  );

  fireEvent.click(screen.getByRole("button", { name: "start playback" }));
  return view;
}

describe("audio player like button", () => {
  beforeEach(() => {
    stubMediaElement();
  });

  it("disables the action until the liked state is known", async () => {
    const likedQuery = deferred<Awaited<ReturnType<typeof getLikedTrackIds>>>();
    vi.mocked(getLikedTrackIds).mockReturnValue(likedQuery.promise);
    renderPlayer();

    const loadingButton = await screen.findByRole("button", {
      name: "Loading liked status for Alabaster",
    });
    expect(loadingButton).toHaveAttribute("aria-disabled", "true");
    expect(loadingButton).not.toHaveAttribute("aria-pressed");
    fireEvent.click(loadingButton);
    expect(toggleLikeTrack).not.toHaveBeenCalled();

    likedQuery.resolve(likedIds([TRACK_ID]));
    const resolvedButton = await screen.findByRole("button", {
      name: REMOVE_LABEL,
    });
    expect(resolvedButton).not.toHaveAttribute("aria-disabled");
    expect(resolvedButton).toHaveAttribute("aria-pressed", "true");
  });

  it("shows an unliked heart in both layouts when the track is not liked", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    renderPlayer();

    // Playback opens the expanded dialog.
    const expandedButton = await screen.findByRole("button", {
      name: ADD_LABEL,
    });
    expect(expandedButton).toHaveAttribute("aria-pressed", "false");

    // The minimized bar renders its own like button.
    fireEvent.click(screen.getByText("minimize player"));
    const miniBar = screen.getByRole("region", { name: "Audio player" });
    expect(
      within(miniBar).getByRole("button", { name: ADD_LABEL }),
    ).toHaveAttribute("aria-pressed", "false");
  });

  it("shows a liked heart when the track is in the liked ids", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([TRACK_ID]));
    renderPlayer();

    const button = await screen.findByRole("button", { name: REMOVE_LABEL });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("optimistically flips the heart and calls the toggle endpoint", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    const toggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockReturnValue(toggle.promise);
    renderPlayer();

    fireEvent.click(await screen.findByRole("button", { name: ADD_LABEL }));

    // Flips before the request resolves.
    const button = await screen.findByRole("button", { name: REMOVE_LABEL });
    expect(button).toHaveAttribute("aria-pressed", "true");
    expect(toggleLikeTrack).toHaveBeenCalledWith(TRACK_ID);

    toggle.resolve({
      error: false,
      data: { track_id: TRACK_ID, is_liked: true },
    });
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: REMOVE_LABEL }),
      ).toHaveAttribute("aria-pressed", "true");
    });
  });

  it("rolls back the heart and shows a toast when the toggle fails", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    vi.mocked(toggleLikeTrack).mockRejectedValue(new Error("network down"));
    renderPlayer();

    fireEvent.click(await screen.findByRole("button", { name: ADD_LABEL }));

    await waitFor(() => {
      expect(showActionFailed).toHaveBeenCalledWith(
        "update like",
        "Unable to update liked status. Please try again.",
      );
    });
    expect(
      screen.getByRole("button", { name: ADD_LABEL }),
    ).toHaveAttribute("aria-pressed", "false");
  });

  it("shares pending state when the player layout changes", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    const toggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockReturnValue(toggle.promise);
    renderPlayer();

    fireEvent.click(await screen.findByRole("button", { name: ADD_LABEL }));
    await screen.findByRole("button", { name: REMOVE_LABEL });
    fireEvent.click(screen.getByText("minimize player"));

    const miniBar = screen.getByRole("region", { name: "Audio player" });
    const minimizedButton = within(miniBar).getByRole("button", {
      name: REMOVE_LABEL,
    });
    expect(minimizedButton).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(minimizedButton);

    expect(toggleLikeTrack).toHaveBeenCalledTimes(1);

    toggle.resolve({
      error: false,
      data: { track_id: TRACK_ID, is_liked: true },
    });
    await waitFor(() => {
      expect(
        within(miniBar).getByRole("button", { name: REMOVE_LABEL }),
      ).not.toHaveAttribute("aria-busy");
    });
  });
});
