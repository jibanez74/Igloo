import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  showDetailsQueryOpts,
  showSeasonEpisodesQueryOpts,
} from "@/lib/query-opts";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { showDetailsSearchSchema } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { prepareYouTubeExtrasForDisplay } from "@/lib/format";
import { unwrapFloat, unwrapInt, unwrapString } from "@/lib/nullable";
import { cn } from "@/lib/utils";
import MediaNotFound from "@/components/shared/MediaNotFound";
import CastSection from "@/components/shared/CastSection";
import ExtraVideosSection from "@/components/shared/ExtraVideosSection";
import ShowDetailsSkeleton from "@/components/shows/ShowDetailsSkeleton";
import ShowDetailsSkipLinks from "@/components/shows/ShowDetailsSkipLinks";
import ShowDetailsHero from "@/components/shows/ShowDetailsHero";
import ShowDetailsMetadataChips from "@/components/shows/ShowDetailsMetadataChips";
import ShowOverviewSection from "@/components/shows/ShowOverviewSection";
import ShowSeasonsSection from "@/components/shows/ShowSeasonsSection";
import ShowCrewSection from "@/components/shows/ShowCrewSection";
import ShowAboutSection from "@/components/shows/ShowAboutSection";
import type { CastSectionItem } from "@/components/shared/CastSection";
import type { ShowDetailsDataType } from "@/types";

export const Route = createFileRoute("/_auth/tv-shows/$id")({
  validateSearch: showDetailsSearchSchema,
  loaderDeps: ({ search: { season } }) => ({ season }),
  loader: async ({ context, params, deps: { season } }) => {
    const showId = parseInt(params.id, 10);
    if (Number.isNaN(showId) || showId <= 0) return;

    const details = await context.queryClient.ensureQueryData(
      showDetailsQueryOpts(showId),
    );

    // Resolve the season here rather than in the component so the page paints
    // complete: the URL's season when it exists, else the first in the order
    // the API returned (specials sort last, so that is season one for a normal
    // show).
    if (details.error) return;
    const seasons = details.data?.seasons ?? [];
    if (seasons.length === 0) return;

    const selected =
      season != null && seasons.some(s => s.season_number === season)
        ? season
        : seasons[0].season_number;

    await context.queryClient.ensureQueryData(
      showSeasonEpisodesQueryOpts(showId, selected),
    );
  },
  component: ShowDetailsPage,
});

function showCastToCastSection(
  cast: ShowDetailsDataType["cast"],
): CastSectionItem[] {
  // Keyed on credit_id, not artist_id: TMDB aggregate credits can give one
  // artist several roles on the same show.
  return cast.map(c => ({
    key: c.credit_id,
    name: c.artist_name,
    character: c.character,
    profilePath: unwrapString(c.artist_profile),
    episodeCount: c.episode_count,
  }));
}

function ShowDetailsPage() {
  const { id } = Route.useParams();
  const showId = parseInt(id, 10);

  const { data, isPending, isError } = useQuery(showDetailsQueryOpts(showId));

  const payload = data?.data;
  const show = payload?.show;

  if (isError || (data && data.error)) {
    return (
      <MediaNotFound
        message={
          data?.message || "Failed to load show details. Please try again later."
        }
        backTo="/"
        backLabel="Back to Home"
      />
    );
  }

  if (isPending) {
    return <ShowDetailsSkeleton />;
  }

  if (!show || !payload) {
    return (
      <div className="py-12 text-center">
        <h2 className="text-xl font-semibold text-muted-foreground">
          Show not found
        </h2>
      </div>
    );
  }

  return <ShowDetailsContent key={showId} showId={showId} payload={payload} />;
}

