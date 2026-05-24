import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren, ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MovieDetailsHeroActions from "@/components/MovieDetailsHeroActions";
import MovieLikeButton from "@/components/MovieLikeButton";
import {
  MOVIES_LIKED_KEY,
  MOVIE_LIKE_STATUS_KEY,
  MOVIE_WATCH_PROGRESS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  LibraryMovieDetailsMovieType,
  MovieWatchProgressType,
} from "@/types";

const toggleLikeMovieMock = vi.fn();
const setMovieWatchedMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    Link: ({
      children,
      params,
      search,
      to,
      ...props
    }: {
      children: ReactNode;
      params?: { id?: string };
      search?: unknown;
      to?: string;
    }) => {
      void search;
      const href =
        typeof to === "string"
          ? to.replace("$id", params?.id ?? "")
          : "#";

      return (
        <a href={href} {...props}>
          {children}
        </a>
      );
    },
  };
});

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    setMovieWatched: (...args: unknown[]) => setMovieWatchedMock(...args),
    toggleLikeMovie: (...args: unknown[]) => toggleLikeMovieMock(...args),
  };
});

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  };
});

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: false,
      },
      queries: {
        retry: false,
      },
    },
  });
}

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
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });

  return { promise, resolve, reject };
}

function renderWithClient(
  ui: ReactElement,
  setup?: (queryClient: QueryClient) => void,
) {
  const queryClient = createTestQueryClient();
  setup?.(queryClient);

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }

  return {
    queryClient,
    invalidateSpy: vi.spyOn(queryClient, "invalidateQueries"),
    ...render(ui, { wrapper: Wrapper }),
  };
}

function movie(overrides: Partial<LibraryMovieDetailsMovieType> = {}) {
  return {
    id: 22,
    title: "Arrival",
    file_path: "/movies/arrival.mkv",
    file_name: "arrival.mkv",
    size: 1024,
    container: "mkv",
    mime_type: "video/x-matroska",
    adult: false,
    tmdb_id: { Int64: 329865, Valid: true },
    imdb_id: { String: "tt2543164", Valid: true },
    poster_path: { String: "/poster.jpg", Valid: true },
    backdrop_path: { String: "/backdrop.jpg", Valid: true },
    language: { String: "en", Valid: true },
    year: { Int64: 2016, Valid: true },
    release_date: { String: "2016-11-11", Valid: true },
    overview: { String: "A linguist works with aliens.", Valid: true },
    tag_line: { String: "", Valid: false },
    certification: { String: "PG-13", Valid: true },
    critic_rating: { Float64: 94, Valid: true },
    audience_rating: { Float64: 82, Valid: true },
    revenue: { Float64: 0, Valid: false },
    budget: { Float64: 0, Valid: false },
    run_time: { Int64: 116, Valid: true },
    duration: { Float64: 6960, Valid: true },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  } satisfies LibraryMovieDetailsMovieType;
}

function progress(
  overrides: Partial<MovieWatchProgressType> = {},
): ApiResponseType<MovieWatchProgressType> {
  return success({
    progress_sec: 120,
    duration_sec: 3600,
    watched: false,
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  });
}

function renderLikeButton(isLiked = false) {
  return renderWithClient(
    <MovieLikeButton movieId={22} variant="hero" />,
    queryClient => {
      queryClient.setQueryData(
        [MOVIE_LIKE_STATUS_KEY, 22],
        success({ is_liked: isLiked }),
      );
    },
  );
}

function renderHeroActions(watched = false) {
  return renderWithClient(
    <MovieDetailsHeroActions
      movieId={22}
      movie={movie()}
      movieTitle="Arrival"
      user={null}
      playbackSettings={{
        mode: "direct",
        audioTrack: 0,
        subtitleTrack: null,
      }}
      onPlaybackSettingsChange={vi.fn()}
      playbackSettingsOpen={false}
      onPlaybackSettingsOpenChange={vi.fn()}
      technicalDetailsOpen={false}
      onTechnicalDetailsOpenChange={vi.fn()}
      editOpen={false}
      onEditOpenChange={vi.fn()}
      deleteOpen={false}
      onDeleteOpenChange={vi.fn()}
    />,
    queryClient => {
      queryClient.setQueryData(
        [MOVIE_LIKE_STATUS_KEY, 22],
        success({ is_liked: false }),
      );
      queryClient.setQueryData(
        [MOVIE_WATCH_PROGRESS_KEY, 22],
        progress({ watched }),
      );
    },
  );
}

