import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import EditMovieDialog from "@/components/movies/EditMovieDialog";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ApiResponseType, LibraryMovieDetailsMovieType, TmdbSearchResultType } from "@/types";
import { createTestQueryClient } from "../helpers/render";

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

function nullableString(value: string | null) {
  return {
    String: value ?? "",
    Valid: value != null,
  };
}

function nullableInt(value: number | null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function buildMovie(
  overrides: Partial<LibraryMovieDetailsMovieType> = {},
): LibraryMovieDetailsMovieType {
  return {
    id: 17,
    title: "The Matrix",
    file_path: "/media/movies/the-matrix.mkv",
    file_name: "the-matrix.mkv",
    size: 1024,
    container: "mkv",
    mime_type: "video/x-matroska",
    adult: false,
    tmdb_id: nullableInt(603),
    imdb_id: nullableString("tt0133093"),
    poster_path: nullableString("/matrix.jpg"),
    backdrop_path: nullableString("/matrix-backdrop.jpg"),
    language: nullableString("en"),
    year: nullableInt(1999),
    release_date: nullableString("1999-03-31"),
    overview: nullableString("A hacker learns the truth about reality."),
    tag_line: nullableString("Welcome to the Real World."),
    certification: nullableString("R"),
    critic_rating: { Float64: 8.7, Valid: true },
    audience_rating: { Float64: 8.6, Valid: true },
    revenue: { Float64: 463517383, Valid: true },
    budget: { Float64: 63000000, Valid: true },
    run_time: { Int64: 136, Valid: true },
    duration: { Float64: 8160, Valid: true },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

type DialogHarnessProps = {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
};

function createQueryClient() {
  const queryClient = createTestQueryClient();

  return queryClient;
}

function getTmdbTitleInput() {
  return screen.getByLabelText("Title", {
    selector: "#tmdb-title",
  }) as HTMLInputElement;
}

function getTmdbYearInput() {
  return screen.getByLabelText("Year", {
    selector: "#tmdb-year",
  }) as HTMLInputElement;
}

function getTmdbIdInput() {
  return screen.getByLabelText("TMDB ID") as HTMLInputElement;
}

function getManualInput(label: string) {
  return screen.getByLabelText(label, {
    selector: "input",
  }) as HTMLInputElement;
}

function getManualOverviewInput() {
  return screen.getByLabelText("Overview") as HTMLTextAreaElement;
}

async function openManualTab(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("tab", { name: "Manual" }));
  await waitFor(() => {
    expect(getManualInput("Title")).toBeVisible();
  });
}

function renderDialog(options: Partial<DialogHarnessProps> = {}) {
  const queryClient = createQueryClient();
  const onOpenChange = vi.fn();
  let currentProps: DialogHarnessProps = {
    movie: options.movie ?? buildMovie(),
    movieId: options.movieId ?? options.movie?.id ?? 17,
  };

  function DialogHarness({ movieId, movie }: DialogHarnessProps) {
    const [open, setOpen] = useState(true);

    return (
      <EditMovieDialog
        movieId={movieId}
        movie={movie}
        open={open}
        onOpenChange={(nextOpen) => {
          onOpenChange(nextOpen);
          setOpen(nextOpen);
        }}
      />
    );
  }

  const view = render(
    <QueryClientProvider client={queryClient}>
      <DialogHarness {...currentProps} />
    </QueryClientProvider>,
  );

  return {
    onOpenChange,
    rerenderDialog(nextProps: Partial<DialogHarnessProps>) {
      currentProps = {
        ...currentProps,
        ...nextProps,
      };

      view.rerender(
        <QueryClientProvider client={queryClient}>
          <DialogHarness {...currentProps} />
        </QueryClientProvider>,
      );
    },
  };
}

describe("EditMovieDialog", () => {
  it("resets TMDB and manual initial values when switching movies while open", async () => {
    const user = userEvent.setup();
    const { rerenderDialog } = renderDialog();

    await user.clear(getTmdbTitleInput());
    await user.type(getTmdbTitleInput(), "Custom TMDB query");

    await openManualTab(user);
    await user.clear(getManualInput("Title"));
    await user.type(getManualInput("Title"), "Custom manual title");

    await user.click(screen.getByRole("tab", { name: "Identify with TMDB" }));

    rerenderDialog({
      movieId: 18,
      movie: buildMovie({
        id: 18,
        title: "Inception",
        tmdb_id: nullableInt(27205),
        year: nullableInt(2010),
        release_date: nullableString("2010-07-16"),
        overview: nullableString("A thief steals secrets through dreams."),
      }),
    });

    await waitFor(() => {
      expect(getTmdbTitleInput()).toHaveValue("Inception");
    });
    expect(getTmdbYearInput().value).toBe("2010");
    expect(getTmdbIdInput().value).toBe("27205");

    await openManualTab(user);
    expect(getManualInput("Title")).toHaveValue("Inception");
    expect(getManualInput("Year").value).toBe("2010");
    expect(getManualOverviewInput()).toHaveAccessibleName("Overview");
    expect(getManualOverviewInput()).toHaveValue(
      "A thief steals secrets through dreams.",
    );
  });

  it("updates clean manual fields when same-movie backing data refreshes", async () => {
    const user = userEvent.setup();
    const { rerenderDialog } = renderDialog();

    await openManualTab(user);

    rerenderDialog({
      movie: buildMovie({
        title: "The Matrix Reloaded",
        year: nullableInt(2003),
        release_date: nullableString("2003-05-15"),
        overview: nullableString("Neo faces a new threat from the machines."),
        certification: nullableString("R"),
      }),
    });

    await waitFor(() => {
      expect(getManualInput("Title")).toHaveValue("The Matrix Reloaded");
    });
    expect(getManualInput("Year").value).toBe("2003");
    expect(getManualInput("Release Date")).toHaveValue("2003-05-15");
    expect(getManualOverviewInput()).toHaveValue(
      "Neo faces a new threat from the machines.",
    );
  });

  it("preserves dirty manual fields when same-movie backing data refreshes", async () => {
    const user = userEvent.setup();
    const { rerenderDialog } = renderDialog();

    await openManualTab(user);
    await user.clear(getManualInput("Title"));
    await user.type(getManualInput("Title"), "My Matrix Cut");

    rerenderDialog({
      movie: buildMovie({
        title: "The Matrix Reloaded",
        year: nullableInt(2003),
        overview: nullableString("Neo faces a new threat from the machines."),
      }),
    });

    await waitFor(() => {
      expect(getManualInput("Year").value).toBe("2003");
    });
    expect(getManualInput("Title")).toHaveValue("My Matrix Cut");
    expect(getManualOverviewInput()).toHaveValue(
      "Neo faces a new threat from the machines.",
    );
  });

  it("saves only fields changed from the current manual baseline", async () => {
    apiMocks.updateMovieMetadata.mockResolvedValue(success({}));

    const user = userEvent.setup();
    const { rerenderDialog, onOpenChange } = renderDialog();

    await openManualTab(user);

    rerenderDialog({
      movie: buildMovie({
        title: "The Matrix Reloaded",
        year: nullableInt(2003),
        overview: nullableString("Neo faces a new threat from the machines."),
      }),
    });

    await waitFor(() => {
      expect(getManualInput("Title")).toHaveValue("The Matrix Reloaded");
    });

    await user.clear(getManualInput("Title"));
    await user.type(getManualInput("Title"), "The Matrix Reloaded: Edited");
    await user.clear(getManualInput("Year"));
    await user.type(getManualInput("Year"), "2004");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(apiMocks.updateMovieMetadata).toHaveBeenCalledWith(17, {
        title: "The Matrix Reloaded: Edited",
        year: 2004,
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

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
