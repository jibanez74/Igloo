import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { Tv } from "lucide-react";
import VideoPlaybackPage from "@/components/playback/VideoPlaybackPage";
import { TMDB_BACKDROP_SIZE, TMDB_POSTER_SIZE, TMDB_STILL_SIZE } from "@/lib/constants";
import { episodeUpNextPresentation } from "@/lib/episode-playback";
import { episodeTitle } from "@/lib/format";
import { episodeMediaRef } from "@/lib/media-ref";
import { unwrapString } from "@/lib/nullable";
import { loadPlayRoute } from "@/lib/play-route-loader";
import { showEpisodeQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { routeHead } from "@/lib/route-head";
import { playSearchSchema, type PlaySearchParams } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { UpNextItem } from "@/types/playback";

const PLAY_EPISODE_FALLBACK_HEAD = routeHead("Playing Episode");

export const Route = createFileRoute(
  "/_auth/tv-shows/$id/episodes/$episodeId/play",
)({
  validateSearch: playSearchSchema,
  loaderDeps: ({ search }) => ({
    mode: search.mode,
    audio_track: search.audio_track,
    subtitle_track: search.subtitle_track,
    start: search.start,
    autoplay: search.autoplay,
  }),
  loader: async ({ context, params, deps }) => {
    const episodeId = parseRouteId(params.episodeId);
    if (episodeId == null) return { title: null };

    const search = await loadPlayRoute({
      queryClient: context.queryClient,
      media: episodeMediaRef(episodeId),
      ensureDetails: () =>
        context.queryClient.ensureQueryData(showEpisodeQueryOpts(episodeId)),
      deps,
    });
    if (search) {
      throw redirect({
        to: "/tv-shows/$id/episodes/$episodeId/play",
        params: { id: params.id, episodeId: params.episodeId },
        search,
        replace: true,
      });
    }

    // Cache only: a URL that already carries a mode starts playback without
    // waiting on the details, so a cold load titles the tab generically.
    const details = context.queryClient.getQueryData(
      showEpisodeQueryOpts(episodeId).queryKey,
    );

    return {
      title:
        details && !details.error
          ? episodeTitle(
              details.data.show.name,
              details.data.season.season_number,
              details.data.episode.episode_number,
              details.data.episode.name,
            )
          : null,
    };
  },
  head: ({ loaderData }) =>
    loaderData?.title
      ? routeHead(`Playing ${loaderData.title}`)
      : PLAY_EPISODE_FALLBACK_HEAD,
  component: PlayEpisodePage,
});

function PlayEpisodePage() {
  const { id, episodeId: episodeIdParam } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  // 0 for a malformed id: the request then 404s into the player's error
  // screen, exactly as an unknown episode does.
  const episodeId = parseRouteId(episodeIdParam) ?? 0;

  const { data, isPending, isError } = useQuery(
    showEpisodeQueryOpts(episodeId),
  );
  const payload = data && !data.error ? data.data : null;
  const notFound = Boolean(isError || (data && data.error));

  // Back lands on the season the episode belongs to, so a viewer who paged
  // to season four is not dropped on season one.
  const seasonNumber = payload?.season.season_number;

  // The next episode is played from its saved position when it has one, so
  // the hand-off never stops on the resume dialog; the loader resolves the
  // new file's default mode and tracks. A push, not a replace: Back returns
  // to the episode that just ended.
  const nextEpisode = payload?.next_episode ?? null;
  const nextPresentation = nextEpisode
    ? episodeUpNextPresentation(nextEpisode)
    : null;
  const upNext: UpNextItem | null =
    nextEpisode && nextPresentation
      ? {
          title: nextPresentation.title,
          stillUrl: nextPresentation.stillUrl,
          resume: nextPresentation.resume,
          onPlay: () =>
            void navigate({
              to: "/tv-shows/$id/episodes/$episodeId/play",
              params: { id, episodeId: String(nextEpisode.id) },
              search: {
                start: nextPresentation.startSec,
                autoplay: true,
              },
            }),
        }
      : null;

  return (
    // Keyed on the episode: the player's refs and state belong to one media
    // item, and the router reuses the component when only the param changes.
    <VideoPlaybackPage
      key={episodeId}
      media={episodeMediaRef(episodeId)}
      search={search}
      onNavigateSearch={(update) =>
        navigate({
          search: (prev: PlaySearchParams) => update(prev),
          replace: true,
        })
      }
      onBackFallback={() =>
        navigate({
          to: "/tv-shows/$id",
          params: { id },
          search: seasonNumber != null ? { season: seasonNumber } : {},
        })
      }
      title={
        payload
          ? episodeTitle(
              payload.show.name,
              payload.season.season_number,
              payload.episode.episode_number,
              payload.episode.name,
            )
          : "Episode"
      }
      artworkUrl={
        payload
          ? buildTmdbImageUrl(
              unwrapString(payload.show.poster_path),
              TMDB_POSTER_SIZE,
            )
          : null
      }
      posterUrl={
        payload
          ? buildTmdbImageUrl(
              unwrapString(payload.episode.still_path),
              TMDB_STILL_SIZE,
            ) ||
            buildTmdbImageUrl(
              unwrapString(payload.show.backdrop_path),
              TMDB_BACKDROP_SIZE,
            )
          : null
      }
      headerIcon={Tv}
      detailsPending={isPending}
      notFound={notFound}
      upNext={upNext}
    />
  );
}