beforeEach(() => {
  toggleLikeMovieMock.mockReset();
  setMovieWatchedMock.mockReset();
  showActionFailedMock.mockReset();
});

describe("MovieLikeButton", () => {
  it("optimistically marks a movie liked before the API response resolves", async () => {
    const pending = deferred<ApiResponseType<{ movie_id: number; is_liked: boolean }>>();
    toggleLikeMovieMock.mockReturnValue(pending.promise);

    const user = userEvent.setup();
    const { queryClient, invalidateSpy } = renderLikeButton(false);

    await user.click(screen.getByRole("button", { name: /like this movie/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /unlike this movie/i }))
        .toHaveAttribute("aria-pressed", "true");
    });

    expect(queryClient.getQueryData([MOVIE_LIKE_STATUS_KEY, 22])).toEqual(
      success({ is_liked: true }),
    );

    pending.resolve(success({ movie_id: 22, is_liked: true }));

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: [MOVIES_LIKED_KEY],
      });
    });
    expect(toggleLikeMovieMock).toHaveBeenCalledWith(22);
  });

  it("rolls back the optimistic like when the API reports a failure", async () => {
    const pending = deferred<ApiResponseType<{ movie_id: number; is_liked: boolean }>>();
    toggleLikeMovieMock.mockReturnValue(pending.promise);

    const user = userEvent.setup();
    const { queryClient } = renderLikeButton(false);

    await user.click(screen.getByRole("button", { name: /like this movie/i }));

    await waitFor(() => {
      expect(queryClient.getQueryData([MOVIE_LIKE_STATUS_KEY, 22])).toEqual(
        success({ is_liked: true }),
      );
    });

    pending.resolve({ error: true, message: "Like failed." });

    await waitFor(() => {
      expect(queryClient.getQueryData([MOVIE_LIKE_STATUS_KEY, 22])).toEqual(
        success({ is_liked: false }),
      );
    });
    expect(showActionFailedMock).toHaveBeenCalledWith(
      "update like",
      "Like failed.",
    );
  });
});

describe("MovieDetailsHeroActions watched button", () => {
  it("optimistically marks a movie watched before the API response resolves", async () => {
    const pending = deferred<ApiResponseType<{ movie_id: number; watched: boolean }>>();
    setMovieWatchedMock.mockReturnValue(pending.promise);

    const user = userEvent.setup();
    const { queryClient } = renderHeroActions(false);

    await user.click(screen.getByRole("button", { name: /mark movie as watched/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /mark movie as unwatched/i }))
        .toHaveAttribute("aria-pressed", "true");
    });

    expect(setMovieWatchedMock).toHaveBeenCalledWith(22, true);
    expect(queryClient.getQueryData([MOVIE_WATCH_PROGRESS_KEY, 22])).toEqual(
      progress({ progress_sec: 0, watched: true }),
    );
  });

  it("calls the backend to mark a watched movie unwatched", async () => {
    const pending = deferred<ApiResponseType<{ movie_id: number; watched: boolean }>>();
    setMovieWatchedMock.mockReturnValue(pending.promise);

    const user = userEvent.setup();
    const { queryClient } = renderHeroActions(true);

    await user.click(screen.getByRole("button", { name: /mark movie as unwatched/i }));

    await waitFor(() => {
      expect(setMovieWatchedMock).toHaveBeenCalledWith(22, false);
    });
    expect(queryClient.getQueryData([MOVIE_WATCH_PROGRESS_KEY, 22])).toEqual(
      progress({ watched: false }),
    );
  });

  it("rolls back the optimistic watched status when the API reports a failure", async () => {
    const pending = deferred<ApiResponseType<{ movie_id: number; watched: boolean }>>();
    setMovieWatchedMock.mockReturnValue(pending.promise);

    const user = userEvent.setup();
    const { queryClient } = renderHeroActions(true);

    await user.click(screen.getByRole("button", { name: /mark movie as unwatched/i }));

    await waitFor(() => {
      expect(queryClient.getQueryData([MOVIE_WATCH_PROGRESS_KEY, 22])).toEqual(
        progress({ watched: false }),
      );
    });

    pending.resolve({ error: true, message: "Watched failed." });

    await waitFor(() => {
      expect(queryClient.getQueryData([MOVIE_WATCH_PROGRESS_KEY, 22])).toEqual(
        progress({ watched: true }),
      );
    });
    expect(showActionFailedMock).toHaveBeenCalledWith(
      "update watched status",
      "Watched failed.",
    );
  });
});
