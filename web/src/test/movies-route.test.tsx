import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routeTree } from "@/routeTree.gen";

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

function nullableString(value = "") {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function movie(id: number, title: string, year: number) {
  return {
    id,
    title,
    poster_path: nullableString(),
    year: nullableInt64(year),
  };
}

function playlist(id: number, name: string, movieCount: number) {
  return {
    id,
    user_id: 1,
    name,
    description: nullableString(),
    cover_image: nullableString(),
    is_public: false,
    folder_id: {
      Int64: 0,
      Valid: false,
    },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    movie_count: movieCount,
    total_duration: 0,
    is_owner: true,
    can_edit: true,
  };
}

function mockMoviesFetch() {
  const libraryMovies = [movie(1, "Arrival", 2016), movie(2, "Heat", 1995)];
  const likedMovies = [movie(3, "Moonlight", 2016)];
  const playlists = [playlist(11, "Weekend Picks", 2)];

  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Movie User",
            email: "movies@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url === "/api/movies/stats") {
      return jsonResponse({
        error: false,
        data: {
          total_movies: 42,
        },
      });
    }

    if (url === "/api/movies/library?page=1&per_page=24&sort=asc") {
      return jsonResponse({
        error: false,
        data: {
          movies: libraryMovies,
          total: libraryMovies.length,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/movies/genres") {
      return jsonResponse({
        error: false,
        data: {
          genres: [
            {
              genre_id: 10,
              genre_tag: "Action",
              movie_count: 2,
            },
            {
              genre_id: 20,
              genre_tag: "Drama",
              movie_count: 1,
            },
          ],
        },
      });
    }

    if (url === "/api/movies/genres/10/movies?page=1&per_page=24&sort=asc") {
      return jsonResponse({
        error: false,
        data: {
          movies: [movie(4, "Die Hard", 1988)],
          total: 1,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/movies/playlists") {
      return jsonResponse({
        error: false,
        data: {
          playlists,
        },
      });
    }

    if (url === "/api/movies/liked?page=1&per_page=24&sort=asc") {
      return jsonResponse({
        error: false,
        data: {
          movies: likedMovies,
          total: likedMovies.length,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function createMoviesQueryClient() {
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

async function renderMoviesRoute(initialEntry: string) {
  window.scrollTo = vi.fn();
  mockMoviesFetch();

  const queryClient = createMoviesQueryClient();
  const history = createMemoryHistory({
    initialEntries: [initialEntry],
  });
  const router = createRouter({
    routeTree,
    context: {
      queryClient,
    },
    history,
  });

  await act(async () => {
    await router.load();
  });

  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ queryClient }} />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("movies route focus restoration", () => {
  it("restores focus to the selected genre button after clearing the filter", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=genres&genreId=10");

    await screen.findByRole("button", { name: "Clear genre filter" });
    await user.click(screen.getByRole("button", { name: "Clear genre filter" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Action/i })).toHaveFocus();
    });
  });

  it("moves focus to Back to playlists when liked movies is opened from the playlists toolbar", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=playlists");

    await user.click(screen.getByRole("button", { name: "Liked movies" }));

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Back to playlists" }),
      ).toHaveFocus();
    });
  });

  it("restores focus to the playlists toolbar liked movies button after returning", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/?tab=playlists&view=liked");

    await user.click(
      await screen.findByRole("button", { name: "Back to playlists" }),
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Liked movies" })).toHaveFocus();
    });
  });

  it("does not override dropdown focus behavior when liked movies is opened from More options", async () => {
    const user = userEvent.setup();

    await renderMoviesRoute("/movies/");

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Liked movies" }),
    );

    await screen.findByRole("button", { name: "Back to playlists" });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  });
});
