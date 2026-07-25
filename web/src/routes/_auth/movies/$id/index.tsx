import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  authUserQueryOpts,
  libraryMovieDetailsQueryOpts,
  movieLikeStatusQueryOpts,
  movieTechnicalDetailsQueryOpts,
  movieWatchProgressQueryOpts,
  playbackSettingsQueryOpts,
} from "@/lib/query-opts";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import {
  formatRuntimeMinutes,
  prepareYouTubeExtrasForDisplay,
} from "@/lib/format";
import { unwrapFloat, unwrapInt, unwrapString } from "@/lib/nullable";
import MediaNotFound from "@/components/shared/MediaNotFound";
import MovieDetailsSkeleton from "@/components/movies/MovieDetailsSkeleton";
import CastSection from "@/components/movies/CastSection";
import MovieDetailsHero from "@/components/movies/MovieDetailsHero";
import MovieDetailsSkipLinks from "@/components/movies/MovieDetailsSkipLinks";
import MovieDetailsMetadataChips from "@/components/movies/MovieDetailsMetadataChips";
import MovieDetailsHeroActions from "@/components/movies/MovieDetailsHeroActions";
import MovieDetailsResumeProgress from "@/components/movies/MovieDetailsResumeProgress";
import MovieOverviewSection from "@/components/movies/MovieOverviewSection";
import MovieKeyCrewSection from "@/components/movies/MovieKeyCrewSection";
import MovieAboutSection from "@/components/movies/MovieAboutSection";
import MovieExtraVideosSection from "@/components/movies/MovieExtraVideosSection";
import MovieChaptersSection from "@/components/movies/MovieChaptersSection";
import {
  getAvailableModes,
  getPrimaryVideoStream,
  resolvePlaybackSettings
} from "@/lib/playback";
import { deriveMediaCapabilityBadges } from "@/lib/media-capabilities";
import type { PlaybackSettings } from "@/types/playback";
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
        context.queryClient.ensureQueryData(
          movieWatchProgressQueryOpts(movieId),
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
        backTo="/movies"
        backLabel="Back to Movies"
      />
    );
  }

  if (isPending) {
    return <MovieDetailsSkeleton />;
  }

  if (!movie || !payload) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
          Movie not found
        </h2>
      </div>
    );
  }

  return (
    <LibraryMovieDetailsContent
      key={movieId}
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
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const { data: techData } = useQuery(movieTechnicalDetailsQueryOpts(movieId));
  const { data: userData } = useQuery(authUserQueryOpts());
  const user: AuthUser | null =
    userData?.error === false && userData.data?.user
      ? (userData.data.user)
      : null;
  const { data: playbackSettingsData } = useQuery({
    ...playbackSettingsQueryOpts(user?.id ?? 0),
    enabled: user !== null,
  });
  const userPlaybackPrefs =
    playbackSettingsData?.error === false &&
    playbackSettingsData.data?.settings
      ? playbackSettingsData.data.settings
      : null;
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
    null,
    availableModes,
    audioStreams,
    subtitleStreams,
    userPlaybackPrefs,
  );

  // Null until the user saves the Playback Settings dialog. Before a save the
  // play link tracks the smart default as queries load; after a save the
  // choice is only re-resolved against the current streams, never replaced.
  const [savedSettings, setSavedSettings] = useState<PlaybackSettings | null>(
    null,
  );
  const playbackSettings =
    savedSettings === null
      ? smartDefault
      : resolvePlaybackSettings(
          savedSettings,
          availableModes,
          audioStreams,
          subtitleStreams,
          userPlaybackPrefs,
        );

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

  const runtime = formatRuntimeMinutes(runTimeMins);

  const castForSection = libraryCastToCastSection(cast);

  const certificationLabel =
    certification != null && certification.trim() !== ""
      ? certification.trim()
      : null;

  const showCrewSection = crew.length > 0;

  const capabilityBadges = deriveMediaCapabilityBadges(techData?.data);

  return (
    <article
      aria-labelledby="movie-title"
      className="w-full min-w-0 pb-6 sm:pb-10"
    >
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <MovieDetailsSkipLinks
        showCrewSection={showCrewSection}
        castNonEmpty={castForSection.length > 0}
        chaptersNonEmpty={chapters.length > 0}
        extrasNonEmpty={youtubeExtraVideos.length > 0}
      />

      <MovieDetailsHero
        backdropUrl={backdropUrl}
        posterUrl={posterUrl}
        movieTitle={movie.title}
        releaseYear={releaseYear}
        releaseDateStr={releaseDateStr}
        tagLine={tagLine}
        genres={genres}
        metadataSlot={
          <MovieDetailsMetadataChips
            criticRating={criticRating}
            audienceRating={audienceRating}
            certificationLabel={certificationLabel}
            runtime={runtime}
            runTimeMins={runTimeMins}
            releaseDateStr={releaseDateStr}
            capabilityBadges={capabilityBadges}
          />
        }
        progressSlot={<MovieDetailsResumeProgress movieId={movieId} />}
        actionsSlot={
          <MovieDetailsHeroActions
            movieId={movieId}
            movie={movie}
            movieTitle={movie.title}
            user={user}
            playbackSettings={playbackSettings}
            onPlaybackSettingsChange={setSavedSettings}
            playbackSettingsOpen={playbackSettingsOpen}
            onPlaybackSettingsOpenChange={setPlaybackSettingsOpen}
            technicalDetailsOpen={technicalDetailsOpen}
            onTechnicalDetailsOpenChange={setTechnicalDetailsOpen}
            editOpen={editOpen}
            onEditOpenChange={setEditOpen}
            deleteOpen={deleteOpen}
            onDeleteOpenChange={setDeleteOpen}
          />
        }
      />

      <div
        className={cn(
          DETAIL_PAGE_CONTENT_ENTER_CLASS,
          "delay-150 motion-reduce:delay-0",
        )}
      >
        <MovieOverviewSection overview={overview} />
        <MovieKeyCrewSection crew={crew} />

        {castForSection.length > 0 && <CastSection cast={castForSection} />}

        <MovieChaptersSection
          chapters={chapters}
          movieId={movieId}
          playbackSettings={playbackSettings}
        />

        <MovieExtraVideosSection
          videos={youtubeExtraVideos}
          movieId={movieId}
        />

        <MovieAboutSection
          movieTitle={movie.title}
          language={language}
          budget={budget}
          revenue={revenue}
          companies={production_companies}
        />
      </div>
    </article>
  );
}
