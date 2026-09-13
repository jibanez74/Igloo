import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { Tv } from "lucide-react";
import VideoPlaybackPage from "@/components/playback/VideoPlaybackPage";
import { TMDB_POSTER_SIZE, TMDB_STILL_SIZE } from "@/lib/constants";
import { episodeResumeProgress } from "@/lib/episode-playback";
import { episodeCode, episodeTitle } from "@/lib/format";
import { episodeMediaRef } from "@/lib/media-ref";
import { unwrapString } from "@/lib/nullable";
import { loadPlayRoute } from "@/lib/play-route-loader";
import { showEpisodeQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { playSearchSchema, type PlaySearchParams } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { UpNextItem } from "@/types/playback";

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
    if (episodeId == null) return;

    const search = await loadPlayRoute({
      queryClient: context.queryClient,
      media: episodeMediaRef(episodeId),
      ensureDetails: () =>
        context.queryClient.ensureQueryData(showEpisodeQueryOpts(episodeId)),
      deps,
    });
    if (!search) return;

    throw redirect({
      to: "/tv-shows/$id/episodes/$episodeId/play",
      params: { id: params.id, episodeId: params.episodeId },
      search,
      replace: true,
    });
  },
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
  const nextResume = nextEpisode ? episodeResumeProgress(nextEpisode) : null;
  const upNext: UpNextItem | null = nextEpisode
    ? {
        title: `${episodeCode(nextEpisode.season_number, nextEpisode.episode_number)} · ${nextEpisode.name}`,
        stillUrl: buildTmdbImageUrl(
          unwrapString(nextEpisode.still_path),
          TMDB_STILL_SIZE,
        ),
        resume: nextResume !== null,
        onPlay: () =>
          void navigate({
            to: "/tv-shows/$id/episodes/$episodeId/play",
            params: { id, episodeId: String(nextEpisode.id) },
            search: {
              start: Math.floor(nextResume?.progressSec ?? 0),
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
      headerIcon={Tv}
      detailsPending={isPending}
      notFound={notFound}
      upNext={upNext}
    />
  );
}
