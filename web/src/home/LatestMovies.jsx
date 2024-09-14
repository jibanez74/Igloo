import { useQuery } from "@tanstack/react-query";
import { getLatestMovies } from "./httpHome";
import Spinner from "../shared/Spinner";
import MovieCard from "../shared/MovieCard";
import Alert from "../shared/Alert";

export default function LatestMovies() {
  const {
    data: movies,
    isPending,
    error,
    isError,
  } = useQuery({
    queryKey: ["recent-movies"],
    queryFn: getLatestMovies,
  });

  return (
    <>
      <h2 className='text-2xl font-bold mb-4'>Latest Movies</h2>

      {isPending ? (
        <Spinner />
      ) : isError ? (
        <Alert
          title='Error'
          msg='unable to fetch movies'
          time={6000}
          variant='danger'
        />
      ) : (
        <div className='grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4'>
          {movies.map(m => (
            <MovieCard key={m._id} movie={m} />
          ))}
        </div>
      )}
    </>
  );
}
