import { createFileRoute } from "@tanstack/react-router";
import MovieCard from "../../components/MovieCard";
import Pagination from "../../components/Pagination";
import type { PaginationSearch } from "../../types/Pagination";
import type { MovieResponse } from "../../types/Movie";

export const Route = createFileRoute("/movies/")({
  validateSearch: (search: Record<string, unknown>): PaginationSearch => ({
    page: Number(search?.page ?? 1),
    limit: Number(search?.limit ?? 24),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps }): Promise<MovieResponse> => {
    const res = await fetch(
      `/api/v1/movies?page=${deps.page}&limit=${deps.limit}`,
      {
        credentials: "include",
      }
    );

    if (!res.ok) {
      throw new Error(`Failed to fetch movies: ${res.statusText}`);
    }

    return res.json();
  },
  component: MoviesPage,
});

function MoviesPage() {
  const { movies, total_movies, current_page, total_pages } =
    Route.useLoaderData();
  const navigate = Route.useNavigate();

  return (
    <main
      className='min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900'
      aria-label='Movie gallery page'
    >
      <div className='max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8'>
        {/* Header Section */}
        <header className='mb-8'>
          <h1 className='text-3xl font-bold text-white mb-2'>Movies</h1>
          <p className='text-sky-200' role='status' aria-live='polite'>
            {total_movies} {total_movies === 1 ? "movie" : "movies"} available
          </p>
        </header>

        {/* Gallery Section */}
        <section className='min-h-[200px]' aria-label='Movie gallery'>
          {movies.length === 0 ? (
            <p
              className='flex items-center justify-center h-64 text-sky-200'
              role='status'
            >
              No movies available
            </p>
          ) : (
            <ul
              className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6'
              role='list'
            >
              {movies.map(movie => (
                <li key={movie.id}>
                  <MovieCard movie={movie} imgLoading='lazy' />
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* Pagination Section */}
        {total_pages > 1 && (
          <nav className='mt-8' aria-label='Movie gallery pagination'>
            <Pagination
              currentPage={current_page}
              totalPages={total_pages}
              onPageChange={page => {
                window.scrollTo({ top: 0, behavior: "smooth" });
                navigate({
                  search: prev => ({ ...prev, page }),
                });
              }}
            />
          </nav>
        )}
      </div>
    </main>
  );
}
