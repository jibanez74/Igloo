import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ListVideo } from "lucide-react";
import MovieCard from "@/components/movies/MovieCard";
import LibraryAllTab, {
  LibraryAllTabSkeleton,
  type LibraryNoun,
} from "@/components/shared/LibraryAllTab";
import MediaDetailGuard from "@/components/shared/MediaDetailGuard";
import {
  moviePlaylistDetailsQueryOpts,
  moviePlaylistMoviesQueryOpts,
} from "@/lib/query-opts";
import {
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  MOVIES_PER_PAGE,
  MOVIES_PLAYLISTS_TAB_SEARCH,
} from "@/lib/constants";
import { pluralize } from "@/lib/format";
import { cn } from "@/lib/utils";
import { unwrapString } from "@/lib/nullable";
import { parseRouteId } from "@/lib/route-id";
import type { MoviePlaylistDetailResponseType } from "@/types";

const MOVIE_NOUN: LibraryNoun = { singular: "movie", plural: "movies" };

export const Route = createFileRoute("/_auth/movies/playlist/$id")({
  loader: async ({ context, params }) => {
    const id = parseRouteId(params.id);
    if (id == null) return;
    await Promise.all([
      context.queryClient.ensureQueryData(moviePlaylistDetailsQueryOpts(id)),
      context.queryClient.ensureQueryData(
        moviePlaylistMoviesQueryOpts(id, 1, MOVIES_PER_PAGE, "asc"),
      ),
    ]);
  },
  component: MoviePlaylistPage,
});

function MoviePlaylistPage() {
  const { id } = Route.useParams();
  const playlistId = parseRouteId(id);

  // A malformed id never reaches the API: the query options disable
  // themselves for the zero sentinel, and the guard goes straight to
  // not-found rather than sitting on a skeleton.
  const { data, isLoading, isError } = useQuery(
    moviePlaylistDetailsQueryOpts(playlistId ?? 0),
  );

  return (
    <MediaDetailGuard
      id={playlistId}
      noun="playlist"
      back="moviePlaylists"
      isPending={isLoading}
      isError={isError}
      data={data}
      payload={data?.error === false ? data.data : null}
      skeleton={<MoviePlaylistSkeleton />}
    >
      {(loaded, id) => <MoviePlaylistContent key={id} playlistId={id} data={loaded} />}
    </MediaDetailGuard>
  );
}

type MoviePlaylistContentProps = {
  playlistId: number;
  data: MoviePlaylistDetailResponseType;
};

function MoviePlaylistContent({ playlistId, data }: MoviePlaylistContentProps) {
  const [page, setPage] = useState(1);
  const [sort, setSort] = useState<"asc" | "desc">("asc");

  const { playlist, movie_count } = data;
  const desc = unwrapString(playlist.description);

  return (
    <div className="min-w-0">
      <title>{playlist.name} - Igloo</title>
      <meta name="description" content={`Movie playlist: ${playlist.name}`} />

      <MoviePlaylistsBackLink />

      <header className="mb-8">
        <div className="flex items-start gap-3">
          <ListVideo className="mt-1 size-8 shrink-0 text-primary" aria-hidden="true" />
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
              {playlist.name}
            </h1>
            {desc ? (
              <p className="mt-2 text-muted-foreground">{desc}</p>
            ) : null}
            <p className="mt-2 text-sm text-muted-foreground">
              {pluralize(movie_count, "movie")}
            </p>
          </div>
        </div>
      </header>

      <LibraryAllTab
        queryOpts={moviePlaylistMoviesQueryOpts(playlistId, page, MOVIES_PER_PAGE, sort)}
        getItems={data => data.movies}
        renderCard={movie => <MovieCard movie={movie} />}
        currentPage={page}
        sort={sort}
        perPage={MOVIES_PER_PAGE}
        noun={MOVIE_NOUN}
        emptyIcon={ListVideo}
        emptyMessage="No movies in this playlist yet."
        onPageChange={setPage}
        onSortToggle={() => {
          setSort(s => (s === "asc" ? "desc" : "asc"));
          setPage(1);
        }}
        toolbarStartSlot={
          <span className="text-sm text-muted-foreground">Playlist movies</span>
        }
      />
    </div>
  );
}

function MoviePlaylistsBackLink() {
  return (
    <Link
      to="/movies"
      search={MOVIES_PLAYLISTS_TAB_SEARCH}
      className={cn(
        MOTION_MICRO_COLORS_CLASS,
        "mb-6 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-primary",
      )}
    >
      <ArrowLeft className="size-4" aria-hidden="true" />
      Movie playlists
    </Link>
  );
}

// Authored beside the layout it mirrors (design-system §3.4): the header's
// icon, title and count, the toolbar strip LibraryAllTab reserves in every
// state, then the poster grid. The back link is real: it works while loading.
function MoviePlaylistSkeleton() {
  return (
    <div className="min-w-0">
      <MoviePlaylistsBackLink />
      <div
        className={MOTION_LOADING_STATE_CLASS}
        role="status"
        aria-label="Loading playlist"
      >
        <span className="sr-only">Loading playlist...</span>
        <div className="mb-8 flex items-start gap-3" aria-hidden="true">
          <div className="mt-1 size-8 shrink-0 rounded-sm bg-muted" />
          <div className="min-w-0 flex-1 space-y-2">
            <div className="h-8 max-w-sm rounded-sm bg-muted md:h-9" />
            <div className="h-5 w-24 rounded-sm bg-muted" />
          </div>
        </div>
        <div className="mb-5 min-h-8" aria-hidden="true" />
        <div aria-hidden="true">
          <LibraryAllTabSkeleton perPage={MOVIES_PER_PAGE} />
        </div>
      </div>
    </div>
  );
}
