import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Film } from "lucide-react";
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
        className='mb-4 text-xl font-semibold tracking-tight md:text-2xl'
      >
        Now Playing in Theaters
      </h2>

      {isPending ? (
        <div
          className='flex items-center justify-center py-12'
          role='status'
          aria-label='Loading movies...'
        >
          <Spinner className='size-8 text-cyan-400' />
          <span className='sr-only'>Loading movies...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant='destructive'
          className='border-red-500/20 bg-red-500/10 text-red-400'
        >
          <AlertCircle className="size-4" aria-hidden="true" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>
            {data.message || "Failed to load movies. Please try again later."}
          </AlertDescription>
        </Alert>
      ) : movies.length > 0 ? (
        <>
          <span
            tabIndex={0}
            className='sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-slate-800 focus:px-4 focus:py-2 focus:text-white'
            aria-label={`Now Playing in Theaters section, ${movies.length} movies`}
          >
            Now Playing in Theaters - {movies.length} movies
          </span>
          <div className='grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6'>
            {movies.map((movie: TheaterMovieType) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </>
      ) : (
        <div className='py-12 text-center'>
          <div className='mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full border border-cyan-500/20 bg-slate-800'>
            <Film className="size-6 text-cyan-600" aria-hidden="true" />
          </div>
          <h3 className='mb-2 text-lg font-semibold text-slate-300'>
            No Movies Available
          </h3>
          <p className='mx-auto max-w-md text-slate-400'>
            Unable to fetch movies currently playing in theaters. Check back
            later.
          </p>
        </div>
      )}
    </section>
  );
}
