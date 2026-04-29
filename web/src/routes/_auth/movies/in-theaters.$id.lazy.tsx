import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Play } from "lucide-react";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import {
  MOVIE_DETAILS_CONTENT_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { prepareYouTubeExtrasForDisplay } from "@/lib/format";
import MediaNotFound from "@/components/MediaNotFound";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import CastSection from "@/components/CastSection";
import MovieDetailsBackdrop from "@/components/MovieDetailsBackdrop";
import MovieDetailsSkipLinks from "@/components/MovieDetailsSkipLinks";
import MovieDetailsPosterBlock from "@/components/MovieDetailsPosterBlock";
import MovieDetailsTitleHeading from "@/components/MovieDetailsTitleHeading";
import MovieDetailsMetadataChips from "@/components/MovieDetailsMetadataChips";
import MovieDetailsGenresList from "@/components/MovieDetailsGenresList";
import MovieOverviewSection from "@/components/MovieOverviewSection";
import MovieKeyCrewSection from "@/components/MovieKeyCrewSection";
import MovieAdditionalDetailsSection from "@/components/MovieAdditionalDetailsSection";
import MovieExtraVideosSection from "@/components/MovieExtraVideosSection";
import MovieProductionCompaniesSection from "@/components/MovieProductionCompaniesSection";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type {
  CrewMemberType,
  LibraryMovieCrewType,
  LibraryMovieExtraVideoType,
  LibraryMovieProductionCompanyType,
  MovieDetailsType,
} from "@/types";
import type { NullableString } from "@/types";

export const Route = createLazyFileRoute("/_auth/movies/in-theaters/$id")({
  component: MovieDetailsPage,
});

function toNullableString(value: string | null | undefined): NullableString {
  if (value == null || value === "") return { String: "", Valid: false };
  return { String: value, Valid: true };
}

function tmdbCrewToLibraryCrew(
  movieId: number,
  crew: CrewMemberType[],
): LibraryMovieCrewType[] {
  return crew.map(c => ({
    id: c.id,
    movie_id: movieId,
    artist_id: c.id,
    job: c.job,
    department: c.department,
    artist_name: c.name,
    artist_profile: toNullableString(c.profile_path),
  }));
}

function tmdbYouTubeResultsToLibraryExtras(
  results: MovieDetailsType["videos"]["results"],
): LibraryMovieExtraVideoType[] {
  const mapped: LibraryMovieExtraVideoType[] = results
    .filter(v => v.site === "YouTube")
    .map((v, index) => ({
      id: index + 1,
      title: v.name,
      external_id: toNullableString(v.id),
      key: v.key,
      type: v.type,
      site: v.site,
      official: v.official,
      created_at: "",
      updated_at: "",
    }));
  return prepareYouTubeExtrasForDisplay(mapped);
}

function tmdbProductionCompaniesToLibrary(
  companies: MovieDetailsType["production_companies"],
): LibraryMovieProductionCompanyType[] {
  return companies.map(pc => ({
    id: pc.id,
    name: pc.name,
    tmdb_id: pc.id,
    logo: toNullableString(pc.logo_path ?? undefined),
    country: toNullableString(pc.origin_country ?? undefined),
  }));
}

function MovieDetailsPage() {
  const { id } = Route.useParams();
  const movieId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(movieDetailsQueryOpts(movieId));
  const movie = data?.data?.movie;

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

  if (!movie) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Movie not found
        </h2>
      </div>
    );
  }

  return <MovieDetailsContent movie={movie} />;
}

function MovieDetailsContent({ movie }: { movie: MovieDetailsType }) {
  const backdropUrl = buildTmdbImageUrl(
    movie.backdrop_path ?? null,
    TMDB_BACKDROP_SIZE,
  );
  const posterUrl = buildTmdbImageUrl(
    movie.poster_path ?? null,
    TMDB_POSTER_SIZE,
  );

  const releaseDateStr = movie.release_date || null;
  const releaseYear = releaseDateStr
    ? new Date(releaseDateStr).getFullYear()
    : null;

  const pageTitle = releaseYear
    ? `${movie.title} (${releaseYear}) - Igloo`
    : `${movie.title} - Igloo`;
  const pageDescription = movie.overview
    ? movie.overview.slice(0, 160)
    : `Watch ${movie.title} in your Igloo media library.`;

  const runtime = movie.runtime
    ? `${Math.floor(movie.runtime / 60)}h ${movie.runtime % 60}m`
    : null;

  const trailer = movie.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube",
  );

  const crewForSection = tmdbCrewToLibraryCrew(
    movie.id,
    movie.credits?.crew ?? [],
  );
  const castList = movie.credits?.cast ?? [];
  const youtubeExtraVideos = tmdbYouTubeResultsToLibraryExtras(
    movie.videos?.results ?? [],
  );
  const productionCompanies = tmdbProductionCompaniesToLibrary(
    movie.production_companies ?? [],
  );

  const genresForList =
    movie.genres?.map(g => ({ id: g.id, tag: g.name })) ?? [];

  const showCrewSection = crewForSection.length > 0;
  const trailerReturnPath = `/movies/in-theaters/${movie.id}`;

  return (
    <article
      aria-labelledby="movie-title"
      className="w-full min-w-0 pb-6 sm:pb-10"
    >
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <MovieDetailsSkipLinks
        showCrewSection={showCrewSection}
        castNonEmpty={castList.length > 0}
        chaptersNonEmpty={false}
        extrasNonEmpty={youtubeExtraVideos.length > 0}
        companiesNonEmpty={productionCompanies.length > 0}
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

              {movie.tagline && (
                <p className="mt-2 max-w-full text-base wrap-break-word text-slate-400 italic sm:text-lg">
                  <q>{movie.tagline}</q>
                </p>
              )}

              <MovieDetailsMetadataChips
                criticRating={null}
                audienceRating={null}
                certificationLabel={null}
                runtime={runtime}
                runTimeMins={movie.runtime ?? null}
                releaseDateStr={releaseDateStr}
                tmdbVoteAverage={
                  movie.vote_average > 0 ? movie.vote_average : null
                }
              />

              <MovieDetailsGenresList genres={genresForList} />

              {trailer && (
                <div className="mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start">
                  <Link
                    to="/trailer"
                    search={{
                      mediaType: "movie",
                      mediaId: movie.id,
                      returnTo: trailerReturnPath,
                    }}
                    mask={{
                      to: "/movies/in-theaters/$id",
                      params: { id: String(movie.id) },
                    }}
                    className={cn(
                      buttonVariants({ variant: "accent", size: "lg" }),
                      "min-h-11 min-w-34 touch-manipulation sm:min-w-0",
                    )}
                  >
                    <Play className="size-4 fill-current" aria-hidden="true" />
                    Play Trailer
                  </Link>
                </div>
              )}

              <MovieOverviewSection overview={movie.overview || null} />
              <MovieKeyCrewSection crew={crewForSection} />
            </div>
          </div>
        </div>

        <div
          className={cn(
            MOVIE_DETAILS_CONTENT_ENTER_CLASS,
            "delay-150 motion-reduce:delay-0",
          )}
        >
          {castList.length > 0 && <CastSection cast={castList} />}

          <MovieAdditionalDetailsSection
            status={movie.status || null}
            language={movie.original_language || null}
            budget={movie.budget}
            revenue={movie.revenue}
          />

          <MovieExtraVideosSection
            videos={youtubeExtraVideos}
            movieId={movie.id}
            trailerReturnTo={trailerReturnPath}
          />

          <MovieProductionCompaniesSection companies={productionCompanies} />
        </div>
      </div>
    </article>
  );
}
