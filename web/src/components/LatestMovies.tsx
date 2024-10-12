import { useQuery } from "@tanstack/react-query";
import Spinner from "./Spinner";
import Alert from "./Alert";
import MovieCard from "./MovieCard";
import type { SimpleMovie } from "@/types/Movie";
import type { Res } from "@/types/Response";

export default function LatestMovies() {
  const {
    data: movies,
    isPending,
    isError,
  } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<SimpleMovie[]> => {
      const res = await fetch("/api/v1/movie/latest");

      if (!res.ok) {
        throw new Error(`${res.status} - ${res.statusText}`);
      }

      const r: Res<SimpleMovie[]> = await res.json();

      return r.data ? r.data : [];
    },
  });

  return (
    <section aria-label='Latest Movies'>
      <h2 className='text-2xl font-bold mb-4'>Latest Movies</h2>

      <div aria-live='polite'>
        {isPending ? (
          <Spinner />
        ) : isError ? (
          <Alert
            title='Error'
            msg='Unable to fetch latest movies'
            time={6000}
            variant='danger'
          />
        ) : (
          <div
            className='grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4'
            role='grid'
            aria-label='Latest movies grid'
          >
            {movies.map(m => (
              <MovieCard key={m.ID} movie={m} />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
