import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import EditMovieDialog from "@/components/EditMovieDialog";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ApiResponseType, LibraryMovieDetailsMovieType, TmdbSearchResultType } from "@/types";

const apiMocks = vi.hoisted(() => ({
  identifyMovie: vi.fn(),
  searchTmdbMovies: vi.fn(),
  updateMovieMetadata: vi.fn(),
}));
const toastMocks = vi.hoisted(() => ({
  error: vi.fn(),
  info: vi.fn(),
  success: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    identifyMovie: (...args: unknown[]) => apiMocks.identifyMovie(...args),
    searchTmdbMovies: (...args: unknown[]) => apiMocks.searchTmdbMovies(...args),
    updateMovieMetadata: (...args: unknown[]) =>
      apiMocks.updateMovieMetadata(...args),
  };
});

vi.mock("sonner", () => ({
  toast: {
    error: toastMocks.error,
    info: toastMocks.info,
    success: toastMocks.success,
  },
}));

function success<T extends Record<string, unknown>>(data: T): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function tmdbResult(
  overrides: Partial<TmdbSearchResultType> = {},
): TmdbSearchResultType {
  return {
    tmdb_id: 603,
    title: "The Matrix",
    release_date: "1999-03-31",
    overview: "A hacker learns the truth about reality.",
    poster_path: "/matrix.jpg",
    already_in_library: false,
    ...overrides,
  };
}

function buildMovie(): LibraryMovieDetailsMovieType {
  return {
    id: 17,
    title: "The Matrix",
    file_path: "/media/movies/the-matrix.mkv",
    file_name: "the-matrix.mkv",
    size: 1024,
    container: "mkv",
    mime_type: "video/x-matroska",
    adult: false,
    tmdb_id: { Int64: 603, Valid: true },
    imdb_id: { String: "tt0133093", Valid: true },
    poster_path: { String: "/matrix.jpg", Valid: true },
    backdrop_path: { String: "/matrix-backdrop.jpg", Valid: true },
    language: { String: "en", Valid: true },
    year: { Int64: 1999, Valid: true },
    release_date: { String: "1999-03-31", Valid: true },
    overview: { String: "A hacker learns the truth about reality.", Valid: true },
    tag_line: { String: "Welcome to the Real World.", Valid: true },
    certification: { String: "R", Valid: true },
    critic_rating: { Float64: 8.7, Valid: true },
    audience_rating: { Float64: 8.6, Valid: true },
    revenue: { Float64: 463517383, Valid: true },
    budget: { Float64: 63000000, Valid: true },
    run_time: { Int64: 136, Valid: true },
    duration: { Float64: 8160, Valid: true },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
  const onOpenChange = vi.fn();

  function DialogHarness() {
    const [open, setOpen] = useState(true);

    return (
      <EditMovieDialog
        movieId={17}
        movie={buildMovie()}
        open={open}
        onOpenChange={(nextOpen) => {
          onOpenChange(nextOpen);
          setOpen(nextOpen);
        }}
      />
    );
  }

  render(
    <QueryClientProvider client={queryClient}>
      <DialogHarness />
    </QueryClientProvider>,
  );

  return { onOpenChange };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe("EditMovieDialog", () => {
  it("supports keyboard selection when identifying a movie with TMDB", async () => {
    apiMocks.searchTmdbMovies.mockResolvedValue(
      success({
        results: [
          tmdbResult(),
          tmdbResult({
            tmdb_id: 605,
            title: "The Matrix Revolutions",
            release_date: "2003-11-05",
          }),
        ],
      }),
    );
    apiMocks.identifyMovie.mockResolvedValue(success({}));

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    const titleInput = screen.getByLabelText("Title");
    const applyButton = screen.getByRole("button", { name: "Apply Selected" });

    await waitFor(() => {
      expect(titleInput).toHaveFocus();
    });
    expect(applyButton).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Search TMDB" }));

    await waitFor(() => {
      expect(apiMocks.searchTmdbMovies).toHaveBeenCalledWith({
        title: "The Matrix",
        year: 1999,
        tmdb_id: 603,
      });
    });

    const resultRadios = await screen.findAllByRole("radio");

    await user.tab();
    expect(resultRadios[0]).toHaveFocus();

    await user.keyboard("{Space}");
    await waitFor(() => {
      expect(resultRadios[0]).toBeChecked();
      expect(applyButton).toBeEnabled();
    });

    await user.keyboard("{ArrowDown}");
    expect(resultRadios[1]).toHaveFocus();
    await waitFor(() => {
      expect(resultRadios[1]).toBeChecked();
    });

    await user.click(applyButton);

    await waitFor(() => {
      expect(apiMocks.identifyMovie).toHaveBeenCalledWith(17, 605);
    });
    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    expect(screen.queryByRole("dialog", { name: "Edit Movie" }))
      .not.toBeInTheDocument();
    expect(toastMocks.success).toHaveBeenCalledWith(
      "Movie identified successfully",
    );
  });
});
