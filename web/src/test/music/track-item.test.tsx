import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TrackActionsMenu from "@/components/music/TrackActionsMenu";
import TrackItem from "@/components/music/TrackItem";
import { toggleLikeTrack } from "@/lib/api";
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
  toggleLikeTrack: vi.fn(),
}));

describe("TrackItem motion", () => {
  it("uses shared motion contracts for row controls", () => {
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
    expect(screen.getByRole("button", { name: "Add Signal Fire to liked" }))
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
  it("toggles the cached liked ids through the shared mutation", async () => {
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
    queryClient.setQueryData([LIKED_TRACK_IDS_KEY], {
      error: false,
      data: { liked_track_ids: [] },
    });

    const likeButton = screen.getByRole("button", {
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
});
