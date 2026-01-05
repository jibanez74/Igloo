import { useState } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import {
  TMDB_IMAGE_BASE,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import CastSection from "@/components/CastSection";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import YoutubePlayer from "@/components/YoutubePlayer";
import type { MovieDetailsType } from "@/types";

export const Route = createLazyFileRoute("/_auth/movies/in-theaters/$id")({
  component: MovieDetailsPage,
});

function MovieDetailsPage() {
  const { id } = Route.useParams();
  const movieId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(movieDetailsQueryOpts(movieId));

  if (isError || (data && data.error)) {
    return (
      <Alert
        variant='destructive'
        className='bg-red-500/10 border-red-500/20 text-red-400'
      >
        <i className='fa-solid fa-circle-exclamation' aria-hidden='true'></i>
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>
          {data?.message ||
            "Failed to load movie details. Please try again later."}
        </AlertDescription>
      </Alert>
    );
  }

  const movie = data?.data?.movie;

  // Show skeleton while loading to prevent layout shift
  if (isPending) {
    return <MovieDetailsSkeleton />;
  }

  if (!movie) {
    return (
      <div className='text-center py-12'>
        <h2 className='text-xl font-semibold text-slate-300'>
          Movie not found
        </h2>
      </div>
    );
  }

  return <MovieDetailsContent movie={movie} />;
}

function MovieDetailsSkeleton() {
  return (
    <div
      className='animate-pulse'
      role='status'
      aria-label='Loading movie details'
    >
      <span className='sr-only'>Loading movie details...</span>

      {/* Backdrop skeleton */}
      <div
        className='relative -mx-4 sm:-mx-6 lg:-mx-8 xl:-mx-12'
        aria-hidden='true'
      >
        <div className='aspect-21/9 bg-slate-800' />
        <div className='absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent' />
      </div>

      {/* Content skeleton */}
      <div className='-mt-32 relative z-10' aria-hidden='true'>
        <div className='flex flex-col md:flex-row gap-6 lg:gap-8'>
          {/* Poster skeleton */}
          <div className='shrink-0 mx-auto md:mx-0'>
            <div className='w-48 md:w-64 lg:w-72 aspect-2/3 rounded-xl bg-slate-800' />
          </div>

          {/* Info skeleton */}
          <div className='flex-1 space-y-4'>
            <div className='h-10 bg-slate-800 rounded w-3/4' />
            <div className='h-5 bg-slate-800 rounded w-1/2' />
            <div className='flex gap-2'>
              <div className='h-6 w-20 bg-slate-800 rounded-full' />
              <div className='h-6 w-24 bg-slate-800 rounded-full' />
              <div className='h-6 w-16 bg-slate-800 rounded-full' />
            </div>
            <div className='space-y-2 pt-4'>
              <div className='h-4 bg-slate-800 rounded w-full' />
              <div className='h-4 bg-slate-800 rounded w-full' />
              <div className='h-4 bg-slate-800 rounded w-3/4' />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function MovieDetailsContent({ movie }: { movie: MovieDetailsType }) {
  const [trailerOpen, setTrailerOpen] = useState(false);

  const backdropUrl = movie.backdrop_path
    ? `${TMDB_IMAGE_BASE}/${TMDB_BACKDROP_SIZE}${movie.backdrop_path}`
    : null;

    const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}/${TMDB_POSTER_SIZE}${movie.poster_path}`
    : null;

  const releaseYear = movie.release_date
    ? new Date(movie.release_date).getFullYear()
    : null;

  const runtime = movie.runtime
    ? `${Math.floor(movie.runtime / 60)}h ${movie.runtime % 60}m`
    : null;

  const director = movie.credits?.crew?.find(c => c.job === "Director");
  const writers = movie.credits?.crew
    ?.filter(c => c.department === "Writing")
    .slice(0, 3);

  const trailer = movie.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube"
  );

  const getRatingColor = (score: number) => {
    if (score >= 7) return "bg-teal-500 text-white"; // Northern lights green-blue
    if (score >= 5) return "bg-sky-500 text-white"; // Glacier blue
    return "bg-slate-500 text-white"; // Frozen gray
  };

  const formatCurrency = (amount: number) => {
    if (!amount) return "-";
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
      maximumFractionDigits: 0,
    }).format(amount);
  };

  return (
    <article aria-labelledby='movie-title'>
      {/* Skip navigation for screen readers */}
      <nav
        aria-label='Skip to section'
        className='sr-only focus-within:not-sr-only'
      >
        <ul className='flex gap-2 mb-4'>
          <li>
            <a
              href='#movie-title'
              className='text-cyan-400 underline focus:outline-none focus:ring-2 focus:ring-cyan-400 rounded px-2 py-1'
            >
              Skip to movie info
            </a>
          </li>
          <li>
            <a
              href='#overview-heading'
              className='text-cyan-400 underline focus:outline-none focus:ring-2 focus:ring-cyan-400 rounded px-2 py-1'
            >
              Skip to overview
            </a>
          </li>
          {movie.credits?.cast && movie.credits.cast.length > 0 && (
            <li>
              <a
                href='#cast-heading'
                className='text-cyan-400 underline focus:outline-none focus:ring-2 focus:ring-cyan-400 rounded px-2 py-1'
              >
                Skip to cast
              </a>
            </li>
          )}
          <li>
            <a
              href='#details-heading'
              className='text-cyan-400 underline focus:outline-none focus:ring-2 focus:ring-cyan-400 rounded px-2 py-1'
            >
              Skip to details
            </a>
          </li>
        </ul>
      </nav>

      {/* Hero backdrop */}
      <header className='relative -mx-4 sm:-mx-6 lg:-mx-8 xl:-mx-12'>
        {backdropUrl ? (
          <img
            src={backdropUrl}
            alt=''
            aria-hidden='true'
            className='w-full aspect-21/9 object-cover object-top'
          />
        ) : (
          <div className='w-full aspect-21/9 bg-slate-800' aria-hidden='true' />
        )}
        <div
          className='absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent'
          aria-hidden='true'
        />
      </header>

      {/* Main content */}
      <div className='-mt-32 relative z-10'>
        <div className='flex flex-col md:flex-row gap-6 lg:gap-8'>
          {/* Poster */}
          <figure className='shrink-0 mx-auto md:mx-0'>
            <div className='w-48 md:w-64 lg:w-72 rounded-xl overflow-hidden shadow-2xl shadow-cyan-500/10 border border-cyan-500/20'>
              {posterUrl ? (
                <img
                  src={posterUrl}
                  alt={`Movie poster for ${movie.title}`}
                  className='w-full aspect-2/3 object-cover'
                />
              ) : (
                <div
                  className='w-full aspect-2/3 bg-slate-800 flex items-center justify-center'
                  role='img'
                  aria-label='No poster available'
                >
                  <i
                    className='fa-solid fa-film text-slate-600 text-5xl'
                    aria-hidden='true'
                  />
                </div>
              )}
            </div>
          </figure>

          {/* Movie info */}
          <div className='flex-1'>
            {/* Title and year */}
            <h1
              id='movie-title'
              tabIndex={-1}
              className='text-3xl md:text-4xl lg:text-5xl font-bold text-white outline-none'
            >
              {movie.title}
              {releaseYear && (
                <span className='text-slate-400 font-normal ml-3'>
                  (<time dateTime={movie.release_date}>{releaseYear}</time>)
                </span>
              )}
            </h1>

            {/* Tagline */}
            {movie.tagline && (
              <p className='text-lg text-slate-400 italic mt-2'>
                <q>{movie.tagline}</q>
              </p>
            )}

            {/* Meta info row */}
            <ul
              className='flex flex-wrap items-center gap-3 mt-4 list-none'
              aria-label='Movie details'
            >
              {/* Rating badge */}
              {movie.vote_average > 0 && (
                <li
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full font-bold ${getRatingColor(movie.vote_average)}`}
                  aria-label={`User rating: ${movie.vote_average.toFixed(1)} out of 10`}
                >
                  <i className='fa-solid fa-star text-sm' aria-hidden='true' />
                  <span aria-hidden='true'>
                    {movie.vote_average.toFixed(1)}
                  </span>
                </li>
              )}

              {/* Runtime */}
              {runtime && (
                <li className='text-slate-300 flex items-center gap-1.5'>
                  <i
                    className='fa-regular fa-clock text-slate-500'
                    aria-hidden='true'
                  />
                  <time
                    dateTime={`PT${movie.runtime}M`}
                    aria-label={`Duration: ${runtime}`}
                  >
                    {runtime}
                  </time>
                </li>
              )}

              {/* Release date */}
              {movie.release_date && (
                <li className='text-slate-300 flex items-center gap-1.5'>
                  <i
                    className='fa-regular fa-calendar text-slate-500'
                    aria-hidden='true'
                  />
                  <time dateTime={movie.release_date}>
                    {new Date(movie.release_date).toLocaleDateString("en-US", {
                      year: "numeric",
                      month: "short",
                      day: "numeric",
                    })}
                  </time>
                </li>
              )}
            </ul>

            {/* Genres */}
            {movie.genres && movie.genres.length > 0 && (
              <ul
                className='flex flex-wrap gap-2 mt-4 list-none'
                aria-label={`Genres: ${movie.genres.map(g => g.name).join(", ")}`}
              >
                {movie.genres.map((genre, index) => (
                  <li
                    key={genre.id}
                    tabIndex={0}
                    role='listitem'
                    aria-posinset={index + 1}
                    aria-setsize={movie.genres!.length}
                    className='px-3 py-1 bg-slate-800/80 text-cyan-200 text-sm rounded-full border border-cyan-500/30 backdrop-blur-sm outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50 focus-visible:border-cyan-400'
                  >
                    {genre.name}
                  </li>
                ))}
              </ul>
            )}

            {/* Trailer button */}
            {trailer && (
              <>
                <Button
                  variant='accent'
                  size='lg'
                  onClick={() => setTrailerOpen(true)}
                  className='mt-6'
                  aria-haspopup='dialog'
                >
                  <i className='fa-solid fa-play' aria-hidden='true' />
                  Play Trailer
                </Button>
                <YoutubePlayer
                  videoKey={trailer.key}
                  title={`${movie.title} - Trailer`}
                  open={trailerOpen}
                  onOpenChange={setTrailerOpen}
                />
              </>
            )}

            {/* Overview */}
            <section className='mt-6' aria-labelledby='overview-heading'>
              <h2
                id='overview-heading'
                tabIndex={-1}
                className='text-xl font-semibold text-white mb-2 outline-none'
              >
                Overview
              </h2>
              <p className='text-slate-300 leading-relaxed'>
                {movie.overview || "No overview available."}
              </p>
            </section>

            {/* Crew highlights */}
            {(director || (writers && writers.length > 0)) && (
              <section className='mt-6' aria-labelledby='crew-heading'>
                <h2 id='crew-heading' className='sr-only'>
                  Key Crew
                </h2>
                <dl className='grid grid-cols-2 sm:grid-cols-3 gap-4'>
                  {director && (
                    <div
                      tabIndex={0}
                      className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
                      role='group'
                      aria-label={`Director: ${director.name}`}
                    >
                      <dt className='text-sm text-slate-400'>Director</dt>
                      <dd className='font-semibold text-white'>
                        {director.name}
                      </dd>
                    </div>
                  )}
                  {writers?.map(writer => (
                    <div
                      key={writer.id}
                      tabIndex={0}
                      className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
                      role='group'
                      aria-label={`${writer.job}: ${writer.name}`}
                    >
                      <dt className='text-sm text-slate-400'>{writer.job}</dt>
                      <dd className='font-semibold text-white'>
                        {writer.name}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            )}
          </div>
        </div>

        {/* Cast section */}
        {movie.credits?.cast && <CastSection cast={movie.credits.cast} />}

        {/* Additional details */}
        <section
          className='mt-10 p-4 bg-slate-800/30 rounded-xl border border-cyan-500/10'
          aria-labelledby='details-heading'
        >
          <h2
            id='details-heading'
            tabIndex={-1}
            className='sr-only outline-none'
          >
            Additional Details
          </h2>
          <dl className='grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-6'>
            <div
              tabIndex={0}
              className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
              role='group'
              aria-label={`Status: ${movie.status || "Unknown"}`}
            >
              <dt className='text-sm font-semibold text-cyan-300/70 uppercase tracking-wide'>
                Status
              </dt>
              <dd className='text-white mt-1'>{movie.status || "-"}</dd>
            </div>
            <div
              tabIndex={0}
              className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
              role='group'
              aria-label={`Original Language: ${movie.original_language?.toUpperCase() || "Unknown"}`}
            >
              <dt className='text-sm font-semibold text-cyan-300/70 uppercase tracking-wide'>
                Original Language
              </dt>
              <dd className='text-white mt-1 uppercase'>
                {movie.original_language || "-"}
              </dd>
            </div>
            <div
              tabIndex={0}
              className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
              role='group'
              aria-label={`Budget: ${formatCurrency(movie.budget)}`}
            >
              <dt className='text-sm font-semibold text-cyan-300/70 uppercase tracking-wide'>
                Budget
              </dt>
              <dd className='text-white mt-1'>
                <data value={movie.budget}>{formatCurrency(movie.budget)}</data>
              </dd>
            </div>
            <div
              tabIndex={0}
              className='p-2 -m-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50'
              role='group'
              aria-label={`Revenue: ${formatCurrency(movie.revenue)}`}
            >
              <dt className='text-sm font-semibold text-cyan-300/70 uppercase tracking-wide'>
                Revenue
              </dt>
              <dd className='text-white mt-1'>
                <data value={movie.revenue}>
                  {formatCurrency(movie.revenue)}
                </data>
              </dd>
            </div>
          </dl>
        </section>
      </div>
    </article>
  );
}
