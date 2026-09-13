import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { Film } from "lucide-react";
import VideoPlaybackPage from "@/components/playback/VideoPlaybackPage";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { movieMediaRef } from "@/lib/media-ref";
import { unwrapFloatOrUndefined, unwrapString } from "@/lib/nullable";
import { loadPlayRoute } from "@/lib/play-route-loader";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { playSearchSchema, type PlaySearchParams } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";

export const Route = createFileRoute("/_auth/movies/$id/play")({
  validateSearch: playSearchSchema,
  loaderDeps: ({ search }) => ({
    mode: search.mode,
    audio_track: search.audio_track,
    subtitle_track: search.subtitle_track,
    start: search.start,
  }),
  loader: async ({ context, params, deps }) => {
    const movieId = parseRouteId(params.id);
    if (movieId == null) return;

    const search = await loadPlayRoute({
      queryClient: context.queryClient,
      media: movieMediaRef(movieId),
      ensureDetails: () =>
        context.queryClient.ensureQueryData(
          libraryMovieDetailsQueryOpts(movieId),
        ),
      deps,
    });
    if (!search) return;

    throw redirect({
      to: "/movies/$id/play",
      params: { id: params.id },
      search,
      replace: true,
    });
  },
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { id } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  // 0 for a malformed id: the request then 404s into the player's error
  // screen, exactly as an unknown movie does.
  const movieId = parseRouteId(id) ?? 0;

  const { data, isPending, isError } = useQuery(
    libraryMovieDetailsQueryOpts(movieId),
  );
  const movie = data && !data.error ? data.data?.movie : null;
  const notFound = Boolean(isError || (data && data.error) || (data && !movie));

  return (
    <VideoPlaybackPage
      media={movieMediaRef(movieId)}
      search={search}
      onNavigateSearch={(update) =>
        navigate({
          search: (prev: PlaySearchParams) => update(prev),
          replace: true,
        })
      }
      onBackFallback={() => navigate({ to: "/movies" })}
      title={movie?.title ?? "Movie"}
      artworkUrl={
        movie
          ? buildTmdbImageUrl(unwrapString(movie.poster_path), TMDB_POSTER_SIZE)
          : null
      }
      headerIcon={Film}
      detailsPending={isPending}
      notFound={notFound}
      fallbackDurationSec={unwrapFloatOrUndefined(movie?.duration)}
    />
  );
}
