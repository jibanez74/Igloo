import { Show, For } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import MovieCard from "../../../components/MovieCard";
import Pagination from "../../../components/Pagination";
import type { MoviesResponse } from "../../../types/Movie";

export const Route = createFileRoute("/_auth/movies/")({
  component: MoviesPage,
  loader: async (): Promise<MoviesResponse> => {
    try {
      const res = await fetch("/api/v1/movies", {
        credentials: "include",
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          `${res.status} - ${data.error ? data.error : res.statusText}`
        );
      }

      return data;
    } catch (err) {
      console.error(err);
      throw new Error("a network error occurred while fetching movies");
    }
  },
});

function MoviesPage() {
  const data = Route.useLoaderData();
  const { movies, current_page, total_movies, total_pages } = data();

  return (
    <main class="py-8">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between mb-6">
          <h1 class="text-2xl font-bold text-white">Movies</h1>

          <p class="text-blue-200">Total Movies: {total_movies}</p>
        </div>

        <ul class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6">
          <For each={movies}>
            {(movie) => (
              <li>
                <MovieCard movie={movie} imgLoading="lazy" />
              </li>
            )}
          </For>
        </ul>

        <Show when={total_pages > 1}>
          <nav class="mt-8" aria-label="Movie gallery pagination">
            <Pagination
              currentPage={current_page}
              totalPages={total_pages}
              onPageChange={(page) => {
                window.scrollTo({ top: 0, behavior: "smooth" });
                console.log(page);
              }}
            />
          </nav>
        </Show>
      </div>
    </main>
  );
}
