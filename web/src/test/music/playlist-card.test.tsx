import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PlaylistCard from "@/components/music/PlaylistCard";
import type { PlaylistSummaryType } from "@/types";
import { nullableString } from "@/test/helpers/fixtures";
import { renderWithQueryClient } from "@/test/helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

function playlist(trackCount: number): PlaylistSummaryType {
  return {
    id: 7,
    user_id: 1,
    name: "Late Shift",
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    track_count: trackCount,
    total_duration: 214_000,
    is_owner: true,
    can_edit: true,
  };
}

describe("PlaylistCard", () => {
  // The visible line has always chosen the right form; its accessible name
  // said "1 tracks" until the two were built from one helper.
  it("says one track in both the visible line and the accessible name", () => {
    renderWithQueryClient(<PlaylistCard playlist={playlist(1)} />);

    expect(screen.getByText("1 track · 3m 34s")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Late Shift, 1 track, 3m 34s" }),
    ).toBeInTheDocument();
  });

  it("pluralizes both for several tracks", () => {
    renderWithQueryClient(<PlaylistCard playlist={playlist(12)} />);

    expect(screen.getByText("12 tracks · 3m 34s")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Late Shift, 12 tracks, 3m 34s" }),
    ).toBeInTheDocument();
  });
});