function ShowDetailsContent({
  showId,
  payload,
}: {
  showId: number;
  payload: ShowDetailsDataType;
}) {
  const { season } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const {
    show,
    seasons,
    cast,
    crew,
    creators,
    genres,
    networks,
    production_companies,
    extra_videos,
  } = payload;

  // The selected season is URL state. An out-of-range season in the URL falls
  // back to the first season rather than rendering an empty list.
  const selectedSeason =
    season != null && seasons.some(s => s.season_number === season)
      ? season
      : (seasons[0]?.season_number ?? 0);

  const posterPath = unwrapString(show.poster_path);
  const backdropPath = unwrapString(show.backdrop_path);
  const overview = unwrapString(show.overview);
  const tagline = unwrapString(show.tagline);
  const firstAirDate = unwrapString(show.first_air_date);
  const lastAirDate = unwrapString(show.last_air_date);
  const premiereYear = unwrapInt(show.premiere_year);
  const certification = unwrapString(show.certification);
  const voteAverage = unwrapFloat(show.vote_average);

  const posterUrl = buildTmdbImageUrl(posterPath, TMDB_POSTER_SIZE);
  const backdropUrl = buildTmdbImageUrl(backdropPath, TMDB_BACKDROP_SIZE);

  const pageTitle = premiereYear
    ? `${show.name} (${premiereYear}) - Igloo`
    : `${show.name} - Igloo`;
  const pageDescription = overview
    ? overview.slice(0, 160)
    : `Browse ${show.name} in your Igloo media library.`;

  const castForSection = showCastToCastSection(cast);
  const youtubeExtraVideos = prepareYouTubeExtrasForDisplay(extra_videos);

  // Specials are excluded from both tallies: TMDB's season and episode counts
  // cover the numbered run only, so counting season zero would compare a local
  // "3 seasons" against a TMDB "2".
  const numberedSeasons = seasons.filter(s => s.season_number > 0);
  const availableEpisodeCount = numberedSeasons.reduce(
    (total, s) => total + s.available_episode_count,
    0,
  );

  const certificationLabel =
    certification != null && certification.trim() !== ""
      ? certification.trim()
      : null;

  return (
    <article aria-labelledby="show-title" className="w-full min-w-0 pb-6 sm:pb-10">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <ShowDetailsSkipLinks
        seasonsNonEmpty={seasons.length > 0}
        crewNonEmpty={creators.length > 0 || crew.length > 0}
        castNonEmpty={castForSection.length > 0}
        extrasNonEmpty={youtubeExtraVideos.length > 0}
      />

      <ShowDetailsHero
        backdropUrl={backdropUrl}
        posterUrl={posterUrl}
        name={show.name}
        premiereYear={premiereYear}
        firstAirDate={firstAirDate}
        tagline={tagline}
        genres={genres}
        metadataSlot={
          <ShowDetailsMetadataChips
            tmdbVoteAverage={voteAverage}
            certificationLabel={certificationLabel}
            status={unwrapString(show.status)}
            firstAirDate={firstAirDate}
            lastAirDate={lastAirDate}
            seasonCount={numberedSeasons.length}
            tmdbSeasonCount={unwrapInt(show.tmdb_season_count)}
            availableEpisodeCount={availableEpisodeCount}
            tmdbEpisodeCount={unwrapInt(show.tmdb_episode_count)}
          />
        }
      />

      <div
        className={cn(
          DETAIL_PAGE_CONTENT_ENTER_CLASS,
          "delay-150 motion-reduce:delay-0",
        )}
      >
        <ShowOverviewSection overview={overview} />

        <ShowSeasonsSection
          showId={showId}
          seasons={seasons}
          selectedSeason={selectedSeason}
          onSelectSeason={seasonNumber => {
            // replace: true so paging through seasons does not stack history.
            navigate({
              search: { season: seasonNumber },
              replace: true,
            });
          }}
        />

        <ShowCrewSection creators={creators} crew={crew} />

        {castForSection.length > 0 && <CastSection cast={castForSection} />}

        <ExtraVideosSection
          videos={youtubeExtraVideos}
          returnTo={`/tv-shows/${showId}`}
        />

        <ShowAboutSection
          name={show.name}
          originalName={unwrapString(show.original_name)}
          status={unwrapString(show.status)}
          type={unwrapString(show.type)}
          language={unwrapString(show.language)}
          firstAirDate={firstAirDate}
          lastAirDate={lastAirDate}
          networks={networks}
          companies={production_companies}
        />
      </div>
    </article>
  );
}
