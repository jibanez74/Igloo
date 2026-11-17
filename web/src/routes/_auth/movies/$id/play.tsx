import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { Film } from "lucide-react";
import VideoPlaybackPage from "@/components/playback/VideoPlaybackPage";
import { useLastSuccess } from "@/hooks/useLastSuccess";
import { TMDB_BACKDROP_SIZE, TMDB_POSTER_SIZE } from "@/lib/constants";
import { movieMediaRef } from "@/lib/media-ref";
import { unwrapFloatOrUndefined, unwrapString } from "@/lib/nullable";
import { loadPlayRoute } from "@/lib/play-route-loader";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import { parseRouteId } from "@/lib/route-id";
import { routeHead } from "@/lib/route-head";
import { playSearchSchema, type PlaySearchParams } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";

const PLAY_MOVIE_FALLBACK_HEAD = routeHead("Playing Movie");

export const Route = createFileRoute("/_auth/movies/$id/play")({
  // The player fills the window: no sidebar, header or page padding.
  staticData: { immersive: true },
  validateSearch: playSearchSchema,
  // The loader reads the start only to build the redirect for a URL without a
  // mode. Keyed on it otherwise, every HLS rebase (a new start) reran the
  // loader and the head for nothing.
  loaderDeps: ({ search }) =>
    search.mode === undefined
      ? { mode: search.mode, start: search.start }
      : { mode: search.mode },
  loader: async ({ context, params, deps }) => {
    const movieId = parseRouteId(params.id);
    if (movieId == null) return { title: null };

    const search = await loadPlayRoute({
      queryClient: context.queryClient,
      media: movieMediaRef(movieId),
      ensureDetails: () =>
        context.queryClient.ensureQueryData(
          libraryMovieDetailsQueryOpts(movieId),
        ),
      deps,
    });
    if (search) {
      throw redirect({
        to: "/movies/$id/play",
        params: { id: params.id },
        search,
        replace: true,
      });
    }

    // Cache only: a URL that already carries a mode starts playback without
    // waiting on the details, so a cold load titles the tab generically.
    const details = context.queryClient.getQueryData(
      libraryMovieDetailsQueryOpts(movieId).queryKey,
    );

    return {
      title:
        details && !details.error ? (details.data.movie?.title ?? null) : null,
    };
  },
  head: ({ loaderData }) =>
    loaderData?.title
      ? routeHead(`Playing ${loaderData.title}`)
      : PLAY_MOVIE_FALLBACK_HEAD,
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { id } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  // 0 for a malformed id disables the details query, which then never leaves
  // pending, so the id itself makes the page not found, like an unknown movie.
  const parsedId = parseRouteId(id);
  const movieId = parsedId ?? 0;

  const detailsQuery = useQuery(libraryMovieDetailsQueryOpts(movieId));
  const { isPending, isError } = detailsQuery;
  // A failed background refetch must not turn a playing movie "not found".
  const data = useLastSuccess(detailsQuery.data);
  const movie = data && !data.error ? data.data?.movie : null;
  const notFound =
    parsedId === null ||
    Boolean(isError || (data && data.error) || (data && !movie));

  return (
    // Keyed on the movie: the player's refs and state belong to one media
    // item, and the router reuses the component when only the param changes.
    <VideoPlaybackPage
      key={movieId}
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
      posterUrl={
        movie
          ? buildTmdbImageUrl(unwrapString(movie.backdrop_path), TMDB_BACKDROP_SIZE) ||
            buildTmdbImageUrl(unwrapString(movie.poster_path), TMDB_POSTER_SIZE)
          : null
      }
      headerIcon={Film}
      detailsPending={isPending}
      notFound={notFound}
      fallbackDurationSec={unwrapFloatOrUndefined(movie?.duration)}
    />
  );
}
