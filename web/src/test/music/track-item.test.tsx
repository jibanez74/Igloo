import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TrackActionsMenu from "@/components/music/TrackActionsMenu";
import TrackItem from "@/components/music/TrackItem";
import { getLikedTrackIds, toggleLikeTrack } from "@/lib/api";
import {
  LIKED_TRACK_IDS_KEY,
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";
import { renderWithQueryClient } from "@/test/helpers/render";

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
  toggleLikeTrack: vi.fn(),
}));

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

describe("TrackItem motion", () => {
  it("uses shared motion contracts for row controls", async () => {
    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        trackIndex={1}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
        isDraggable
      />,
    );

    const row = screen.getByText("Signal Fire").closest(".group");
    expect(row).toHaveClass(...MOTION_TRACK_ROW_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Drag to reorder" })).toHaveClass(
      ...MOTION_TRACK_MENU_TRIGGER_CLASS.split(" "),
    );
    expect(await screen.findByRole("button", { name: "Add Signal Fire to liked" }))
      .toHaveClass(...MOTION_TRACK_ICON_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Play Signal Fire" })).toHaveClass(
      ...MOTION_TRACK_PLAY_BUTTON_CLASS.split(" "),
    );
  });

  it("uses the shared menu trigger contract", () => {
    renderWithQueryClient(
      <TrackActionsMenu trackId={7} trackTitle="Signal Fire" />,
    );

    expect(
      screen.getByRole("button", { name: "More actions for Signal Fire" }),
    ).toHaveClass(...MOTION_TRACK_MENU_TRIGGER_CLASS.split(" "));
  });
});

describe("TrackItem like button", () => {
  it("stays focusable while liked status loads", async () => {
    const status = deferred<Awaited<ReturnType<typeof getLikedTrackIds>>>();
    vi.mocked(getLikedTrackIds).mockReturnValue(status.promise);

    renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    const loadingButton = screen.getByRole("button", {
      name: "Loading liked status for Signal Fire",
    });
    expect(loadingButton).toHaveAttribute("aria-disabled", "true");
    expect(loadingButton).not.toBeDisabled();
    loadingButton.focus();
    expect(loadingButton).toHaveFocus();
    fireEvent.click(loadingButton);
    expect(toggleLikeTrack).not.toHaveBeenCalled();

    status.resolve(likedIds([]));
    const readyButton = await screen.findByRole("button", {
      name: "Add Signal Fire to liked",
    });
    expect(readyButton).not.toHaveAttribute("aria-disabled");
  });

  it("toggles the cached liked ids through the shared mutation", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    vi.mocked(toggleLikeTrack).mockResolvedValue({
      error: false,
      data: { track_id: 7, is_liked: true },
    });

    const { queryClient } = renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );
    const likeButton = await screen.findByRole("button", {
      name: "Add Signal Fire to liked",
    });
    expect(likeButton).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(likeButton);

    await waitFor(() => {
      expect(toggleLikeTrack).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual({
        error: false,
        data: { liked_track_ids: [7] },
      });
    });
  });

  it("rolls back only the failed track when another toggle succeeds", async () => {
    vi.mocked(getLikedTrackIds).mockResolvedValue(likedIds([]));
    const firstToggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    const secondToggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockImplementation(trackId =>
      trackId === 7 ? firstToggle.promise : secondToggle.promise,
    );

    const { queryClient } = renderWithQueryClient(
      <>
        <TrackItem
          id={7}
          title="Signal Fire"
          duration={211}
          variant="album"
          onPlay={vi.fn()}
          showActionsMenu={false}
        />
        <TrackItem
          id={8}
          title="Northern Sky"
          duration={226}
          variant="album"
          onPlay={vi.fn()}
          showActionsMenu={false}
        />
      </>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Add Signal Fire to liked" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Add Northern Sky to liked" }),
    );

    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7, 8]),
      );
    });

    secondToggle.resolve({
      error: false,
      data: { track_id: 8, is_liked: true },
    });
    await waitFor(() => {
      expect(toggleLikeTrack).toHaveBeenCalledTimes(2);
    });
    firstToggle.reject(new Error("network down"));

    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([8]),
      );
    });
  });

  it("refetches canonical ids when the cache is invalid at success", async () => {
    vi.mocked(getLikedTrackIds)
      .mockResolvedValueOnce(likedIds([]))
      .mockResolvedValueOnce(likedIds([7]));
    const toggle = deferred<Awaited<ReturnType<typeof toggleLikeTrack>>>();
    vi.mocked(toggleLikeTrack).mockReturnValue(toggle.promise);

    const { queryClient } = renderWithQueryClient(
      <TrackItem
        id={7}
        title="Signal Fire"
        duration={211}
        variant="album"
        onPlay={vi.fn()}
        showActionsMenu={false}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Add Signal Fire to liked" }),
    );
    await waitFor(() => {
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7]),
      );
    });

    queryClient.setQueryData([LIKED_TRACK_IDS_KEY], {
      error: true,
      message: "stale cache",
    });
    toggle.resolve({
      error: false,
      data: { track_id: 7, is_liked: true },
    });

    await waitFor(() => {
      expect(getLikedTrackIds).toHaveBeenCalledTimes(2);
      expect(queryClient.getQueryData([LIKED_TRACK_IDS_KEY])).toEqual(
        likedIds([7]),
      );
    });
  });
});
