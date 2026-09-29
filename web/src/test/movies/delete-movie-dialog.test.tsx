import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeleteMovieDialog from "@/components/movies/DeleteMovieDialog";
import {
  CONTINUE_WATCHING_KEY,
  LATEST_MOVIES_KEY,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIES_LIBRARY_KEY,
  MOVIES_LIKED_KEY,
  MOVIES_STATS_KEY,
} from "@/lib/constants";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

const mocks = vi.hoisted(() => ({
  deleteMovie: vi.fn(),
  navigate: vi.fn(),
  showActionFailed: vi.fn(),
  showDeleted: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  deleteMovie: mocks.deleteMovie,
}));

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: mocks.showActionFailed,
  showDeleted: mocks.showDeleted,
}));

vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual<
    typeof import("@tanstack/react-router")
  >("@tanstack/react-router");

  return { ...actual, useNavigate: () => mocks.navigate };
});

const MOVIE_ID = 7;

function renderDialog() {
  const queryClient = createTestQueryClient();
  // Cached lists that can each hold the movie being deleted.
  const listKeys = [
    [MOVIES_LIBRARY_KEY, 1, "asc"],
    [MOVIES_STATS_KEY],
    [MOVIES_LIKED_KEY, 1],
    [LATEST_MOVIES_KEY],
    [CONTINUE_WATCHING_KEY],
  ];
  for (const key of listKeys) {
    queryClient.setQueryData(key, { error: false, data: {} });
  }
  queryClient.setQueryData([LIBRARY_MOVIE_DETAILS_KEY, MOVIE_ID], {
    error: false,
    data: {},
  });

  renderWithQueryClient(
    <DeleteMovieDialog
      movieId={MOVIE_ID}
      movieTitle="Heat"
      open
      onOpenChange={vi.fn()}
    />,
    { queryClient },
  );

  return { queryClient, listKeys };
}

describe("DeleteMovieDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.navigate.mockResolvedValue(undefined);
  });

  it("drops the movie and marks every list that could show it stale", async () => {
    mocks.deleteMovie.mockResolvedValue({ error: false });
    const user = userEvent.setup();
    const { queryClient, listKeys } = renderDialog();

    await user.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(mocks.navigate).toHaveBeenCalledWith({ to: "/" });
    });
    await waitFor(() => {
      for (const key of listKeys) {
        expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true);
      }
    });
    expect(
      queryClient.getQueryData([LIBRARY_MOVIE_DETAILS_KEY, MOVIE_ID]),
    ).toBeUndefined();
    expect(mocks.showDeleted).toHaveBeenCalledWith('"Heat"');
  });

  it("keeps the caches and stays put when the server refuses", async () => {
    mocks.deleteMovie.mockResolvedValue({
      error: true,
      message: "Movie is playing.",
    });
    const user = userEvent.setup();
    const { queryClient, listKeys } = renderDialog();

    await user.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(mocks.showActionFailed).toHaveBeenCalledWith(
        "delete movie",
        "Movie is playing.",
      );
    });
    expect(mocks.navigate).not.toHaveBeenCalled();
    for (const key of listKeys) {
      expect(queryClient.getQueryState(key)?.isInvalidated).toBe(false);
    }
  });
});
