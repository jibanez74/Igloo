import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Film } from "lucide-react";
import { latestMoviesQueryOpts } from "@/lib/query-opts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import MovieCard from "@/components/MovieCard";


export default function LatestMovies() {
  const { data, isPending } = useQuery(latestMoviesQueryOpts());

  const movies = data && !data.error ? (data.data?.movies ?? []) : [];
  const hasError = data && data.error;

  const getAnnouncementMessage = () => {
    if (isPending) return undefined;
    if (hasError) return data.message || "Failed to load movies";
    if (movies.length === 0) return "No movies in your library";
    return `${movies.length} movies loaded`;
  };

  return (
    <section
      role="region"
      aria-labelledby="recent-movies"
      aria-label="Recently Added Movies"
      className="mt-8 md:mt-10"
    >
      <LiveAnnouncer message={getAnnouncementMessage()} />

      <h2
        id="recent-movies"
        className="mb-4 text-xl font-semibold tracking-tight text-white md:text-2xl"
      >
        Recently Added Movies
      </h2>

      {isPending ? (
        <div
          className="flex min-h-50 items-center justify-center py-12 sm:min-h-70"
          role="status"
          aria-label="Loading movies..."
        >
          <Spinner className="size-8 text-cyan-400" />
          <span className="sr-only">Loading movies...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant="destructive"
          className="border-red-500/20 bg-red-500/10 text-red-400"
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
            className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-slate-800 focus:px-4 focus:py-2 focus:text-white"
            aria-label={`Recently Added Movies section, ${movies.length} movies`}
          >
            Recently Added Movies - {movies.length} movies
          </span>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 sm:gap-4 md:grid-cols-4 lg:grid-cols-6">
            {movies.map(movie => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </>
      ) : (
        <div className="py-12 text-center sm:py-16">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full border border-cyan-500/20 bg-slate-800">
            <Film className="size-6 text-cyan-600" aria-hidden="true" />
          </div>
          <h3 className="mb-2 text-lg font-semibold text-slate-300">
            No Movies Yet
          </h3>
          <p className="mx-auto max-w-md px-4 text-slate-400 sm:px-0">
            Your movie library is empty. Add a movies folder in settings and run
            a scan to get started.
          </p>
        </div>
      )}
    </section>
  );
}
