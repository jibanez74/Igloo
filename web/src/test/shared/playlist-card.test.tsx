import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PlaylistCard from "@/components/shared/PlaylistCard";
import type { MoviePlaylistSummaryType, PlaylistSummaryType } from "@/types";
import { nullableInt64, nullableString } from "@/test/helpers/fixtures";
import { renderWithQueryClient } from "@/test/helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

const playlistRow = {
  user_id: 1,
  description: nullableString(),
  cover_image: nullableString(),
  is_public: false,
  movie_id: nullableInt64(),
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  is_owner: true,
  can_edit: true,
};

function musicPlaylist(
  fields: Partial<PlaylistSummaryType> = {},
): PlaylistSummaryType {
  return {
    ...playlistRow,
    id: 7,
    name: "Late Shift",
    content_type: "track",
    track_count: 1,
    total_duration: 214_000,
    ...fields,
  };
}

function moviePlaylist(
  fields: Partial<MoviePlaylistSummaryType> = {},
): MoviePlaylistSummaryType {
  return {
    ...playlistRow,
    id: 9,
    name: "Friday Feature",
    content_type: "movie",
    movie_count: 7,
    ...fields,
  };
}

describe("PlaylistCard", () => {
  // The visible line has always chosen the right form; its accessible name
  // said "1 tracks" until the two were built from one helper.
  it("says one track in both the visible line and the accessible name", () => {
    renderWithQueryClient(<PlaylistCard playlist={musicPlaylist()} />);

    expect(screen.getByText("1 track · 3m 34s")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Late Shift, 1 track, 3m 34s" }),
    ).toHaveAttribute("href", "/music/playlist/7");
  });

  it("pluralizes both for several tracks", () => {
    renderWithQueryClient(
      <PlaylistCard playlist={musicPlaylist({ track_count: 12 })} />,
    );

    expect(screen.getByText("12 tracks · 3m 34s")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Late Shift, 12 tracks, 3m 34s" }),
    ).toBeInTheDocument();
  });

  it("leaves a zero duration out of both the line and the name", () => {
    renderWithQueryClient(
      <PlaylistCard
        playlist={musicPlaylist({ track_count: 0, total_duration: 0 })}
      />,
    );

    expect(screen.getByText("0 tracks")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Late Shift, 0 tracks" }),
    ).toBeInTheDocument();
  });

  it("links a movie playlist to its own page with a movie count", () => {
    renderWithQueryClient(<PlaylistCard playlist={moviePlaylist()} />);

    expect(screen.getByText("7 movies")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Friday Feature, 7 movies" }),
    ).toHaveAttribute("href", "/movies/playlist/9");
  });

  it("swaps a cover that fails to load for the placeholder", () => {
    const { container } = renderWithQueryClient(
      <PlaylistCard
        playlist={moviePlaylist({
          cover_image: nullableString("/api/static/playlists/9.jpg"),
        })}
      />,
    );

    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    fireEvent.error(img!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });
});
