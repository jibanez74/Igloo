import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Film, Star, Clock, Calendar, Play } from "lucide-react";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import {
  TMDB_IMAGE_BASE,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import CastSection from "@/components/CastSection";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { buttonVariants } from "@/components/ui/button";
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
        className='border-red-500/20 bg-red-500/10 text-red-400'
      >
        <AlertCircle className="size-4" aria-hidden="true" />
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
      <div className='py-12 text-center'>
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
      <div className='relative z-10 -mt-32' aria-hidden='true'>
        <div className='flex flex-col gap-6 md:flex-row lg:gap-8'>
          {/* Poster skeleton */}
          <div className='mx-auto shrink-0 md:mx-0'>
            <div className='aspect-2/3 w-48 rounded-xl bg-slate-800 md:w-64 lg:w-72' />
          </div>

          {/* Info skeleton */}
          <div className='flex-1 space-y-4'>
            <div className='h-10 w-3/4 rounded-sm bg-slate-800' />
            <div className='h-5 w-1/2 rounded-sm bg-slate-800' />
            <div className='flex gap-2'>
              <div className='h-6 w-20 rounded-full bg-slate-800' />
              <div className='h-6 w-24 rounded-full bg-slate-800' />
              <div className='h-6 w-16 rounded-full bg-slate-800' />
            </div>
            <div className='space-y-2 pt-4'>
              <div className='h-4 w-full rounded-sm bg-slate-800' />
              <div className='h-4 w-full rounded-sm bg-slate-800' />
              <div className='h-4 w-3/4 rounded-sm bg-slate-800' />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function MovieDetailsContent({ movie }: { movie: MovieDetailsType }) {
  const backdropUrl = movie.backdrop_path
    ? `${TMDB_IMAGE_BASE}/${TMDB_BACKDROP_SIZE}${movie.backdrop_path}`
    : null;

  const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}/${TMDB_POSTER_SIZE}${movie.poster_path}`
    : null;

  const releaseYear = movie.release_date
    ? new Date(movie.release_date).getFullYear()
    : null;

  // React 19 document metadata - dynamic based on movie
  const pageTitle = releaseYear
    ? `${movie.title} (${releaseYear}) - Igloo`
    : `${movie.title} - Igloo`;
  const pageDescription = movie.overview
    ? movie.overview.slice(0, 160)
    : `Watch ${movie.title} in your Igloo media library.`;

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
    if (score >= 7) return "bg-amber-500 text-slate-900"; // High rating - gold
    if (score >= 5) return "bg-amber-600/70 text-white"; // Medium rating - darker gold
    return "bg-slate-500 text-white"; // Low rating - gray
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
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      {/* Skip navigation for screen readers */}
      <nav
        aria-label='Skip to section'
        className='sr-only focus-within:not-sr-only'
      >
        <ul className='mb-4 flex gap-2'>
          <li>
            <a
              href='#movie-title'
              className='rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none'
            >
              Skip to movie info
            </a>
          </li>
          <li>
            <a
              href='#overview-heading'
              className='rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none'
            >
              Skip to overview
            </a>
          </li>
          {movie.credits?.cast && movie.credits.cast.length > 0 && (
            <li>
              <a
                href='#cast-heading'
                className='rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none'
              >
                Skip to cast
              </a>
            </li>
          )}
          <li>
            <a
              href='#details-heading'
              className='rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none'
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
            className='aspect-21/9 w-full object-cover object-top'
          />
        ) : (
          <div className='aspect-21/9 w-full bg-slate-800' aria-hidden='true' />
        )}
        <div
          className='absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent'
          aria-hidden='true'
        />
      </header>

      {/* Main content */}
      <div className='relative z-10 -mt-32'>
        <div className='flex flex-col gap-6 md:flex-row lg:gap-8'>
          {/* Poster */}
          <figure className='mx-auto shrink-0 md:mx-0'>
            <div className='w-48 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 md:w-64 lg:w-72'>
              {posterUrl ? (
                <img
                  src={posterUrl}
                  alt={`Movie poster for ${movie.title}`}
                  className='aspect-2/3 w-full object-cover'
                />
              ) : (
                <div
                  className='flex aspect-2/3 w-full items-center justify-center bg-slate-800'
                  role='img'
                  aria-label='No poster available'
                >
                  <Film className="size-12 text-slate-600" aria-hidden="true" />
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
              className='text-3xl font-bold text-white outline-none md:text-4xl lg:text-5xl'
            >
              {movie.title}
              {releaseYear && (
                <span className='ml-3 font-normal text-slate-400'>
                  (<time dateTime={movie.release_date}>{releaseYear}</time>)
                </span>
              )}
            </h1>

            {/* Tagline */}
            {movie.tagline && (
              <p className='mt-2 text-lg text-slate-400 italic'>
                <q>{movie.tagline}</q>
              </p>
            )}

            {/* Meta info row */}
            <ul
              className='mt-4 flex list-none flex-wrap items-center gap-3'
              aria-label='Movie details'
            >
              {/* Rating badge */}
              {movie.vote_average > 0 && (
                <li
                  className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${getRatingColor(
                    movie.vote_average
                  )}`}
                  aria-label={`User rating: ${movie.vote_average.toFixed(
                    1
                  )} out of 10`}
                >
                  <Star className="size-3.5 fill-current" aria-hidden="true" />
                  <span aria-hidden='true'>
                    {movie.vote_average.toFixed(1)}
                  </span>
                </li>
              )}

              {/* Runtime */}
              {runtime && (
                <li className='flex items-center gap-1.5 text-slate-300'>
                  <Clock className="size-4 text-slate-400" aria-hidden="true" />
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
                <li className='flex items-center gap-1.5 text-slate-300'>
                  <Calendar className="size-4 text-slate-400" aria-hidden="true" />
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
                className='mt-4 flex list-none flex-wrap gap-2'
                aria-label={`Genres: ${movie.genres
                  .map(g => g.name)
                  .join(", ")}`}
              >
                {movie.genres.map((genre, index) => (
                  <li
                    key={genre.id}
                    tabIndex={0}
                    role='listitem'
                    aria-posinset={index + 1}
                    aria-setsize={movie.genres!.length}
                    className='rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm outline-none focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-400/50'
                  >
                    {genre.name}
                  </li>
                ))}
              </ul>
            )}

            {/* Trailer button with route masking */}
            {trailer && (
              <Link
                to='/trailer'
                search={{
                  mediaType: "movie",
                  mediaId: movie.id,
                  returnTo: "/movies/in-theaters/$id",
                }}
                mask={{
                  to: "/movies/in-theaters/$id",
                  params: { id: String(movie.id) },
                }}
                className={
                  buttonVariants({ variant: "accent", size: "lg" }) + " mt-6"
                }
              >
                <Play className="size-4 fill-current" aria-hidden="true" />
                Play Trailer
              </Link>
            )}

            {/* Overview */}
            <section className='mt-6' aria-labelledby='overview-heading'>
              <h2
                id='overview-heading'
                tabIndex={-1}
                className='mb-2 text-xl font-semibold text-white outline-none'
              >
                Overview
              </h2>
              <p className='leading-relaxed text-slate-300'>
                {movie.overview || "No overview available."}
              </p>
            </section>

            {/* Crew highlights */}
            {(director || (writers && writers.length > 0)) && (
              <section className='mt-6' aria-labelledby='crew-heading'>
                <h2 id='crew-heading' className='sr-only'>
                  Key Crew
                </h2>
                <dl className='grid grid-cols-2 gap-4 sm:grid-cols-3'>
                  {director && (
                    <div
                      tabIndex={0}
                      className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
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
                      className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
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
          className='mt-10 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4'
          aria-labelledby='details-heading'
        >
          <h2
            id='details-heading'
            tabIndex={-1}
            className='sr-only outline-none'
          >
            Additional Details
          </h2>
          <dl className='grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4'>
            <div
              tabIndex={0}
              className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
              role='group'
              aria-label={`Status: ${movie.status || "Unknown"}`}
            >
              <dt className='text-sm font-semibold tracking-wide text-amber-300/70 uppercase'>
                Status
              </dt>
              <dd className='mt-1 text-white'>{movie.status || "-"}</dd>
            </div>
            <div
              tabIndex={0}
              className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
              role='group'
              aria-label={`Original Language: ${
                movie.original_language?.toUpperCase() || "Unknown"
              }`}
            >
              <dt className='text-sm font-semibold tracking-wide text-amber-300/70 uppercase'>
                Original Language
              </dt>
              <dd className='mt-1 text-white uppercase'>
                {movie.original_language || "-"}
              </dd>
            </div>
            <div
              tabIndex={0}
              className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
              role='group'
              aria-label={`Budget: ${formatCurrency(movie.budget)}`}
            >
              <dt className='text-sm font-semibold tracking-wide text-amber-300/70 uppercase'>
                Budget
              </dt>
              <dd className='mt-1 text-white'>
                <data value={movie.budget}>{formatCurrency(movie.budget)}</data>
              </dd>
            </div>
            <div
              tabIndex={0}
              className='-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50'
              role='group'
              aria-label={`Revenue: ${formatCurrency(movie.revenue)}`}
            >
              <dt className='text-sm font-semibold tracking-wide text-amber-300/70 uppercase'>
                Revenue
              </dt>
              <dd className='mt-1 text-white'>
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
