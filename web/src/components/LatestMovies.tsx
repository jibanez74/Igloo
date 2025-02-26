import {useQuery} from '@tanstack/react-query'
import MovieCard from "./MovieCard";
import ErrorWarning from "./ErrorWarning";
import Spinner from "./Spinner";
import type { SimpleMovie } from "../types/Movie";

export default function LatestMovies() {
  const {data: movies, error, isError, isPending} = useQuery({
    queryKey: ['latest-movies'],
    queryFn: async (): Promise<SimpleMovie[]> => {
      try {
        const res = await fetch("/api/v1/movies/latest", {
          credentials: "include",
        });

        const data = await res.json()

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          )
        }

        return data.movies


      } catch (err) {
        console.error(err)
        throw new Error("a network occured while fetching movies")
      }
    }
  })


  return (
    <section className='py-8'>
      <div className='max-w-7xl mx-auto px-4 sm:px-6 lg:px-8'>
        <div className='flex items-center justify-between h-10 mb-6'>
          <h2 className='text-2xl font-bold text-white'>Latest Movies</h2>
          <div className='w-5 h-5 flex items-center justify-center'>
            {isPending && <Spinner />}
          </div>
        </div>

        <div className='h-10'>
          <ErrorWarning error={error?.message || ''} isVisible={isError} />
        </div>

        <div className='min-h-[200px]'>
          {!isPending && movies?.length === 0 && (
            <div className='h-full flex items-center justify-center'>
              <p className='text-blue-200'>No movies available</p>
            </div>
          )}

          <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6'>
            {isPending
              ? // Loading placeholders
                Array.from({ length: 12 }).map((_, index) => (
                  <div
                    key={`placeholder-${index}`}
                    className='aspect-[2/3] rounded-xl bg-slate-800/50 animate-pulse'
                  />
                ))
              : movies?.map(movie => (
                  <MovieCard key={movie.id} movie={movie} imgLoading='eager' />
                ))}
          </div>
        </div>
      </div>
    </section>
  );
}
