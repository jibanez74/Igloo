import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Film,
  Star,
  Clock,
  Calendar,
  Play,
  Heart,
  MoreVertical,
  Users,
  Info,
  Radio,
  Settings2,
} from "lucide-react";
import { toast } from "sonner";
import {
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
import {
  TMDB_BACKDROP_SIZE,
  TMDB_IMAGE_BASE,
  TMDB_LOGO_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { formatCurrency, formatDate } from "@/lib/format";
import { unwrapFloat, unwrapInt, unwrapString } from "@/lib/nullable";
import MediaNotFound from "@/components/MediaNotFound";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import CastSection from "@/components/CastSection";
import TechnicalDetailsDialog from "@/components/TechnicalDetailsDialog";
import PlaybackSettingsDialog from "@/components/PlaybackSettingsDialog";
import {
  DEFAULT_PLAYBACK_SETTINGS,
  type PlaybackSettings,
} from "@/lib/playback";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type {
  CastMemberType,
  LibraryMovieDetailsResponse,
} from "@/types";

export const Route = createFileRoute("/_auth/movies/$id/")({
  loader: async ({ context, params }) => {
    const movieId = parseInt(params.id, 10);
    if (!Number.isNaN(movieId) && movieId > 0) {
      await Promise.all([
        context.queryClient.ensureQueryData(
          libraryMovieDetailsQueryOpts(movieId),
        ),
        context.queryClient.ensureQueryData(
          movieTechnicalDetailsQueryOpts(movieId),
        ),
      ]);
    }
  },
  component: MovieDetailsPage,
});

function buildTmdbUrl(path: string | null, size: string): string {
  if (!path) return "";
  return `${TMDB_IMAGE_BASE}/${size}${path.startsWith("/") ? path : `/${path}`}`;
}

function libraryCastToCastSection(cast: LibraryMovieDetailsResponse["cast"]): CastMemberType[] {
  return cast.map(c => ({
    id: c.id,
    name: c.artist_name,
    character: c.character,
    profile_path: unwrapString(c.artist_profile) ?? "",
    order: c.cast_order,
  }));
}

function MovieDetailsPage() {
  const { id } = Route.useParams();
  const movieId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(
    libraryMovieDetailsQueryOpts(movieId),
  );

  const payload = data?.data;
  const movie = payload?.movie;

  if (isError || (data && data.error)) {
    return (
      <MediaNotFound
        message={
          data?.message ||
          "Failed to load movie details. Please try again later."
        }
      />
    );
  }

  if (isPending) {
    return <MovieDetailsSkeleton />;
  }

  if (!movie || !payload) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Movie not found
        </h2>
      </div>
    );
  }

  return (
    <LibraryMovieDetailsContent
      movieId={movieId}
      payload={payload}
    />
  );
}

