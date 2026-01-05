import { useQuery } from "@tanstack/react-query";
import { inTheatersQueryOpts } from "@/lib/query-opts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import MovieCard from "@/components/MovieCard";
import type { TheaterMovieType } from "@/types";

export default function MoviesInTheaters() {
  const { data, isPending } = useQuery(inTheatersQueryOpts());

  // Sort movies from newest to oldest by release date
  const movies =
    data && !data.error
      ? [...data.data.movies].sort((a, b) => {
          const dateA = new Date(a.release_date).getTime();
          const dateB = new Date(b.release_date).getTime();
          return dateB - dateA;
        })
      : [];

  const hasError = data && data.error;

  return (
    <section
      role='region'
      aria-labelledby='movies-in-theaters'
      aria-label='Now Playing in Theaters'
      className='mt-10'
    >
      <h2
        id='movies-in-theaters'
        className='text-xl md:text-2xl font-semibold tracking-tight mb-4'
      >
        Now Playing in Theaters
      </h2>

      {isPending ? (
        <div
          className='py-12 flex items-center justify-center'
          role='status'
          aria-label='Loading movies...'
        >
          <Spinner className='size-8 text-cyan-400' />
          <span className='sr-only'>Loading movies...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant='destructive'
          className='bg-red-500/10 border-red-500/20 text-red-400'
        >
          <i className='fa-solid fa-circle-exclamation' aria-hidden='true'></i>
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>
            {data.message || "Failed to load movies. Please try again later."}
          </AlertDescription>
        </Alert>
      ) : movies.length > 0 ? (
        <>
          <span
            tabIndex={0}
            className='sr-only focus:not-sr-only focus:absolute focus:bg-slate-800 focus:text-white focus:px-4 focus:py-2 focus:rounded-md focus:z-50'
            aria-label={`Now Playing in Theaters section, ${movies.length} movies`}
          >
            Now Playing in Theaters - {movies.length} movies
          </span>
          <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
            {movies.map((movie: TheaterMovieType) => (
            <MovieCard key={movie.id} movie={movie} />
          ))}
        </div>
        </>
      ) : (
        <div className='text-center py-12'>
          <div className='mx-auto w-16 h-16 bg-slate-800 rounded-full flex items-center justify-center mb-4 border border-cyan-500/20'>
            <i className='fa-solid fa-film text-cyan-600 text-2xl'></i>
          </div>
          <h3 className='text-lg font-semibold text-slate-300 mb-2'>
            No Movies Available
          </h3>
          <p className='text-slate-400 max-w-md mx-auto'>
            Unable to fetch movies currently playing in theaters. Check back
            later.
          </p>
        </div>
      )}
    </section>
  );
}
