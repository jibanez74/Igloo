import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  authUserQueryOpts,
  libraryMovieDetailsQueryOpts,
  movieLikeStatusQueryOpts,
  movieTechnicalDetailsQueryOpts,
} from "@/lib/query-opts";
import {
  MOVIE_DETAILS_CONTENT_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { prepareYouTubeExtrasForDisplay } from "@/lib/format";
import { unwrapFloat, unwrapInt, unwrapString } from "@/lib/nullable";
import MediaNotFound from "@/components/MediaNotFound";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import CastSection from "@/components/CastSection";
import MovieDetailsBackdrop from "@/components/MovieDetailsBackdrop";
import MovieDetailsSkipLinks from "@/components/MovieDetailsSkipLinks";
import MovieDetailsPosterBlock from "@/components/MovieDetailsPosterBlock";
import MovieDetailsTitleHeading from "@/components/MovieDetailsTitleHeading";
import MovieDetailsMetadataChips from "@/components/MovieDetailsMetadataChips";
import MovieDetailsGenresList from "@/components/MovieDetailsGenresList";
import MovieDetailsHeroActions from "@/components/MovieDetailsHeroActions";
import MovieOverviewSection from "@/components/MovieOverviewSection";
import MovieKeyCrewSection from "@/components/MovieKeyCrewSection";
import MovieAdditionalDetailsSection from "@/components/MovieAdditionalDetailsSection";
import MovieExtraVideosSection from "@/components/MovieExtraVideosSection";
import MovieProductionCompaniesSection from "@/components/MovieProductionCompaniesSection";
import MovieChaptersSection from "@/components/MovieChaptersSection";
import {
  DEFAULT_PLAYBACK_SETTINGS,
  getAvailableModes,
  getDefaultPlaybackSettings,
  getPrimaryVideoStream,
  resolvePlaybackSettings,
  type PlaybackSettings,
} from "@/lib/playback";
import { cn } from "@/lib/utils";
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
        context.queryClient.ensureQueryData(
          movieLikeStatusQueryOpts(movieId),
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
  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const chapters = techData?.data?.chapters ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const videoStream = getPrimaryVideoStream(videoStreams);
  const mimeType = techData?.data?.movie?.mime_type;
  const availableModes = getAvailableModes(
    videoStream?.height ?? 0,
    videoStream?.codec,
    audioStreams[0]?.codec,
    mimeType ?? undefined,
  );
  const smartDefault: PlaybackSettings = resolvePlaybackSettings(
    getDefaultPlaybackSettings(availableModes),
    availableModes,
    audioStreams,
    subtitleStreams,
  );

  const [playbackSettings, setPlaybackSettings] = useState<PlaybackSettings>(
    DEFAULT_PLAYBACK_SETTINGS,
  );

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

  const youtubeExtraVideos = prepareYouTubeExtrasForDisplay(extra_videos);

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

  const castForSection = libraryCastToCastSection(cast);

  const certificationLabel =
    certification != null && certification.trim() !== ""
      ? certification.trim()
      : null;

  const showCrewSection = crew.length > 0;

  return (
    <article aria-labelledby="movie-title" className="pb-6 sm:pb-10">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <MovieDetailsSkipLinks
        showCrewSection={showCrewSection}
        castNonEmpty={castForSection.length > 0}
        chaptersNonEmpty={chapters.length > 0}
        extrasNonEmpty={youtubeExtraVideos.length > 0}
        companiesNonEmpty={production_companies.length > 0}
      />

      <div className={cn(MOVIE_DETAILS_CONTENT_ENTER_CLASS)}>
        <MovieDetailsBackdrop backdropUrl={backdropUrl} />
      </div>

      <div className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32">
        <div
          className={cn(
            MOVIE_DETAILS_CONTENT_ENTER_CLASS,
            "delay-75 motion-reduce:delay-0",
          )}
        >
          <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
            <MovieDetailsPosterBlock
              posterUrl={posterUrl}
              movieTitle={movie.title}
            />

            <div className="min-w-0 flex-1 text-center lg:text-left">
              <MovieDetailsTitleHeading
                title={movie.title}
                releaseYear={releaseYear}
                releaseDateStr={releaseDateStr}
              />

              {tagLine && (
                <p className="mt-2 max-w-full text-base wrap-break-word text-slate-400 italic sm:text-lg">
                  <q>{tagLine}</q>
                </p>
              )}

              <MovieDetailsMetadataChips
                criticRating={criticRating}
                audienceRating={audienceRating}
                certificationLabel={certificationLabel}
                runtime={runtime}
                runTimeMins={runTimeMins}
                releaseDateStr={releaseDateStr}
              />

              <MovieDetailsGenresList genres={genres} />

              <MovieDetailsHeroActions
                movieId={movieId}
                movie={movie}
                movieTitle={movie.title}
                user={user}
                playbackSettings={playbackSettings}
                onPlaybackSettingsChange={setPlaybackSettings}
                playbackSettingsOpen={playbackSettingsOpen}
                onPlaybackSettingsOpenChange={setPlaybackSettingsOpen}
                technicalDetailsOpen={technicalDetailsOpen}
                onTechnicalDetailsOpenChange={setTechnicalDetailsOpen}
                editOpen={editOpen}
                onEditOpenChange={setEditOpen}
                deleteOpen={deleteOpen}
                onDeleteOpenChange={setDeleteOpen}
              />

              <MovieOverviewSection overview={overview} />
              <MovieKeyCrewSection crew={crew} />
            </div>
          </div>
        </div>

        <div
          className={cn(
            MOVIE_DETAILS_CONTENT_ENTER_CLASS,
            "delay-150 motion-reduce:delay-0",
          )}
        >
          {castForSection.length > 0 && <CastSection cast={castForSection} />}

          <MovieChaptersSection
            chapters={chapters}
            movieId={movieId}
            playbackSettings={playbackSettings}
          />

          <MovieAdditionalDetailsSection
            language={language}
            budget={budget}
            revenue={revenue}
          />

          <MovieExtraVideosSection
            videos={youtubeExtraVideos}
            movieId={movieId}
          />

          <MovieProductionCompaniesSection companies={production_companies} />
        </div>
      </div>
    </article>
  );
}
