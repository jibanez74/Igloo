import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SpotifyPicker from "@/components/music/SpotifyPicker";
import type { ApiResponseType, SpotifyAlbumSearchResultType } from "@/types";
import { renderWithQueryClient } from "../helpers/render";

const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
  showCreated: vi.fn(),
  showInfo: vi.fn(),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
  showCreated: toastMocks.showCreated,
  showInfo: toastMocks.showInfo,
}));

function albumResult(
  overrides: Partial<SpotifyAlbumSearchResultType> = {},
): SpotifyAlbumSearchResultType {
  return {
    spotify_id: "album123",
    title: "Blue Record",
    artist_names: ["The Band"],
    release_date: "2026-01-02",
    album_type: "album",
    total_tracks: 9,
    cover_url: "",
    spotify_url: "https://open.spotify.com/album/album123",
    already_in_library: false,
    ...overrides,
  };
}

function renderPicker(
  kind: "album" | "track",
  results: SpotifyAlbumSearchResultType[],
) {
  const searchFn = vi.fn(
    async (): Promise<
      ApiResponseType<{ results: SpotifyAlbumSearchResultType[] }>
    > => ({ error: false, data: { results } }),
  );

  renderWithQueryClient(
    <SpotifyPicker
      kind={kind}
      confirmLabel="Send Request"
      initialTitle="Blue"
      searchFn={searchFn}
      onConfirm={async () => {}}
      getTitleSuffix={result => result.release_date.slice(0, 4)}
    />,
  );

  return { searchFn };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SpotifyPicker", () => {
  // Neither request dialog test reaches an empty result set.
  it("names the kind in the no-matches notice", async () => {
    const user = userEvent.setup();
    renderPicker("album", []);

    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(toastMocks.showInfo).toHaveBeenCalledWith(
        "No Spotify album matches found",
      );
    });
  });

  it("names the track kind in its own no-matches notice", async () => {
    const user = userEvent.setup();
    renderPicker("track", []);

    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(toastMocks.showInfo).toHaveBeenCalledWith(
        "No Spotify track matches found",
      );
    });
  });

  it("counts a single result in the singular", async () => {
    const user = userEvent.setup();
    renderPicker("album", [albumResult()]);

    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    expect(await screen.findByText("1 result found")).toBeInTheDocument();
  });

  it("counts several results in the plural", async () => {
    const user = userEvent.setup();
    renderPicker("album", [
      albumResult(),
      albumResult({ spotify_id: "album999", title: "Green Record" }),
    ]);

    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    expect(await screen.findByText("2 results found")).toBeInTheDocument();
  });

  it("labels its title field for the kind it is picking", () => {
    renderPicker("track", []);

    expect(screen.getByLabelText("Track title")).toBeInTheDocument();
  });
});
