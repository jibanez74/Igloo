import { createLazyFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import { tmdbToMovieDetailsView } from "@/lib/movie-details-view";
import { AlertCircle } from "lucide-react";
import CastSection from "@/components/CastSection";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import {
  MovieDetailsLayout,
  MovieDetailsSkipNav,
  MoviePoster,
  MovieTitleBlock,
  MovieTagline,
  MovieMetaRow,
  MovieGenres,
  MovieTrailerButton,
  MovieOverviewSection,
  MovieCrewSection,
  MovieDetailsGrid,
  MovieDocumentMeta,
  MovieProductionCompaniesSection,
} from "@/components/movie-details";
import type { MovieDetailsType } from "@/types";

export const Route = createLazyFileRoute("/_auth/movies/in-theaters/$id")({
  component: MovieDetailsPage,
});

function MovieDetailsPage() {
  const { id } = Route.useParams();
  const movieId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(movieDetailsQueryOpts(movieId));
  const movie = data?.data?.movie;

  if (isError || (data && data.error)) {
    return (
      <Alert
        variant="destructive"
        className="border-red-500/20 bg-red-500/10 text-red-400"
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
  const view = tmdbToMovieDetailsView(movie);

  const pageTitle = view.releaseYear
    ? `${view.title} (${view.releaseYear}) - Igloo`
    : `${view.title} - Igloo`;
  const pageDescription = view.tagline
    ? view.tagline.slice(0, 160)
    : `Watch ${view.title} in your Igloo media library.`;

  const skipSections: { id: string; label: string }[] = [
    { id: "movie-title", label: "Skip to movie info" },
    { id: "overview-heading", label: "Skip to overview" },
    ...(view.cast.length > 0
      ? [{ id: "cast-heading", label: "Skip to cast" }]
      : []),
    { id: "details-heading", label: "Skip to details" },
  ];

  return (
    <MovieDetailsLayout backdropUrl={view.backdropUrl}>
      <MovieDocumentMeta title={pageTitle} description={pageDescription} />
      <MovieDetailsSkipNav sections={skipSections} />

      <div className="flex flex-col gap-6 md:flex-row md:gap-8 lg:gap-10">
        <MoviePoster src={view.posterUrl} title={view.title} />

        <div className="min-w-0 flex-1">
          <MovieTitleBlock
            title={view.title}
            releaseYear={view.releaseYear}
            releaseDate={view.releaseDate}
          />
          <MovieTagline tagline={view.tagline} />
          <MovieMetaRow
            rating={view.rating}
            runtime={view.runtimeFormatted}
            runtimeMinutes={view.runtimeMinutes}
            releaseDate={view.releaseDate}
          />
          <MovieGenres genres={view.genres} />
          {view.trailerKey && (
            <MovieTrailerButton
              movieId={view.id}
              returnTo="/movies/in-theaters/$id"
              returnToParams={{ id: String(view.id) }}
            />
          )}
          <MovieOverviewSection overview={view.overview} />
          <MovieCrewSection
            director={view.director}
            writers={view.writers}
          />
        </div>
      </div>

      {view.cast.length > 0 && <CastSection cast={view.cast} />}
      {view.productionCompanies.length > 0 && (
        <MovieProductionCompaniesSection
          companies={view.productionCompanies}
        />
      )}
      {view.detailsGridItems.length > 0 && (
        <MovieDetailsGrid items={view.detailsGridItems} />
      )}
    </MovieDetailsLayout>
  );
}