function LibraryMovieDetailsContent({
  movieId,
  payload,
}: {
  movieId: number;
  payload: LibraryMovieDetailsResponse;
}) {
  const [technicalDetailsOpen, setTechnicalDetailsOpen] = useState(false);
  const [playbackSettingsOpen, setPlaybackSettingsOpen] = useState(false);
  const [playbackSettings, setPlaybackSettings] = useState<PlaybackSettings>(
    DEFAULT_PLAYBACK_SETTINGS,
  );

  const { movie, cast, crew, genres, production_companies, extra_videos } =
    payload;

  const posterPath = unwrapString(movie.poster_path);
  const backdropPath = unwrapString(movie.backdrop_path);
  const tagLine = unwrapString(movie.tag_line);
  const releaseDateStr = unwrapString(movie.release_date);
  const overview = unwrapString(movie.overview);
  const runTimeMins = unwrapInt(movie.run_time);
  const year = unwrapInt(movie.year);
  const criticRating = unwrapFloat(movie.critic_rating);
  const audienceRating = unwrapFloat(movie.audience_rating);
  const certification = unwrapString(movie.certification);
  const language = unwrapString(movie.language);
  const budget = unwrapFloat(movie.budget);
  const revenue = unwrapFloat(movie.revenue);

  const posterUrl = buildTmdbUrl(posterPath, TMDB_POSTER_SIZE);
  const backdropUrl = buildTmdbUrl(backdropPath, TMDB_BACKDROP_SIZE);

  const releaseYear =
    year ?? (releaseDateStr ? new Date(releaseDateStr).getFullYear() : null);
  const pageTitle = releaseYear
    ? `${movie.title} (${releaseYear}) - Igloo`
    : `${movie.title} - Igloo`;
  const pageDescription = overview
    ? overview.slice(0, 160)
    : `Watch ${movie.title} in your Igloo media library.`;

  const runtime =
    runTimeMins != null
      ? `${Math.floor(runTimeMins / 60)}h ${runTimeMins % 60}m`
      : null;

  const director = crew.find(c => c.job === "Director");
  const writers = crew.filter(c => c.department === "Writing").slice(0, 3);

  const castForSection = libraryCastToCastSection(cast);

  const getCriticRatingColor = (score: number) => {
    if (score >= 7) return "bg-amber-500 text-slate-900";
    if (score >= 5) return "bg-amber-600/70 text-white";
    return "bg-slate-500 text-white";
  };

  const getAudienceRatingColor = (score: number) => {
    if (score >= 7) return "bg-violet-500 text-white";
    if (score >= 5) return "bg-violet-600/70 text-white";
    return "bg-slate-500 text-white";
  };

  return (
    <article aria-labelledby="movie-title">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <nav
        aria-label="Skip to section"
        className="sr-only focus-within:not-sr-only"
      >
        <ul className="mb-4 flex gap-2">
          <li>
            <a
              href="#movie-title"
              className="rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none"
            >
              Skip to movie info
            </a>
          </li>
          <li>
            <a
              href="#overview-heading"
              className="rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none"
            >
              Skip to overview
            </a>
          </li>
          {castForSection.length > 0 && (
            <li>
              <a
                href="#cast-heading"
                className="rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none"
              >
                Skip to cast
              </a>
            </li>
          )}
          <li>
            <a
              href="#details-heading"
              className="rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none"
            >
              Skip to details
            </a>
          </li>
        </ul>
      </nav>

      <header className="relative -mx-4 sm:-mx-6 lg:-mx-8 xl:-mx-12">
        {backdropUrl ? (
          <img
            src={backdropUrl}
            alt=""
            aria-hidden="true"
            className="aspect-21/9 w-full object-cover object-top"
          />
        ) : (
          <div
            className="aspect-21/9 w-full bg-slate-800"
            aria-hidden="true"
          />
        )}
        <div
          className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent"
          aria-hidden="true"
        />
      </header>

      <div className="relative z-10 -mt-32">
        <div className="flex min-w-0 flex-col gap-6 md:flex-row lg:gap-8">
          <figure className="mx-auto min-w-0 shrink-0 md:mx-0">
            <div className="w-48 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 md:w-64 lg:w-72">
              {posterUrl ? (
                <img
                  src={posterUrl}
                  alt={`Movie poster for ${movie.title}`}
                  className="aspect-2/3 w-full object-cover"
                />
              ) : (
                <div
                  className="flex aspect-2/3 w-full items-center justify-center bg-slate-800"
                  role="img"
                  aria-label="No poster available"
                >
                  <Film className="size-12 text-slate-600" aria-hidden="true" />
                </div>
              )}
            </div>
          </figure>

          <div className="min-w-0 flex-1">
            <h1
              id="movie-title"
              tabIndex={-1}
              className="text-3xl font-bold text-white outline-none md:text-4xl lg:text-5xl"
            >
              {movie.title}
              {releaseYear != null && (
                <span className="ml-3 font-normal text-slate-400">
                  (
                  <time dateTime={releaseDateStr ?? undefined}>
                    {releaseYear}
                  </time>
                  )
                </span>
              )}
            </h1>

            {tagLine && (
              <p className="mt-2 text-lg text-slate-400 italic">
                <q>{tagLine}</q>
              </p>
            )}

            <ul
              className="mt-4 flex list-none flex-wrap items-center gap-3"
              aria-label="Movie details"
            >
              {criticRating != null && criticRating > 0 && (
                <li
                  className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${getCriticRatingColor(criticRating)}`}
                  aria-label={`Critic rating: ${criticRating.toFixed(1)} out of 10`}
                >
                  <Star className="size-3.5 fill-current" aria-hidden="true" />
                  <span aria-hidden="true">{criticRating.toFixed(1)}</span>
                </li>
              )}
              {audienceRating != null && audienceRating > 0 && (
                <li
                  className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${getAudienceRatingColor(audienceRating)}`}
                  aria-label={`Audience rating: ${audienceRating.toFixed(1)} out of 10`}
                >
                  <Users className="size-3.5 fill-current" aria-hidden="true" />
                  <span aria-hidden="true">{audienceRating.toFixed(1)}</span>
                </li>
              )}
              {runtime && (
                <li className="flex items-center gap-1.5 text-slate-300">
                  <Clock className="size-4 text-slate-400" aria-hidden="true" />
                  <time
                    dateTime={runTimeMins != null ? `PT${runTimeMins}M` : undefined}
                    aria-label={`Duration: ${runtime}`}
                  >
                    {runtime}
                  </time>
                </li>
              )}
              {releaseDateStr && (
                <li className="flex items-center gap-1.5 text-slate-300">
                  <Calendar
                    className="size-4 text-slate-400"
                    aria-hidden="true"
                  />
                  <time dateTime={releaseDateStr}>
                    {formatDate(releaseDateStr)}
                  </time>
                </li>
              )}
            </ul>

            {genres.length > 0 && (
              <ul
                className="mt-4 flex list-none flex-wrap gap-2"
                aria-label={`Genres: ${genres.map(g => g.tag).join(", ")}`}
              >
                {genres.map((genre, index) => (
                  <li
                    key={genre.id}
                    tabIndex={0}
                    role="listitem"
                    aria-posinset={index + 1}
                    aria-setsize={genres.length}
                    className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm outline-none focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-400/50"
                  >
                    {genre.tag}
                  </li>
                ))}
              </ul>
            )}

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <Link
                to="/movies/$id/play"
                params={{ id: String(movieId) }}
                search={{
                  mode: playbackSettings.mode,
                  audio_track: playbackSettings.audioTrack,
                }}
                className={buttonVariants({ variant: "accent", size: "lg" })}
              >
                <Play className="size-4 fill-current" aria-hidden="true" />
                Play
              </Link>
              <button
                type="button"
                className={buttonVariants({ variant: "outline", size: "lg" })}
                aria-label="Add to likes"
              >
                <Heart className="size-4" aria-hidden="true" />
                Like
              </button>
              <DropdownMenu>
                <DropdownMenuTrigger
                  className={buttonVariants({ variant: "outline", size: "lg" })}
                  aria-label="More options"
                >
                  <MoreVertical className="size-4" aria-hidden="true" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  <DropdownMenuItem onSelect={() => setPlaybackSettingsOpen(true)}>
                    <Settings2 className="size-4" aria-hidden="true" />
                    Playback Settings
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => toast.info("Coming soon")}>
                    <Radio className="size-4" aria-hidden="true" />
                    Watch Together
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => setTechnicalDetailsOpen(true)}>
                    <Info className="size-4" aria-hidden="true" />
                    Technical Details
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <PlaybackSettingsDialog
                movieId={movieId}
                open={playbackSettingsOpen}
                onOpenChange={setPlaybackSettingsOpen}
                settings={playbackSettings}
                onSave={setPlaybackSettings}
              />

              <TechnicalDetailsDialog
                movieId={movieId}
                open={technicalDetailsOpen}
                onOpenChange={setTechnicalDetailsOpen}
              />
            </div>

            {extra_videos.length > 0 && (
              <section className="mt-6" aria-labelledby="extra-videos-heading">
                <h2
                  id="extra-videos-heading"
                  className="mb-3 text-xl font-semibold text-white"
                >
                  Extra Videos
                </h2>
                <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
                  {extra_videos.map(video => (
                    <li key={video.id}>
                      <Link
                        to="/trailer"
                        search={{
                          videoKey: video.key,
                          returnTo: `/movies/${movieId}`,
                        }}
                        className="block rounded-lg border border-amber-500/20 bg-slate-800/80 px-3 py-2 text-sm text-amber-200 transition-colors hover:border-amber-500/40 hover:bg-slate-800"
                      >
                        <span className="font-medium">{video.title}</span>
                        <span className="ml-1 text-slate-400">({video.type})</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section className="mt-6" aria-labelledby="overview-heading">
              <h2
                id="overview-heading"
                tabIndex={-1}
                className="mb-2 text-xl font-semibold text-white outline-none"
              >
                Overview
              </h2>
              <p className="leading-relaxed text-slate-300">
                {overview || "No overview available."}
              </p>
            </section>

            {(director || writers.length > 0) && (
              <section className="mt-6" aria-labelledby="crew-heading">
                <h2 id="crew-heading" className="mb-3 text-xl font-semibold text-white">
                  Key Crew
                </h2>
                <dl className="grid grid-cols-2 gap-4 sm:grid-cols-3">
                  {director && (
                    <div
                      tabIndex={0}
                      className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
                      role="group"
                      aria-label={`Director: ${director.artist_name}`}
                    >
                      <dt className="text-sm text-slate-400">Director</dt>
                      <dd className="font-semibold text-white">
                        {director.artist_name}
                      </dd>
                    </div>
                  )}
                  {writers.map(writer => (
                    <div
                      key={writer.id}
                      tabIndex={0}
                      className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
                      role="group"
                      aria-label={`${writer.job}: ${writer.artist_name}`}
                    >
                      <dt className="text-sm text-slate-400">{writer.job}</dt>
                      <dd className="font-semibold text-white">
                        {writer.artist_name}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            )}
          </div>
        </div>

        {castForSection.length > 0 && (
          <CastSection cast={castForSection} />
        )}

        <section
          className="mt-10 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4"
          aria-labelledby="details-heading"
        >
          <h2
            id="details-heading"
            tabIndex={-1}
            className="sr-only outline-none"
          >
            Additional Details
          </h2>
          <dl className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4">
            <div
              tabIndex={0}
              className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
              role="group"
              aria-label={`Certification: ${certification ?? "Not rated"}`}
            >
              <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
                Certification
              </dt>
              <dd className="mt-1 text-white">{certification ?? "-"}</dd>
            </div>
            <div
              tabIndex={0}
              className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
              role="group"
              aria-label={`Original Language: ${language ?? "Unknown"}`}
            >
              <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
                Original Language
              </dt>
              <dd className="mt-1 text-white uppercase">{language ?? "-"}</dd>
            </div>
            <div
              tabIndex={0}
              className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
              role="group"
              aria-label={`Budget: ${formatCurrency(budget ?? 0)}`}
            >
              <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
                Budget
              </dt>
              <dd className="mt-1 text-white">
                <data value={budget ?? 0}>
                  {formatCurrency(budget ?? 0)}
                </data>
              </dd>
            </div>
            <div
              tabIndex={0}
              className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
              role="group"
              aria-label={`Revenue: ${formatCurrency(revenue ?? 0)}`}
            >
              <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
                Revenue
              </dt>
              <dd className="mt-1 text-white">
                <data value={revenue ?? 0}>
                  {formatCurrency(revenue ?? 0)}
                </data>
              </dd>
            </div>
          </dl>
        </section>

        {production_companies.length > 0 && (
          <section className="mt-10" aria-labelledby="companies-heading">
            <h2
              id="companies-heading"
              className="mb-4 text-2xl font-semibold text-white"
            >
              Production Companies
            </h2>
            <ul className="flex flex-wrap items-center gap-6">
              {production_companies.map(pc => {
                const logoPath = unwrapString(pc.logo);
                const logoUrl = buildTmdbUrl(logoPath, TMDB_LOGO_SIZE);
                return (
                  <li
                    key={pc.id}
                    className="flex items-center gap-3 rounded-lg border border-amber-500/10 bg-slate-800/50 px-4 py-3"
                  >
                    {logoUrl ? (
                      <img
                        src={logoUrl}
                        alt=""
                        className="h-8 w-auto max-w-24 object-contain"
                      />
                    ) : (
                      <span className="text-sm text-slate-500">No logo</span>
                    )}
                    <span className="text-sm font-medium text-white">
                      {pc.name}
                    </span>
                  </li>
                );
              })}
            </ul>
          </section>
        )}
      </div>
    </article>
  );
}
