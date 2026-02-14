import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { AlertCircle, Film, Play } from "lucide-react";
import { latestMoviesQueryOpts } from "@/lib/query-opts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import type { LatestMovieType } from "@/types";

function LatestMovieCard({ movie }: { movie: LatestMovieType }) {
  const { id, title, poster, year } = movie;

  return (
    <article className="group relative min-w-0 animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 transition-all duration-300 fade-in focus-within:ring-2 focus-within:ring-cyan-400 focus-within:ring-offset-2 focus-within:ring-offset-slate-900 hover:-translate-y-1 hover:border-cyan-500/50 hover:shadow-xl hover:shadow-cyan-500/20">
      <Link
        to="/movies/play/$id"
        params={{ id: String(id) }}
        className="block rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-900"
        aria-label={`Play ${title}${year ? `, ${year}` : ""}`}
      >
        {/* Poster with 2:3 aspect ratio (standard movie poster) */}
        <div className="relative aspect-2/3 bg-slate-800">
          {poster ? (
            <img
              src={poster}
              alt=""
              width={500}
              height={750}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Film className="size-10 text-slate-600" aria-hidden="true" />
            </div>
          )}
          {/* Play icon overlay on hover */}
          <div className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 transition-opacity group-hover:opacity-100">
            <span className="flex h-14 w-14 items-center justify-center rounded-full bg-cyan-500/90 text-slate-900 shadow-lg">
              <Play className="size-7 fill-current" aria-hidden="true" />
            </span>
          </div>
          {/* Gradient overlay for text readability */}
          <div className="absolute inset-x-0 bottom-0 h-24 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
        </div>
        {/* Movie info */}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <h3 className="truncate text-sm font-semibold text-white drop-shadow-lg">
            {title}
          </h3>
          {year != null && (
            <p className="mt-0.5 text-xs text-slate-300 drop-shadow-lg">
              {year}
            </p>
          )}
        </div>
      </Link>
    </article>
  );
}

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
          className="flex min-h-[200px] items-center justify-center py-12 sm:min-h-[280px]"
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
              <LatestMovieCard key={movie.id} movie={movie} />
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
