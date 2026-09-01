import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AddToPlaylistDialog from "@/components/music/AddToPlaylistDialog";
import { PLAYLISTS_KEY } from "@/lib/constants";
import type { ApiResponseType, PlaylistsListResponseType } from "@/types";
import { createTestQueryClient } from "../helpers/render";

const getPlaylistsMock = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    // Stubbed only to keep the dialog off the network; no test asserts on it.
    addTracksToPlaylist: vi.fn(),
    getPlaylists: () => getPlaylistsMock(),
  };
});

function success<T extends Record<string, unknown>>(
  data: T,
): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function playlists(): ApiResponseType<PlaylistsListResponseType> {
  return success({
    playlists: Array.from({ length: 6 }, (_, index) => ({
      id: index + 1,
      user_id: 1,
      name: `Playlist ${index + 1}`,
      description: { String: "", Valid: false },
      cover_image: { String: "", Valid: false },
      is_public: false,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      track_count: index,
      total_duration: 0,
      is_owner: true,
      can_edit: true,
    })),
  });
}

function renderDialog() {
  const queryClient = createTestQueryClient();
  const playlistsResponse = playlists();
  queryClient.setQueryData([PLAYLISTS_KEY], playlistsResponse);
  getPlaylistsMock.mockResolvedValue(playlistsResponse);

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }

  return render(
    <AddToPlaylistDialog
      open
      onOpenChange={vi.fn()}
      trackId={7}
      trackTitle="First Contact"
    />,
    { wrapper: Wrapper },
  );
}

beforeEach(() => {
  getPlaylistsMock.mockReset();
});

describe("AddToPlaylistDialog", () => {
  it("gives the playlist search input an accessible name", () => {
    renderDialog();

    expect(screen.getByLabelText("Search playlists")).toBeInTheDocument();
  });
});
