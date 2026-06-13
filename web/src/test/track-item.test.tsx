import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TrackActionsMenu from "@/components/TrackActionsMenu";
import TrackItem from "@/components/TrackItem";
import {
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";
import { renderWithQueryClient } from "@/test/render";

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
