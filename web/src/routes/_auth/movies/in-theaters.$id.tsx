import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Play } from "lucide-react";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { pickTmdbCertification } from "@/lib/tmdb-certification";
import {
  formatRuntimeMinutes,
  prepareYouTubeExtrasForDisplay,
} from "@/lib/format";
import MediaNotFound from "@/components/shared/MediaNotFound";
import MovieDetailsSkeleton from "@/components/movies/MovieDetailsSkeleton";
import CastSection from "@/components/movies/CastSection";
import MovieDetailsHero from "@/components/movies/MovieDetailsHero";
import MovieDetailsSkipLinks from "@/components/movies/MovieDetailsSkipLinks";
import MovieDetailsMetadataChips from "@/components/movies/MovieDetailsMetadataChips";
import MovieOverviewSection from "@/components/movies/MovieOverviewSection";
import MovieKeyCrewSection from "@/components/movies/MovieKeyCrewSection";
import MovieAboutSection from "@/components/movies/MovieAboutSection";
import MovieExtraVideosSection from "@/components/movies/MovieExtraVideosSection";
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

export const Route = createFileRoute("/_auth/movies/in-theaters/$id")({
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
  results: NonNullable<MovieDetailsType["videos"]["results"]>,
): LibraryMovieExtraVideoType[] {
  const mapped: LibraryMovieExtraVideoType[] = [];
  let id = 1;
  for (const v of results) {
    if (v.site !== "YouTube") {
      continue;
    }

    mapped.push({
      id,
      title: v.name,
      external_id: toNullableString(v.id),
      key: v.key,
      type: v.type,
      site: v.site,
      official: v.official,
      created_at: "",
      updated_at: "",
    });
    id += 1;
  }

  return prepareYouTubeExtrasForDisplay(mapped);
}

function tmdbProductionCompaniesToLibrary(
  companies: NonNullable<MovieDetailsType["production_companies"]>,
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
        backTo="/"
        backLabel="Back to Home"
      />
    );
  }

  if (isPending) {
    return <MovieDetailsSkeleton />;
  }

  if (!movie) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
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

  const runtime = formatRuntimeMinutes(movie.runtime);
  const certificationLabel = pickTmdbCertification(movie.release_dates);

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
      />

      <MovieDetailsHero
        backdropUrl={backdropUrl}
        posterUrl={posterUrl}
        movieTitle={movie.title}
        releaseYear={releaseYear}
        releaseDateStr={releaseDateStr}
        tagLine={movie.tagline || null}
        genres={genresForList}
        metadataSlot={
          <MovieDetailsMetadataChips
            criticRating={null}
            audienceRating={null}
            certificationLabel={certificationLabel}
            runtime={runtime}
            runTimeMins={movie.runtime ?? null}
            releaseDateStr={releaseDateStr}
            tmdbVoteAverage={
              movie.vote_average > 0 ? movie.vote_average : null
            }
          />
        }
        actionsSlot={
          trailer ? (
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
          ) : undefined
        }
      />

      <div
        className={cn(
          DETAIL_PAGE_CONTENT_ENTER_CLASS,
          "delay-150 motion-reduce:delay-0",
        )}
      >
        <MovieOverviewSection overview={movie.overview || null} />
        <MovieKeyCrewSection crew={crewForSection} />

        {castList.length > 0 && <CastSection cast={castList} />}

        <MovieExtraVideosSection
          videos={youtubeExtraVideos}
          movieId={movie.id}
          trailerReturnTo={trailerReturnPath}
        />

        <MovieAboutSection
          movieTitle={movie.title}
          status={movie.status || null}
          language={movie.original_language || null}
          budget={movie.budget}
          revenue={movie.revenue}
          companies={productionCompanies}
        />
      </div>
    </article>
  );
}
