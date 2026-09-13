import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Tv } from "lucide-react";
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
import { parseRouteId } from "@/lib/route-id";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { prepareYouTubeExtrasForDisplay } from "@/lib/format";
import {
  trimmedOrNull,
  unwrapFloat,
  unwrapInt,
  unwrapString,
} from "@/lib/nullable";
import { cn } from "@/lib/utils";
import MediaNotFound from "@/components/shared/MediaNotFound";
import CastSection from "@/components/shared/CastSection";
import ExtraVideosSection from "@/components/shared/ExtraVideosSection";
import DetailSkeleton from "@/components/shared/DetailSkeleton";
import DetailSkipLinks from "@/components/shared/DetailSkipLinks";
import DetailHero from "@/components/shared/DetailHero";
import OverviewSection from "@/components/shared/OverviewSection";
import ShowDetailsHeroActions from "@/components/shows/ShowDetailsHeroActions";
import ShowDetailsMetadataChips from "@/components/shows/ShowDetailsMetadataChips";
import ShowSeasonsSection, {
  ShowSeasonsSectionPlaceholder,
} from "@/components/shows/ShowSeasonsSection";
import ShowCrewSection from "@/components/shows/ShowCrewSection";
import ShowAboutSection from "@/components/shows/ShowAboutSection";
import type { CastSectionItem } from "@/components/shared/CastSection";
import type { ShowDetailsDataType } from "@/types";

export const Route = createFileRoute("/_auth/tv-shows/$id/")({
  validateSearch: showDetailsSearchSchema,
  loaderDeps: ({ search: { season } }) => ({ season }),
  loader: async ({ context, params, deps: { season } }) => {
    const showId = parseRouteId(params.id);
    if (showId == null) return;

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
  const showId = parseRouteId(id);

  // A malformed id never reaches the API: the query options disable
  // themselves for the zero sentinel, and the page goes straight to
  // not-found rather than sitting on a skeleton.
  const { data, isPending, isError } = useQuery(
    showDetailsQueryOpts(showId ?? 0),
  );

  const payload = data?.data;
  const show = payload?.show;

  if (showId == null) {
    return (
      <MediaNotFound
        message="That show link is not valid."
        backTo="/tv-shows"
        backLabel="Back to TV Shows"
      />
    );
  }

  if (isError || (data && data.error)) {
    return (
      <MediaNotFound
        message={
          data?.message || "Failed to load show details. Please try again later."
        }
        backTo="/tv-shows"
        backLabel="Back to TV Shows"
      />
    );
  }

  if (isPending) {
    return (
      <DetailSkeleton label="Loading show details" withActions>
        <ShowSeasonsSectionPlaceholder />
      </DetailSkeleton>
    );
  }

  if (!show || !payload) {
    return (
      <MediaNotFound
        message="Show not found."
        backTo="/tv-shows"
        backLabel="Back to TV Shows"
      />
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
  const certificationLabel = trimmedOrNull(unwrapString(show.certification));
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

  return (
    <article aria-labelledby="show-title" className="w-full min-w-0 pb-6 sm:pb-10">
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <DetailSkipLinks
        titleHref="#show-title"
        titleLabel="Skip to show info"
        sections={[
          { href: "#overview-heading", label: "Skip to overview" },
          seasons.length > 0 && {
            href: "#seasons-heading",
            label: "Skip to seasons",
          },
          (creators.length > 0 || crew.length > 0) && {
            href: "#crew-heading",
            label: "Skip to key crew",
          },
          castForSection.length > 0 && {
            href: "#cast-heading",
            label: "Skip to cast",
          },
          youtubeExtraVideos.length > 0 && {
            href: "#extra-videos-heading",
            label: "Skip to extra videos",
          },
          { href: "#details-heading", label: "Skip to about" },
        ]}
      />

      <DetailHero
        backdropUrl={backdropUrl}
        posterUrl={posterUrl}
        posterAlt={`Poster for ${show.name}`}
        placeholderIcon={Tv}
        titleId="show-title"
        title={show.name}
        year={premiereYear}
        dateTime={firstAirDate}
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
        actionsSlot={
          seasons.length > 0 && (
            <ShowDetailsHeroActions
              showId={showId}
              selectedSeason={selectedSeason}
            />
          )
        }
      />

      <div
        className={cn(
          DETAIL_PAGE_CONTENT_ENTER_CLASS,
          "delay-150 motion-reduce:delay-0",
        )}
      >
        <OverviewSection overview={overview} />

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
          // Carry the season back, or returning from a trailer would reset a
          // viewer who had paged to season four down to season one.
          returnTo={
            seasons.length > 0
              ? `/tv-shows/${showId}?season=${selectedSeason}`
              : `/tv-shows/${showId}`
          }
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
