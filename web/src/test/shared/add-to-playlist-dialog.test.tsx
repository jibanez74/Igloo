import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AddToPlaylistDialog from "@/components/shared/AddToPlaylistDialog";
import {
  MOVIE_PLAYLIST_DETAILS_KEY,
  MOVIE_PLAYLIST_MOVIES_KEY,
  MOVIE_PLAYLISTS_KEY,
  PLAYLISTS_KEY,
} from "@/lib/constants";
import type { PlaylistKind } from "@/lib/playlist-kinds";
import type {
  ApiResponseType,
  MoviePlaylistsListResponseType,
  PlaylistsListResponseType,
} from "@/types";
import { moviePlaylistSummary } from "../helpers/movies";
import { playlistSummary } from "../helpers/music";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

const getPlaylistsMock = vi.fn();
const getMoviePlaylistsMock = vi.fn();
const addTracksToPlaylistMock = vi.fn();
const addMoviesToMoviePlaylistMock = vi.fn();
const showAddedMock = vi.fn();
const showInfoMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    addTracksToPlaylist: (...args: unknown[]) => addTracksToPlaylistMock(...args),
    addMoviesToMoviePlaylist: (...args: unknown[]) =>
      addMoviesToMoviePlaylistMock(...args),
    getPlaylists: () => getPlaylistsMock(),
    getMoviePlaylists: () => getMoviePlaylistsMock(),
  };
});

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showAdded: (...args: unknown[]) => showAddedMock(...args),
    showInfo: (...args: unknown[]) => showInfoMock(...args),
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(res => {
    resolve = res;
  });

  return { promise, resolve };
}

function musicPlaylists(): ApiResponseType<PlaylistsListResponseType> {
  return success({
    playlists: Array.from({ length: 6 }, (_, index) =>
      playlistSummary({
        id: index + 1,
        name: `Playlist ${index + 1}`,
        track_count: index,
      }),
    ),
  });
}

function moviePlaylists(): ApiResponseType<MoviePlaylistsListResponseType> {
  return success({
    playlists: [
      moviePlaylistSummary({ id: 2, name: "Friday Feature", movie_count: 7 }),
      moviePlaylistSummary({ id: 3, name: "Solo Picks", movie_count: 1 }),
      // Shared for viewing only: the picker leaves it out.
      moviePlaylistSummary({
        id: 4,
        name: "Guest Picks",
        user_id: 2,
        is_owner: false,
        can_edit: false,
      }),
    ],
  });
}

function renderDialog(kind: PlaylistKind) {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const onOpenChange = vi.fn();

  if (kind === "music") {
    const response = musicPlaylists();
    queryClient.setQueryData([PLAYLISTS_KEY], response);
    getPlaylistsMock.mockResolvedValue(response);
  } else {
    const response = moviePlaylists();
    queryClient.setQueryData([MOVIE_PLAYLISTS_KEY], response);
    getMoviePlaylistsMock.mockResolvedValue(response);
  }

  const view = renderWithQueryClient(
    <AddToPlaylistDialog
      kind={kind}
      open
      onOpenChange={onOpenChange}
      itemId={7}
      itemTitle="First Contact"
    />,
    { queryClient },
  );

  return { ...view, invalidateSpy, onOpenChange };
}

beforeEach(() => {
  getPlaylistsMock.mockReset();
  getMoviePlaylistsMock.mockReset();
  addTracksToPlaylistMock.mockReset();
  addMoviesToMoviePlaylistMock.mockReset();
  showAddedMock.mockReset();
  showInfoMock.mockReset();
  showActionFailedMock.mockReset();
});

describe("AddToPlaylistDialog", () => {
  it("gives the playlist search input an accessible name", () => {
    renderDialog("music");

    expect(screen.getByLabelText("Search playlists")).toBeInTheDocument();
  });

  it("says how many playlists the track went to, singular when it is one", async () => {
    const user = userEvent.setup();
    addTracksToPlaylistMock.mockResolvedValue(success({ added: 1, skipped: 0 }));
    renderDialog("music");

    await user.click(screen.getByRole("button", { name: /^Playlist 2/ }));
    await user.click(screen.getByRole("button", { name: "Add to 1 Playlist" }));

    await waitFor(() => {
      expect(showAddedMock).toHaveBeenCalledWith("Track", "to 1 playlist");
    });
    expect(addTracksToPlaylistMock).toHaveBeenCalledWith(2, [7]);
  });

  it("lists the movie playlists the viewer may edit, with their movie counts", () => {
    renderDialog("movie");

    expect(
      screen.getByRole("button", { name: "Friday Feature 7 movies" }),
    ).toHaveAttribute("aria-pressed", "false");
    expect(
      screen.getByRole("button", { name: "Solo Picks 1 movie" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Guest Picks")).not.toBeInTheDocument();
    // Three playlists is under the search threshold.
    expect(screen.queryByLabelText("Search playlists")).not.toBeInTheDocument();
  });

  it("adds a movie through the movie playlist client and refreshes that playlist", async () => {
    const user = userEvent.setup();
    // Answer only once the pending state has rendered: the dialog must still
    // close from that render, where its Cancel path is held.
    const response = deferred<ApiResponseType<{ added: number; skipped: number }>>();
    addMoviesToMoviePlaylistMock.mockReturnValue(response.promise);
    const { invalidateSpy, onOpenChange } = renderDialog("movie");

    await user.click(screen.getByRole("button", { name: /^Friday Feature/ }));
    await user.click(screen.getByRole("button", { name: "Add to 1 Playlist" }));
    await screen.findByRole("button", { name: /Adding\.\.\./ });
    response.resolve(success({ added: 1, skipped: 0 }));

    await waitFor(() => {
      expect(showAddedMock).toHaveBeenCalledWith("Movie", "to 1 playlist");
    });
    expect(addMoviesToMoviePlaylistMock).toHaveBeenCalledWith(2, [7]);
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: [MOVIE_PLAYLIST_MOVIES_KEY, 2],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: [MOVIE_PLAYLIST_DETAILS_KEY, 2],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: [MOVIE_PLAYLISTS_KEY],
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("says the movie was already there when the server skipped it", async () => {
    const user = userEvent.setup();
    addMoviesToMoviePlaylistMock.mockResolvedValue(
      success({ added: 0, skipped: 1 }),
    );
    const { invalidateSpy, onOpenChange } = renderDialog("movie");

    await user.click(screen.getByRole("button", { name: /^Solo Picks/ }));
    await user.click(screen.getByRole("button", { name: "Add to 1 Playlist" }));

    await waitFor(() => {
      expect(showInfoMock).toHaveBeenCalledWith(
        "Movie already in selected playlists",
      );
    });
    expect(invalidateSpy).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  // The client answers failures as envelopes rather than throwing, so a
  // refused add must be told apart from a skipped duplicate.
  it("reports a refused add as a failure and keeps the dialog open", async () => {
    const user = userEvent.setup();
    const failure = { error: true, message: "internal_error", status: 500 };
    addMoviesToMoviePlaylistMock.mockResolvedValue(failure);
    const { onOpenChange } = renderDialog("movie");

    await user.click(screen.getByRole("button", { name: /^Solo Picks/ }));
    await user.click(screen.getByRole("button", { name: "Add to 1 Playlist" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "add movie to playlists",
        failure,
      );
    });
    expect(showInfoMock).not.toHaveBeenCalled();
    expect(showAddedMock).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Add to Playlist" })).toBeInTheDocument();
  });
});
