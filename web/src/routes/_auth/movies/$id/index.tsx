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
  Pencil,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import {
  authUserQueryOpts,
  libraryMovieDetailsQueryOpts,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
import {
  TMDB_BACKDROP_SIZE,
  TMDB_LOGO_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { formatCurrency, formatDate, formatExtraVideoType } from "@/lib/format";
import { unwrapFloat, unwrapInt, unwrapString } from "@/lib/nullable";
import MediaNotFound from "@/components/MediaNotFound";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import CastSection from "@/components/CastSection";
import TechnicalDetailsDialog from "@/components/TechnicalDetailsDialog";
import PlaybackSettingsDialog from "@/components/PlaybackSettingsDialog";
import EditMovieDialog from "@/components/EditMovieDialog";
import DeleteMovieDialog from "@/components/DeleteMovieDialog";
import {
  DEFAULT_PLAYBACK_SETTINGS,
  getDefaultMode,
  type PlaybackSettings,
} from "@/lib/playback";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type {
  AuthUser,
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

function libraryCastToCastSection(
  cast: LibraryMovieDetailsResponse["cast"],
): CastMemberType[] {
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

  return <LibraryMovieDetailsContent movieId={movieId} payload={payload} />;
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
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const { data: techData } = useQuery(movieTechnicalDetailsQueryOpts(movieId));
  const videoStream = techData?.data?.video_streams?.[0];
  const audioStream = techData?.data?.audio_streams?.[0];
  const mimeType = techData?.data?.movie?.mime_type;
  const smartDefault: PlaybackSettings = !videoStream
    ? DEFAULT_PLAYBACK_SETTINGS
    : {
        mode: getDefaultMode(
          videoStream.codec,
          audioStream?.codec ?? "",
          mimeType ?? "",
          videoStream.height,
        ),
        audioTrack: 0,
      };

  const [playbackSettings, setPlaybackSettings] = useState<PlaybackSettings>(
    DEFAULT_PLAYBACK_SETTINGS,
  );

  // Reset to smart default when techData arrives or movieId changes
  const [prevMovieId, setPrevMovieId] = useState(movieId);
  const [prevSmartMode, setPrevSmartMode] = useState(smartDefault.mode);
  if (movieId !== prevMovieId || smartDefault.mode !== prevSmartMode) {
    setPrevMovieId(movieId);
    setPrevSmartMode(smartDefault.mode);
    setPlaybackSettings(smartDefault);
  }

  const { data: userData } = useQuery(authUserQueryOpts());
  const user: AuthUser | null =
    userData?.error === false && userData.data?.user
      ? (userData.data.user as AuthUser)
      : null;

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

  const posterUrl = buildTmdbImageUrl(posterPath, TMDB_POSTER_SIZE);
  const backdropUrl = buildTmdbImageUrl(backdropPath, TMDB_BACKDROP_SIZE);

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
    <article aria-labelledby="movie-title" className="pb-6 sm:pb-10">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <nav
        aria-label="Skip to section"
        className="sr-only focus-within:not-sr-only"
      >
        <ul className="mb-4 flex flex-wrap gap-2">
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

      <header className="relative -mx-4 sm:-mx-6 lg:-mx-8">
        {backdropUrl ? (
          <img
            src={backdropUrl}
            alt=""
            aria-hidden="true"
            className="h-44 w-full object-cover object-top sm:h-52 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48 md:aspect-21/9"
          />
        ) : (
          <div
            className="h-44 w-full bg-slate-800 sm:h-52 md:aspect-21/9 md:min-h-48"
            aria-hidden="true"
          />
        )}
        <div
          className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent"
          aria-hidden="true"
        />
      </header>

      <div className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32">
        <div className="flex min-w-0 flex-col gap-6 sm:gap-8 md:flex-row md:items-start lg:gap-10">
          <figure className="mx-auto min-w-0 shrink-0 md:mx-0 md:pt-1">
            <div className="w-44 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 sm:w-52 md:w-64 lg:w-72">
              {posterUrl ? (
                <img
                  src={posterUrl}
                  alt={`Movie poster for ${movie.title}`}
                  className="block aspect-2/3 w-full rounded-xl object-cover"
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

          <div className="min-w-0 flex-1 text-center md:text-left">
            <h1
              id="movie-title"
              tabIndex={-1}
              className="flex flex-col gap-1 text-2xl font-bold text-white outline-none sm:text-3xl sm:gap-0 md:flex-row md:flex-wrap md:items-baseline md:gap-x-3 md:text-4xl lg:text-5xl"
            >
              <span className="min-w-0">{movie.title}</span>
              {releaseYear != null && (
                <span className="font-normal text-slate-400 sm:text-3xl md:text-4xl lg:text-5xl">
                  (
                  <time dateTime={releaseDateStr ?? undefined}>
                    {releaseYear}
                  </time>
                  )
                </span>
              )}
            </h1>

            {tagLine && (
              <p className="mt-2 text-base text-slate-400 italic sm:text-lg">
                <q>{tagLine}</q>
              </p>
            )}

            <ul
              className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 md:justify-start"
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
                    dateTime={
                      runTimeMins != null ? `PT${runTimeMins}M` : undefined
                    }
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
                className="mt-4 flex list-none flex-wrap justify-center gap-2 md:justify-start"
                aria-label={`Genres: ${genres.map(g => g.tag).join(", ")}`}
              >
                {genres.map(genre => (
                  <li
                    key={genre.id}
                    className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm"
                  >
                    {genre.tag}
                  </li>
                ))}
              </ul>
            )}

            <div className="mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 md:justify-start">
              <Link
                to="/movies/$id/play"
                params={{ id: String(movieId) }}
                search={{
                  mode: playbackSettings.mode,
                  audio_track: playbackSettings.audioTrack,
                }}
                className={cn(
                  buttonVariants({ variant: "accent", size: "lg" }),
                  "min-h-11 min-w-34 touch-manipulation sm:min-w-0",
                )}
              >
                <Play className="size-4 fill-current" aria-hidden="true" />
                Play
              </Link>
              <button
                type="button"
                className={cn(
                  buttonVariants({ variant: "outline", size: "lg" }),
                  "min-h-11 touch-manipulation",
                )}
                aria-label="Add to likes"
              >
                <Heart className="size-4" aria-hidden="true" />
                Like
              </button>
              <DropdownMenu>
                <DropdownMenuTrigger
                  className={cn(
                    buttonVariants({ variant: "outline", size: "lg" }),
                    "min-h-11 touch-manipulation",
                  )}
                  aria-label="More options"
                >
                  <MoreVertical className="size-4" aria-hidden="true" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  <DropdownMenuItem
                    onSelect={() => setPlaybackSettingsOpen(true)}
                  >
                    <Settings2 className="size-4" aria-hidden="true" />
                    Playback Settings
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => toast.info("Coming soon")}>
                    <Radio className="size-4" aria-hidden="true" />
                    Watch Together
                  </DropdownMenuItem>
                  {user?.is_admin && (
                    <DropdownMenuItem onSelect={() => setEditOpen(true)}>
                      <Pencil className="size-4" aria-hidden="true" />
                      Edit
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem
                    onSelect={() => setTechnicalDetailsOpen(true)}
                  >
                    <Info className="size-4" aria-hidden="true" />
                    Technical Details
                  </DropdownMenuItem>
                  {user?.is_admin && (
                    <>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        onSelect={() => setDeleteOpen(true)}
                        className="text-red-400 focus:text-red-300"
                      >
                        <Trash2 className="size-4" aria-hidden="true" />
                        Delete
                      </DropdownMenuItem>
                    </>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>

              <PlaybackSettingsDialog
                movieId={movieId}
                open={playbackSettingsOpen}
                onOpenChange={setPlaybackSettingsOpen}
                settings={playbackSettings}
                onSave={setPlaybackSettings}
              />

              {user?.is_admin && (
                <EditMovieDialog
                  movieId={movieId}
                  movie={movie}
                  open={editOpen}
                  onOpenChange={setEditOpen}
                />
              )}

              <TechnicalDetailsDialog
                movieId={movieId}
                open={technicalDetailsOpen}
                onOpenChange={setTechnicalDetailsOpen}
              />

              {user?.is_admin && (
                <DeleteMovieDialog
                  movieId={movieId}
                  movieTitle={movie.title}
                  open={deleteOpen}
                  onOpenChange={setDeleteOpen}
                />
              )}
            </div>

            {extra_videos.length > 0 && (
              <section className="mt-6" aria-labelledby="extra-videos-heading">
                <h2
                  id="extra-videos-heading"
                  className="mb-3 text-lg font-semibold text-white sm:text-xl"
                >
                  Extra Videos
                </h2>
                <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                  {extra_videos.map(video => (
                    <li key={video.id}>
                      <Link
                        to="/trailer"
                        search={{
                          videoKey: video.key,
                          returnTo: `/movies/${movieId}`,
                        }}
                        className="flex min-h-13 flex-col justify-center rounded-lg border border-amber-500/20 bg-slate-800/80 px-3 py-2.5 text-left text-sm text-amber-200 transition-colors hover:border-amber-500/40 hover:bg-slate-800 touch-manipulation sm:min-h-0"
                      >
                        <span className="font-medium leading-snug">
                          {video.title}
                        </span>
                        <span className="mt-0.5 text-slate-400">
                          ({formatExtraVideoType(video.type)})
                        </span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section className="mt-6 text-left" aria-labelledby="overview-heading">
              <h2
                id="overview-heading"
                tabIndex={-1}
                className="mb-2 text-lg font-semibold text-white outline-none sm:text-xl"
              >
                Overview
              </h2>
              <p className="text-[15px] leading-relaxed text-slate-300 sm:text-base">
                {overview || "No overview available."}
              </p>
            </section>

            {(director || writers.length > 0) && (
              <section className="mt-6 text-left" aria-labelledby="crew-heading">
                <h2
                  id="crew-heading"
                  className="mb-3 text-lg font-semibold text-white sm:text-xl"
                >
                  Key Crew
                </h2>
                <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3">
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

        {castForSection.length > 0 && <CastSection cast={castForSection} />}

        <section
          className="mt-8 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4 sm:mt-10 sm:p-5"
          aria-labelledby="details-heading"
        >
          <h2
            id="details-heading"
            tabIndex={-1}
            className="sr-only outline-none"
          >
            Additional Details
          </h2>
          <dl className="grid grid-cols-1 gap-5 sm:grid-cols-2 sm:gap-6 lg:grid-cols-4">
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
                <data value={budget ?? 0}>{formatCurrency(budget ?? 0)}</data>
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
                <data value={revenue ?? 0}>{formatCurrency(revenue ?? 0)}</data>
              </dd>
            </div>
          </dl>
        </section>

        {production_companies.length > 0 && (
          <section className="mt-8 sm:mt-10" aria-labelledby="companies-heading">
            <h2
              id="companies-heading"
              className="mb-4 text-xl font-semibold text-white sm:text-2xl"
            >
              Production Companies
            </h2>
            <ul className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-4 md:gap-6">
              {production_companies.map(pc => {
                const logoPath = unwrapString(pc.logo);
                const logoUrl = buildTmdbImageUrl(logoPath, TMDB_LOGO_SIZE);
                return (
                  <li
                    key={pc.id}
                    className="flex min-w-0 items-center gap-3 rounded-lg border border-amber-500/10 bg-slate-800/50 px-4 py-3 sm:max-w-md"
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
