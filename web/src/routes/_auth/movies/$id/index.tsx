import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { libraryToMovieDetailsView } from "@/lib/movie-details-view";
import CastSection from "@/components/CastSection";
import MediaNotFound from "@/components/MediaNotFound";
import MovieDetailsSkeleton from "@/components/MovieDetailsSkeleton";
import {
  MovieDetailsLayout,
  MovieDetailsSkipNav,
  MoviePoster,
  MovieTitleBlock,
  MovieTagline,
  MovieMetaRow,
  MovieGenres,
  MoviePlayButton,
  MovieLikeButton,
  MovieMoreMenu,
  MovieOverviewSection,
  MovieCrewSection,
  MovieDetailsGrid,
  MovieDocumentMeta,
  MovieExtraVideosSection,
  MovieProductionCompaniesSection,
} from "@/components/movie-details";

export const Route = createFileRoute("/_auth/movies/$id/")({
  loader: async ({ context, params }) => {
    const movieId = parseInt(params.id, 10);
    if (!Number.isNaN(movieId) && movieId > 0) {
      await context.queryClient.ensureQueryData(
        libraryMovieDetailsQueryOpts(movieId),
      );
    }
  },
  component: MovieDetailsPage,
});

function MovieDetailsPage() {
  const { id } = Route.useParams();
  const movieId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(
    libraryMovieDetailsQueryOpts(movieId),
  );

  const responseData = data?.data;

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
    return <MovieDetailsSkeleton variant="posterOnly" />;
  }

  if (!responseData?.movie) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-slate-300">
          Movie not found
        </h2>
      </div>
    );
  }

  const view = libraryToMovieDetailsView(responseData);

  let pageTitle: string;
  if (view.releaseYear) {
    pageTitle = `${view.title} (${view.releaseYear}) - Igloo`;
  } else {
    pageTitle = `${view.title} - Igloo`;
  }

  let pageDescription: string;
  if (view.tagline) {
    pageDescription = view.tagline.slice(0, 160);
  } else {
    pageDescription = `Watch ${view.title} in your Igloo media library.`;
  }

  const skipSections: { id: string; label: string }[] = [
    { id: "movie-title", label: "Skip to movie info" },
    { id: "overview-heading", label: "Skip to overview" },
  ];

  if (view.cast.length > 0) {
    skipSections.push({ id: "cast-heading", label: "Skip to cast" });
  }

  if (view.extraVideos.length > 0) {
    skipSections.push({
      id: "extra-videos-heading",
      label: "Skip to extra videos",
    });
  }

  skipSections.push({ id: "details-heading", label: "Skip to details" });

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
          <div className="mt-6 flex flex-wrap items-center gap-3">
            <MoviePlayButton movieId={view.id} />
            <MovieLikeButton movieId={view.id} />
            <MovieMoreMenu movieId={view.id} />
          </div>
          <MovieOverviewSection overview={view.overview} />
          <MovieCrewSection
            director={view.director}
            writers={view.writers}
          />
        </div>
      </div>

      {view.cast.length > 0 && <CastSection cast={view.cast} />}
      {view.extraVideos.length > 0 && (
        <MovieExtraVideosSection extraVideos={view.extraVideos} />
      )}
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
