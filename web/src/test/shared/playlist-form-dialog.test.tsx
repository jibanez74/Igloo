import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import PlaylistFormDialog from "@/components/shared/PlaylistFormDialog";
import {
  MOVIE_PLAYLIST_DETAILS_KEY,
  PLAYLIST_DETAILS_KEY,
} from "@/lib/constants";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  expectRefetchedBeforeReload,
  recordRefreshOrder,
} from "../helpers/route-refresh";

const apiMocks = vi.hoisted(() => ({
  updatePlaylist: vi.fn(),
  updateMoviePlaylist: vi.fn(),
}));
const routerMocks = vi.hoisted(() => ({
  invalidate: vi.fn(() => Promise.resolve()),
}));

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    updatePlaylist: (...args: unknown[]) => apiMocks.updatePlaylist(...args),
    updateMoviePlaylist: (...args: unknown[]) =>
      apiMocks.updateMoviePlaylist(...args),
  };
});

// A rename reloads the playlist route so its head picks up the new name; the
// tests render without a router.
vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual<
    typeof import("@tanstack/react-router")
  >("@tanstack/react-router");

  return { ...actual, useRouter: () => routerMocks };
});

describe("PlaylistFormDialog", () => {
  it("gives the description textarea an accessible name from the visible label", () => {
    const queryClient = createTestQueryClient();

    renderWithQueryClient(
      <PlaylistFormDialog kind="music" mode="create" open onOpenChange={vi.fn()} />,
      { queryClient },
    );

    expect(
      screen.getByRole("textbox", { name: /^Description/ }),
    ).toHaveAccessibleName("Description (optional)");
  });

  it("keeps the cover, refetches the playlist and reloads its route after a rename", async () => {
    const user = userEvent.setup();
    const queryClient = createTestQueryClient();
    const refreshOrder = recordRefreshOrder(queryClient, routerMocks.invalidate);
    const onOpenChange = vi.fn();
    apiMocks.updatePlaylist.mockResolvedValue({
      error: false,
      data: { playlist: { id: 7 } },
    });

    renderWithQueryClient(
      <PlaylistFormDialog
        kind="music"
        mode="edit"
        open
        onOpenChange={onOpenChange}
        playlist={{
          id: 7,
          name: "Road Trip",
          description: { String: "", Valid: false },
          cover_image: { String: "/covers/road-trip.jpg", Valid: true },
          is_public: false,
        }}
      />,
      { queryClient },
    );

    const nameInput = screen.getByRole("textbox", { name: /^Name/ });
    await user.clear(nameInput);
    await user.type(nameInput, "Night Drive");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(routerMocks.invalidate).toHaveBeenCalledOnce();
    });
    expect(apiMocks.updatePlaylist).toHaveBeenCalledWith(7, {
      name: "Night Drive",
      description: undefined,
      cover_image: "/covers/road-trip.jpg",
      is_public: false,
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expectRefetchedBeforeReload(refreshOrder, [PLAYLIST_DETAILS_KEY, 7]);
  });

  // A movie playlist PUT is the same full replace, keyed and routed for the
  // movies library; movie_id is left out so the server keeps it.
  it("edits a movie playlist through the movie client, keys and route", async () => {
    const user = userEvent.setup();
    const queryClient = createTestQueryClient();
    const refreshOrder = recordRefreshOrder(queryClient, routerMocks.invalidate);
    const onOpenChange = vi.fn();
    apiMocks.updateMoviePlaylist.mockResolvedValue({
      error: false,
      data: { playlist: { id: 11 } },
    });

    renderWithQueryClient(
      <PlaylistFormDialog
        kind="movie"
        mode="edit"
        open
        onOpenChange={onOpenChange}
        playlist={{
          id: 11,
          name: "Weekend Picks",
          description: { String: "Two for Saturday.", Valid: true },
          cover_image: { String: "", Valid: false },
          is_public: true,
        }}
      />,
      { queryClient },
    );

    const nameInput = screen.getByRole("textbox", { name: /^Name/ });
    await user.clear(nameInput);
    await user.type(nameInput, "Saturday Picks");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(routerMocks.invalidate).toHaveBeenCalledOnce();
    });
    expect(apiMocks.updateMoviePlaylist).toHaveBeenCalledWith(11, {
      name: "Saturday Picks",
      description: "Two for Saturday.",
      cover_image: undefined,
      is_public: true,
    });
    expect(apiMocks.updatePlaylist).not.toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expectRefetchedBeforeReload(refreshOrder, [MOVIE_PLAYLIST_DETAILS_KEY, 11]);
    expect(routerMocks.invalidate).toHaveBeenCalledWith({
      filter: expect.any(Function),
    });
  });

  it("names the create dialog's purpose per library", () => {
    const queryClient = createTestQueryClient();

    renderWithQueryClient(
      <PlaylistFormDialog kind="movie" mode="create" open onOpenChange={vi.fn()} />,
      { queryClient },
    );

    expect(
      screen.getByRole("dialog", { name: "Create New Playlist" }),
    ).toHaveAccessibleDescription(
      "Create a playlist for movies. Track playlists stay on the Music page.",
    );
  });
});
