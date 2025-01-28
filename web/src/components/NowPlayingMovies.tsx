import { useState, useEffect } from "react";
import MovieCard from "./MovieCard";
import ErrorWarning from "./ErrorWarning";
import Spinner from "./Spinner";
import type { SimpleMovie } from "@/types/Movie";

export default function NowPlayingMovies() {
  const [movies, setMovies] = useState<SimpleMovie[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const [isErrorVisible, setIsErrorVisible] = useState(false);

  useEffect(() => {
    const fetchNowPlayingMovies = async () => {
      try {
        const res = await fetch("/api/v1/movies/now-playing");
        if (!res.ok) {
          throw new Error("Failed to fetch now playing movies");
        }
        const data = await res.json();
        setMovies(data.movies);
        setError("");
      } catch (err) {
        setError(err instanceof Error ? err.message : "An error occurred");
        setIsErrorVisible(true);
      } finally {
        setIsLoading(false);
      }
    };

    fetchNowPlayingMovies();
  }, []);

  return (
    <section className='py-8 bg-slate-900/50 backdrop-blur-sm'>
      <div className='max-w-7xl mx-auto px-4 sm:px-6 lg:px-8'>
        <div className='flex items-center justify-between h-10 mb-6'>
          <h2 className='text-2xl font-bold text-white'>
            Now Playing in Theaters
          </h2>
          <div className='w-5 h-5 flex items-center justify-center'>
            {isLoading && <Spinner />}
          </div>
        </div>

        <div className='h-10'>
          <ErrorWarning error={error} isVisible={isErrorVisible} />
        </div>

        <div className='min-h-[200px]'>
          {!isLoading && movies.length === 0 && (
            <div className='h-full flex items-center justify-center'>
              <p className='text-blue-200'>No movies available</p>
            </div>
          )}

          <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6'>
            {isLoading
              ? // Loading placeholders
                Array.from({ length: 12 }).map((_, index) => (
                  <div
                    key={`placeholder-${index}`}
                    className='aspect-[2/3] rounded-xl bg-slate-800/50 animate-pulse'
                  />
                ))
              : movies.map(movie => (
                  <MovieCard
                    key={movie.ID}
                    movie={movie}
                    showPlayButton={false}
                  />
                ))}
          </div>
        </div>
      </div>
    </section>
  );
}
