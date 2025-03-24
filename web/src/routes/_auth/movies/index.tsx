import { Show, For } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import MovieCard from "../../../components/MovieCard";
import Pagination from "../../../components/Pagination";
import type { PaginationSearch } from "../../../types/Pagination";
import type { PaginationResponse } from "../../../types/Pagination";
import type { SimpleMovie } from "../../../types/Movie";

export const Route = createFileRoute("/_auth/movies/")({
  component: MoviesPage,
  validateSearch: (search: Record<string, unknown>): PaginationSearch => ({
    page: Number(search?.page ?? 1),
    limit: Number(search?.limit ?? 10),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps }): Promise<PaginationResponse<SimpleMovie>> => {
    try {
      const res = await fetch(
        `/api/v1/movies?page=${deps.page}&limit=${deps.limit}`,
        {
          credentials: "same-origin",
        }
      );

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
  const { items, current_page, count, total_pages } = data();

  const navigate = Route.useNavigate();

  const onPageChange = (page: number) =>
    navigate({
      to: "/movies",
      from: Route.fullPath,
      resetScroll: true,
      search: {
        limit: 24,
        page,
      },
    });

  return (
    <div class="py-8">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <header class="flex items-center justify-between mb-6">
          <h1 class="text-2xl font-bold text-yellow-300">Movies</h1>
          <p class="text-blue-200">Total Movies: {count}</p>
        </header>

        <section class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6">
          <For each={items}>
            {(movie) => <MovieCard movie={movie} imgLoading="lazy" />}
          </For>
        </section>

        <Show when={total_pages > 1}>
          <nav class="mt-8" aria-label="Movie gallery pagination">
            <Pagination
              currentPage={current_page}
              totalPages={total_pages}
              onPageChange={onPageChange}
            />
          </nav>
        </Show>
      </div>
    </div>
  );
}
